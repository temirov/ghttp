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
	"net"
	"slices"
	"time"
)

// ServerCertificateConfiguration defines how leaf certificates are generated and persisted.
type ServerCertificateConfiguration struct {
	CertificateValidityDuration      time.Duration
	CertificateRenewalWindowDuration time.Duration
	LeafPrivateKeyBitSize            int
	CertificateFilePermissions       fs.FileMode
	PrivateKeyFilePermissions        fs.FileMode
}

// ServerCertificateRequest describes the desired certificate attributes and output paths.
type ServerCertificateRequest struct {
	Hosts                 []string
	CertificateOutputPath string
	PrivateKeyOutputPath  string
}

// ServerCertificateMaterial contains the leaf certificate artifacts.
type ServerCertificateMaterial struct {
	CertificateBytes []byte
	PrivateKeyBytes  []byte
	TLSCertificate   *x509.Certificate
	PrivateKey       *rsa.PrivateKey
}

// ServerCertificateIssuer signs leaf certificates using a root certificate authority.
type ServerCertificateIssuer struct {
	fileSystem       FileSystem
	clock            Clock
	randomnessSource io.Reader
	configuration    ServerCertificateConfiguration
}

// NewServerCertificateIssuer constructs a ServerCertificateIssuer.
func NewServerCertificateIssuer(fileSystem FileSystem, clock Clock, randomnessSource io.Reader, configuration ServerCertificateConfiguration) ServerCertificateIssuer {
	return ServerCertificateIssuer{
		fileSystem:       fileSystem,
		clock:            clock,
		randomnessSource: randomnessSource,
		configuration:    configuration,
	}
}

// IssueServerCertificate returns a valid leaf certificate for the requested hosts.
func (issuer ServerCertificateIssuer) IssueServerCertificate(ctx context.Context, certificateAuthority CertificateAuthorityMaterial, request ServerCertificateRequest) (ServerCertificateMaterial, error) {
	err := issuer.validateRequest(request)
	if err != nil {
		return ServerCertificateMaterial{}, fmt.Errorf("validate server certificate request: %w", err)
	}

	existingMaterial, existingErr := issuer.loadExisting(request)
	if existingErr == nil {
		shouldRotate := issuer.shouldRotate(existingMaterial.TLSCertificate, request.Hosts)
		if !shouldRotate {
			return existingMaterial, nil
		}
	}
	if existingErr != nil && !errors.Is(existingErr, fs.ErrNotExist) {
		return ServerCertificateMaterial{}, fmt.Errorf("load existing server certificate: %w", existingErr)
	}

	select {
	case <-ctx.Done():
		return ServerCertificateMaterial{}, fmt.Errorf("issue server certificate: %w", ctx.Err())
	default:
	}

	privateKey, privateKeyErr := rsa.GenerateKey(issuer.randomnessSource, issuer.configuration.LeafPrivateKeyBitSize)
	if privateKeyErr != nil {
		return ServerCertificateMaterial{}, fmt.Errorf("generate leaf private key: %w", privateKeyErr)
	}

	now := issuer.clock.Now()
	serialNumber, serialErr := issuer.generateSerialNumber()
	if serialErr != nil {
		return ServerCertificateMaterial{}, fmt.Errorf("generate leaf serial number: %w", serialErr)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: request.Hosts[0],
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(issuer.configuration.CertificateValidityDuration),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	for _, host := range request.Hosts {
		ip := net.ParseIP(host)
		if ip != nil {
			if ip.To4() != nil {
				template.IPAddresses = append(template.IPAddresses, ip)
			} else {
				template.IPAddresses = append(template.IPAddresses, ip)
			}
		} else {
			template.DNSNames = append(template.DNSNames, host)
		}
	}

	certificateDer, certificateErr := x509.CreateCertificate(issuer.randomnessSource, &template, certificateAuthority.Certificate, &privateKey.PublicKey, certificateAuthority.PrivateKey)
	if certificateErr != nil {
		return ServerCertificateMaterial{}, fmt.Errorf("create server certificate: %w", certificateErr)
	}

	certificatePem := pem.EncodeToMemory(&pem.Block{Type: certificatePemBlockType, Bytes: certificateDer})
	privateKeyPem := pem.EncodeToMemory(&pem.Block{Type: privateKeyPemBlockType, Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	writeCertificateErr := issuer.fileSystem.WriteFile(request.CertificateOutputPath, certificatePem, issuer.configuration.CertificateFilePermissions)
	if writeCertificateErr != nil {
		return ServerCertificateMaterial{}, fmt.Errorf("write server certificate: %w", writeCertificateErr)
	}
	writePrivateKeyErr := issuer.fileSystem.WriteFile(request.PrivateKeyOutputPath, privateKeyPem, issuer.configuration.PrivateKeyFilePermissions)
	if writePrivateKeyErr != nil {
		return ServerCertificateMaterial{}, fmt.Errorf("write server private key: %w", writePrivateKeyErr)
	}

	parsedCertificate, parseErr := parseCertificateFromPEM(certificatePem)
	if parseErr != nil {
		return ServerCertificateMaterial{}, fmt.Errorf("parse issued server certificate: %w", parseErr)
	}

	return ServerCertificateMaterial{
		CertificateBytes: certificatePem,
		PrivateKeyBytes:  privateKeyPem,
		TLSCertificate:   parsedCertificate,
		PrivateKey:       privateKey,
	}, nil
}

func (issuer ServerCertificateIssuer) validateRequest(request ServerCertificateRequest) error {
	if len(request.Hosts) == 0 {
		return errors.New("at least one host is required")
	}
	if request.CertificateOutputPath == "" {
		return errors.New("certificate output path is required")
	}
	if request.PrivateKeyOutputPath == "" {
		return errors.New("private key output path is required")
	}
	return nil
}

func (issuer ServerCertificateIssuer) loadExisting(request ServerCertificateRequest) (ServerCertificateMaterial, error) {
	certificateExists, certificateExistsErr := issuer.fileSystem.FileExists(request.CertificateOutputPath)
	if certificateExistsErr != nil {
		return ServerCertificateMaterial{}, fmt.Errorf("check existing certificate: %w", certificateExistsErr)
	}
	privateKeyExists, privateKeyExistsErr := issuer.fileSystem.FileExists(request.PrivateKeyOutputPath)
	if privateKeyExistsErr != nil {
		return ServerCertificateMaterial{}, fmt.Errorf("check existing private key: %w", privateKeyExistsErr)
	}
	if !certificateExists || !privateKeyExists {
		return ServerCertificateMaterial{}, fs.ErrNotExist
	}

	certificateBytes, certificateReadErr := issuer.fileSystem.ReadFile(request.CertificateOutputPath)
	if certificateReadErr != nil {
		return ServerCertificateMaterial{}, fmt.Errorf("read existing certificate: %w", certificateReadErr)
	}
	privateKeyBytes, privateKeyReadErr := issuer.fileSystem.ReadFile(request.PrivateKeyOutputPath)
	if privateKeyReadErr != nil {
		return ServerCertificateMaterial{}, fmt.Errorf("read existing private key: %w", privateKeyReadErr)
	}

	certificate, parseCertificateErr := parseCertificateFromPEM(certificateBytes)
	if parseCertificateErr != nil {
		return ServerCertificateMaterial{}, fmt.Errorf("parse existing certificate: %w", parseCertificateErr)
	}

	privateKey, parsePrivateKeyErr := parseRSAPrivateKeyFromPEM(privateKeyBytes)
	if parsePrivateKeyErr != nil {
		return ServerCertificateMaterial{}, fmt.Errorf("parse existing private key: %w", parsePrivateKeyErr)
	}

	return ServerCertificateMaterial{
		CertificateBytes: certificateBytes,
		PrivateKeyBytes:  privateKeyBytes,
		TLSCertificate:   certificate,
		PrivateKey:       privateKey,
	}, nil
}

func (issuer ServerCertificateIssuer) shouldRotate(certificate *x509.Certificate, requestedHosts []string) bool {
	currentTime := issuer.clock.Now()
	if currentTime.After(certificate.NotAfter) {
		return true
	}
	renewalThreshold := certificate.NotAfter.Add(-issuer.configuration.CertificateRenewalWindowDuration)
	if currentTime.After(renewalThreshold) {
		return true
	}

	existingHosts := append([]string{}, certificate.DNSNames...)
	for _, address := range certificate.IPAddresses {
		existingHosts = append(existingHosts, address.String())
	}
	slices.Sort(existingHosts)
	sortedRequestedHosts := append([]string{}, requestedHosts...)
	slices.Sort(sortedRequestedHosts)
	return !slices.Equal(existingHosts, sortedRequestedHosts)
}

func (issuer ServerCertificateIssuer) generateSerialNumber() (*big.Int, error) {
	upperBound := new(big.Int).Lsh(big.NewInt(1), defaultCertificateSerialNumberUpperBitLen)
	serialNumber, err := rand.Int(issuer.randomnessSource, upperBound)
	if err != nil {
		return nil, err
	}
	return serialNumber, nil
}
