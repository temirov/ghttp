package certificates

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"path/filepath"
	"time"
)

const (
	certificatePemBlockType                   = "CERTIFICATE"
	privateKeyPemBlockType                    = "RSA PRIVATE KEY"
	defaultCertificateSerialNumberUpperBitLen = 128
)

// CertificateAuthorityConfiguration defines storage and lifetime parameters for the root certificate authority.
type CertificateAuthorityConfiguration struct {
	DirectoryPath                    string
	CertificateFileName              string
	PrivateKeyFileName               string
	DirectoryPermissions             fs.FileMode
	CertificateFilePermissions       fs.FileMode
	PrivateKeyFilePermissions        fs.FileMode
	RSAKeyBitSize                    int
	CertificateValidityDuration      time.Duration
	CertificateRenewalWindowDuration time.Duration
	SubjectCommonName                string
	SubjectOrganizationalUnit        string
	SubjectOrganization              string
}

// CertificateAuthorityMaterial contains the root certificate authority artifacts.
type CertificateAuthorityMaterial struct {
	CertificateBytes []byte
	PrivateKeyBytes  []byte
	Certificate      *x509.Certificate
	PrivateKey       *rsa.PrivateKey
}

// CertificateAuthorityManager provisions and loads root certificate authorities.
type CertificateAuthorityManager struct {
	fileSystem       FileSystem
	clock            Clock
	randomnessSource io.Reader
	configuration    CertificateAuthorityConfiguration
}

// NewCertificateAuthorityManager constructs a CertificateAuthorityManager.
func NewCertificateAuthorityManager(fileSystem FileSystem, clock Clock, randomnessSource io.Reader, configuration CertificateAuthorityConfiguration) CertificateAuthorityManager {
	return CertificateAuthorityManager{
		fileSystem:       fileSystem,
		clock:            clock,
		randomnessSource: randomnessSource,
		configuration:    configuration,
	}
}

// EnsureCertificateAuthority returns a valid root certificate authority, creating or rotating it when necessary.
func (manager CertificateAuthorityManager) EnsureCertificateAuthority(ctx context.Context) (CertificateAuthorityMaterial, error) {
	rootCertificatePath := filepath.Join(manager.configuration.DirectoryPath, manager.configuration.CertificateFileName)
	rootPrivateKeyPath := filepath.Join(manager.configuration.DirectoryPath, manager.configuration.PrivateKeyFileName)

	err := manager.fileSystem.EnsureDirectory(manager.configuration.DirectoryPath, manager.configuration.DirectoryPermissions)
	if err != nil {
		return CertificateAuthorityMaterial{}, fmt.Errorf("ensure certificate authority directory: %w", err)
	}

	material, readErr := manager.loadExisting(rootCertificatePath, rootPrivateKeyPath)
	if readErr == nil {
		if manager.shouldRotate(material.Certificate) {
			return manager.generateAndPersist(ctx, rootCertificatePath, rootPrivateKeyPath)
		}
		return material, nil
	}
	if !errors.Is(readErr, fs.ErrNotExist) {
		return CertificateAuthorityMaterial{}, fmt.Errorf("load certificate authority: %w", readErr)
	}

	return manager.generateAndPersist(ctx, rootCertificatePath, rootPrivateKeyPath)
}

func (manager CertificateAuthorityManager) loadExisting(rootCertificatePath string, rootPrivateKeyPath string) (CertificateAuthorityMaterial, error) {
	certificateExists, certificateExistsErr := manager.fileSystem.FileExists(rootCertificatePath)
	if certificateExistsErr != nil {
		return CertificateAuthorityMaterial{}, fmt.Errorf("check certificate file: %w", certificateExistsErr)
	}
	privateKeyExists, privateKeyExistsErr := manager.fileSystem.FileExists(rootPrivateKeyPath)
	if privateKeyExistsErr != nil {
		return CertificateAuthorityMaterial{}, fmt.Errorf("check private key file: %w", privateKeyExistsErr)
	}
	if !certificateExists || !privateKeyExists {
		return CertificateAuthorityMaterial{}, fs.ErrNotExist
	}

	certificateBytes, certificateReadErr := manager.fileSystem.ReadFile(rootCertificatePath)
	if certificateReadErr != nil {
		return CertificateAuthorityMaterial{}, fmt.Errorf("read certificate file: %w", certificateReadErr)
	}
	privateKeyBytes, privateKeyReadErr := manager.fileSystem.ReadFile(rootPrivateKeyPath)
	if privateKeyReadErr != nil {
		return CertificateAuthorityMaterial{}, fmt.Errorf("read private key file: %w", privateKeyReadErr)
	}

	certificate, parseCertificateErr := parseCertificateFromPEM(certificateBytes)
	if parseCertificateErr != nil {
		return CertificateAuthorityMaterial{}, fmt.Errorf("parse certificate: %w", parseCertificateErr)
	}

	privateKey, parsePrivateKeyErr := parseRSAPrivateKeyFromPEM(privateKeyBytes)
	if parsePrivateKeyErr != nil {
		return CertificateAuthorityMaterial{}, fmt.Errorf("parse private key: %w", parsePrivateKeyErr)
	}

	return CertificateAuthorityMaterial{
		CertificateBytes: certificateBytes,
		PrivateKeyBytes:  privateKeyBytes,
		Certificate:      certificate,
		PrivateKey:       privateKey,
	}, nil
}

func (manager CertificateAuthorityManager) shouldRotate(certificate *x509.Certificate) bool {
	currentTime := manager.clock.Now()
	if currentTime.After(certificate.NotAfter) {
		return true
	}
	renewalThreshold := certificate.NotAfter.Add(-manager.configuration.CertificateRenewalWindowDuration)
	return currentTime.After(renewalThreshold)
}

func (manager CertificateAuthorityManager) generateAndPersist(ctx context.Context, rootCertificatePath string, rootPrivateKeyPath string) (CertificateAuthorityMaterial, error) {
	select {
	case <-ctx.Done():
		return CertificateAuthorityMaterial{}, fmt.Errorf("generate certificate authority: %w", ctx.Err())
	default:
	}

	privateKey, privateKeyErr := rsa.GenerateKey(manager.randomnessSource, manager.configuration.RSAKeyBitSize)
	if privateKeyErr != nil {
		return CertificateAuthorityMaterial{}, fmt.Errorf("generate private key: %w", privateKeyErr)
	}

	serialNumber, serialErr := manager.generateSerialNumber()
	if serialErr != nil {
		return CertificateAuthorityMaterial{}, fmt.Errorf("generate serial number: %w", serialErr)
	}

	now := manager.clock.Now()
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:         manager.configuration.SubjectCommonName,
			OrganizationalUnit: []string{manager.configuration.SubjectOrganizationalUnit},
			Organization:       []string{manager.configuration.SubjectOrganization},
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(manager.configuration.CertificateValidityDuration),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
		MaxPathLenZero:        false,
	}

	certificateBytesDer, certificateErr := x509.CreateCertificate(manager.randomnessSource, &template, &template, &privateKey.PublicKey, privateKey)
	if certificateErr != nil {
		return CertificateAuthorityMaterial{}, fmt.Errorf("create certificate: %w", certificateErr)
	}

	certificatePem := pem.EncodeToMemory(&pem.Block{Type: certificatePemBlockType, Bytes: certificateBytesDer})
	privateKeyPem := pem.EncodeToMemory(&pem.Block{Type: privateKeyPemBlockType, Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	writeCertificateErr := manager.fileSystem.WriteFile(rootCertificatePath, certificatePem, manager.configuration.CertificateFilePermissions)
	if writeCertificateErr != nil {
		return CertificateAuthorityMaterial{}, fmt.Errorf("write certificate file: %w", writeCertificateErr)
	}
	writePrivateKeyErr := manager.fileSystem.WriteFile(rootPrivateKeyPath, privateKeyPem, manager.configuration.PrivateKeyFilePermissions)
	if writePrivateKeyErr != nil {
		return CertificateAuthorityMaterial{}, fmt.Errorf("write private key file: %w", writePrivateKeyErr)
	}

	certificate, parseCertificateErr := parseCertificateFromPEM(certificatePem)
	if parseCertificateErr != nil {
		return CertificateAuthorityMaterial{}, fmt.Errorf("parse generated certificate: %w", parseCertificateErr)
	}
	return CertificateAuthorityMaterial{
		CertificateBytes: certificatePem,
		PrivateKeyBytes:  privateKeyPem,
		Certificate:      certificate,
		PrivateKey:       privateKey,
	}, nil
}

func (manager CertificateAuthorityManager) generateSerialNumber() (*big.Int, error) {
	upperBound := new(big.Int).Lsh(big.NewInt(1), defaultCertificateSerialNumberUpperBitLen)
	serial, err := rand.Int(manager.randomnessSource, upperBound)
	if err != nil {
		return nil, err
	}
	return serial, nil
}
