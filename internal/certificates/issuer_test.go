package certificates

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestIssueServerCertificate(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name     string
		setup    func(testingT *testing.T) (CertificateAuthorityMaterial, ServerCertificateIssuer, ServerCertificateRequest, *controllableClock)
		validate func(testingT *testing.T, issuer ServerCertificateIssuer, request ServerCertificateRequest, certificateAuthority CertificateAuthorityMaterial, clock *controllableClock, material ServerCertificateMaterial, ctx context.Context)
	}{
		{
			name: "creates new server certificate when missing",
			setup: func(testingT *testing.T) (CertificateAuthorityMaterial, ServerCertificateIssuer, ServerCertificateRequest, *controllableClock) {
				clock := newControllableClock(time.Date(2025, 4, 1, 12, 0, 0, 0, time.UTC))
				fileSystem := NewOperatingSystemFileSystem()
				caConfiguration := CertificateAuthorityConfiguration{
					DirectoryPath:                    testingT.TempDir(),
					CertificateFileName:              "root_ca.pem",
					PrivateKeyFileName:               "root_ca.key",
					DirectoryPermissions:             0o700,
					CertificateFilePermissions:       0o600,
					PrivateKeyFilePermissions:        0o600,
					RSAKeyBitSize:                    2048,
					CertificateValidityDuration:      7 * 24 * time.Hour,
					CertificateRenewalWindowDuration: 24 * time.Hour,
					SubjectCommonName:                DefaultCertificateAuthorityCommonName,
					SubjectOrganizationalUnit:        DefaultCertificateAuthorityOrganizationalUnit,
					SubjectOrganization:              DefaultCertificateAuthorityOrganization,
				}
				caManager := NewCertificateAuthorityManager(fileSystem, clock, rand.Reader, caConfiguration)
				certificateAuthority, err := caManager.EnsureCertificateAuthority(ctx)
				if err != nil {
					testingT.Fatalf("ensure certificate authority: %v", err)
				}

				issuerConfiguration := ServerCertificateConfiguration{
					CertificateValidityDuration:      72 * time.Hour,
					CertificateRenewalWindowDuration: 12 * time.Hour,
					LeafPrivateKeyBitSize:            2048,
					CertificateFilePermissions:       0o600,
					PrivateKeyFilePermissions:        0o600,
				}
				issuer := NewServerCertificateIssuer(fileSystem, clock, rand.Reader, issuerConfiguration)
				leafCertificatePath := filepath.Join(caConfiguration.DirectoryPath, "leaf_cert.pem")
				leafPrivateKeyPath := filepath.Join(caConfiguration.DirectoryPath, "leaf_key.pem")
				request := ServerCertificateRequest{
					Hosts:                 []string{"localhost", "127.0.0.1"},
					CertificateOutputPath: leafCertificatePath,
					PrivateKeyOutputPath:  leafPrivateKeyPath,
				}
				return certificateAuthority, issuer, request, clock
			},
			validate: func(testingT *testing.T, issuer ServerCertificateIssuer, request ServerCertificateRequest, certificateAuthority CertificateAuthorityMaterial, clock *controllableClock, material ServerCertificateMaterial, ctx context.Context) {
				testingT.Helper()
				if len(material.CertificateBytes) == 0 || len(material.PrivateKeyBytes) == 0 {
					testingT.Fatalf("expected certificate and private key bytes to be present")
				}
				assertFilePermissions(testingT, request.CertificateOutputPath, issuer.configuration.CertificateFilePermissions)
				assertFilePermissions(testingT, request.PrivateKeyOutputPath, issuer.configuration.PrivateKeyFilePermissions)
				expectedHosts := []string{"127.0.0.1", "localhost"}
				actualHosts := append([]string{}, material.TLSCertificate.DNSNames...)
				for _, ipAddress := range material.TLSCertificate.IPAddresses {
					actualHosts = append(actualHosts, ipAddress.String())
				}
				slices.Sort(actualHosts)
				slices.Sort(expectedHosts)
				if !slices.Equal(actualHosts, expectedHosts) {
					testingT.Fatalf("expected hosts %v, got %v", expectedHosts, actualHosts)
				}
			},
		},
		{
			name: "reuses existing server certificate when valid",
			setup: func(testingT *testing.T) (CertificateAuthorityMaterial, ServerCertificateIssuer, ServerCertificateRequest, *controllableClock) {
				clock := newControllableClock(time.Date(2025, 4, 2, 9, 0, 0, 0, time.UTC))
				fileSystem := NewOperatingSystemFileSystem()
				caConfiguration := CertificateAuthorityConfiguration{
					DirectoryPath:                    testingT.TempDir(),
					CertificateFileName:              "root_ca.pem",
					PrivateKeyFileName:               "root_ca.key",
					DirectoryPermissions:             0o700,
					CertificateFilePermissions:       0o600,
					PrivateKeyFilePermissions:        0o600,
					RSAKeyBitSize:                    2048,
					CertificateValidityDuration:      14 * 24 * time.Hour,
					CertificateRenewalWindowDuration: 24 * time.Hour,
					SubjectCommonName:                DefaultCertificateAuthorityCommonName,
					SubjectOrganizationalUnit:        DefaultCertificateAuthorityOrganizationalUnit,
					SubjectOrganization:              DefaultCertificateAuthorityOrganization,
				}
				caManager := NewCertificateAuthorityManager(fileSystem, clock, rand.Reader, caConfiguration)
				certificateAuthority, err := caManager.EnsureCertificateAuthority(ctx)
				if err != nil {
					testingT.Fatalf("ensure certificate authority: %v", err)
				}
				issuerConfiguration := ServerCertificateConfiguration{
					CertificateValidityDuration:      5 * 24 * time.Hour,
					CertificateRenewalWindowDuration: 12 * time.Hour,
					LeafPrivateKeyBitSize:            2048,
					CertificateFilePermissions:       0o600,
					PrivateKeyFilePermissions:        0o600,
				}
				issuer := NewServerCertificateIssuer(fileSystem, clock, rand.Reader, issuerConfiguration)
				leafCertificatePath := filepath.Join(caConfiguration.DirectoryPath, "leaf_cert.pem")
				leafPrivateKeyPath := filepath.Join(caConfiguration.DirectoryPath, "leaf_key.pem")
				request := ServerCertificateRequest{
					Hosts:                 []string{"localhost"},
					CertificateOutputPath: leafCertificatePath,
					PrivateKeyOutputPath:  leafPrivateKeyPath,
				}
				return certificateAuthority, issuer, request, clock
			},
			validate: func(testingT *testing.T, issuer ServerCertificateIssuer, request ServerCertificateRequest, certificateAuthority CertificateAuthorityMaterial, clock *controllableClock, material ServerCertificateMaterial, ctx context.Context) {
				testingT.Helper()
				clock.Advance(24 * time.Hour)
				secondMaterial, err := issuer.IssueServerCertificate(ctx, certificateAuthority, request)
				if err != nil {
					testingT.Fatalf("issue server certificate second time: %v", err)
				}
				if material.TLSCertificate.SerialNumber.Cmp(secondMaterial.TLSCertificate.SerialNumber) != 0 {
					testingT.Fatalf("expected identical serial numbers when reusing certificate")
				}
			},
		},
		{
			name: "rotates when host list changes",
			setup: func(testingT *testing.T) (CertificateAuthorityMaterial, ServerCertificateIssuer, ServerCertificateRequest, *controllableClock) {
				clock := newControllableClock(time.Date(2025, 4, 3, 7, 0, 0, 0, time.UTC))
				fileSystem := NewOperatingSystemFileSystem()
				caConfiguration := CertificateAuthorityConfiguration{
					DirectoryPath:                    testingT.TempDir(),
					CertificateFileName:              "root_ca.pem",
					PrivateKeyFileName:               "root_ca.key",
					DirectoryPermissions:             0o700,
					CertificateFilePermissions:       0o600,
					PrivateKeyFilePermissions:        0o600,
					RSAKeyBitSize:                    2048,
					CertificateValidityDuration:      30 * 24 * time.Hour,
					CertificateRenewalWindowDuration: 48 * time.Hour,
					SubjectCommonName:                DefaultCertificateAuthorityCommonName,
					SubjectOrganizationalUnit:        DefaultCertificateAuthorityOrganizationalUnit,
					SubjectOrganization:              DefaultCertificateAuthorityOrganization,
				}
				caManager := NewCertificateAuthorityManager(fileSystem, clock, rand.Reader, caConfiguration)
				certificateAuthority, err := caManager.EnsureCertificateAuthority(ctx)
				if err != nil {
					testingT.Fatalf("ensure certificate authority: %v", err)
				}
				issuerConfiguration := ServerCertificateConfiguration{
					CertificateValidityDuration:      10 * 24 * time.Hour,
					CertificateRenewalWindowDuration: 24 * time.Hour,
					LeafPrivateKeyBitSize:            2048,
					CertificateFilePermissions:       0o600,
					PrivateKeyFilePermissions:        0o600,
				}
				issuer := NewServerCertificateIssuer(fileSystem, clock, rand.Reader, issuerConfiguration)
				leafCertificatePath := filepath.Join(caConfiguration.DirectoryPath, "leaf_cert.pem")
				leafPrivateKeyPath := filepath.Join(caConfiguration.DirectoryPath, "leaf_key.pem")
				request := ServerCertificateRequest{
					Hosts:                 []string{"localhost"},
					CertificateOutputPath: leafCertificatePath,
					PrivateKeyOutputPath:  leafPrivateKeyPath,
				}
				return certificateAuthority, issuer, request, clock
			},
			validate: func(testingT *testing.T, issuer ServerCertificateIssuer, request ServerCertificateRequest, certificateAuthority CertificateAuthorityMaterial, clock *controllableClock, material ServerCertificateMaterial, ctx context.Context) {
				testingT.Helper()
				requestWithAdditionalHost := ServerCertificateRequest{
					Hosts:                 []string{"localhost", "127.0.0.1"},
					CertificateOutputPath: request.CertificateOutputPath,
					PrivateKeyOutputPath:  request.PrivateKeyOutputPath,
				}
				newMaterial, err := issuer.IssueServerCertificate(ctx, certificateAuthority, requestWithAdditionalHost)
				if err != nil {
					testingT.Fatalf("issue server certificate with new host: %v", err)
				}
				if material.TLSCertificate.SerialNumber.Cmp(newMaterial.TLSCertificate.SerialNumber) == 0 {
					testingT.Fatalf("expected rotation when host list changes")
				}
				if !containsIPAddress(newMaterial.TLSCertificate.IPAddresses, net.ParseIP("127.0.0.1")) {
					testingT.Fatalf("expected certificate to include 127.0.0.1")
				}
			},
		},
		{
			name: "rotates when nearing expiry",
			setup: func(testingT *testing.T) (CertificateAuthorityMaterial, ServerCertificateIssuer, ServerCertificateRequest, *controllableClock) {
				clock := newControllableClock(time.Date(2025, 4, 4, 6, 0, 0, 0, time.UTC))
				fileSystem := NewOperatingSystemFileSystem()
				caConfiguration := CertificateAuthorityConfiguration{
					DirectoryPath:                    testingT.TempDir(),
					CertificateFileName:              "root_ca.pem",
					PrivateKeyFileName:               "root_ca.key",
					DirectoryPermissions:             0o700,
					CertificateFilePermissions:       0o600,
					PrivateKeyFilePermissions:        0o600,
					RSAKeyBitSize:                    2048,
					CertificateValidityDuration:      90 * 24 * time.Hour,
					CertificateRenewalWindowDuration: 24 * time.Hour,
					SubjectCommonName:                DefaultCertificateAuthorityCommonName,
					SubjectOrganizationalUnit:        DefaultCertificateAuthorityOrganizationalUnit,
					SubjectOrganization:              DefaultCertificateAuthorityOrganization,
				}
				caManager := NewCertificateAuthorityManager(fileSystem, clock, rand.Reader, caConfiguration)
				certificateAuthority, err := caManager.EnsureCertificateAuthority(ctx)
				if err != nil {
					testingT.Fatalf("ensure certificate authority: %v", err)
				}
				issuerConfiguration := ServerCertificateConfiguration{
					CertificateValidityDuration:      72 * time.Hour,
					CertificateRenewalWindowDuration: 6 * time.Hour,
					LeafPrivateKeyBitSize:            2048,
					CertificateFilePermissions:       0o600,
					PrivateKeyFilePermissions:        0o600,
				}
				issuer := NewServerCertificateIssuer(fileSystem, clock, rand.Reader, issuerConfiguration)
				leafCertificatePath := filepath.Join(caConfiguration.DirectoryPath, "leaf_cert.pem")
				leafPrivateKeyPath := filepath.Join(caConfiguration.DirectoryPath, "leaf_key.pem")
				request := ServerCertificateRequest{
					Hosts:                 []string{"localhost"},
					CertificateOutputPath: leafCertificatePath,
					PrivateKeyOutputPath:  leafPrivateKeyPath,
				}
				return certificateAuthority, issuer, request, clock
			},
			validate: func(testingT *testing.T, issuer ServerCertificateIssuer, request ServerCertificateRequest, certificateAuthority CertificateAuthorityMaterial, clock *controllableClock, material ServerCertificateMaterial, ctx context.Context) {
				testingT.Helper()
				clock.Advance(issuer.configuration.CertificateValidityDuration - issuer.configuration.CertificateRenewalWindowDuration + time.Hour)
				newMaterial, err := issuer.IssueServerCertificate(ctx, certificateAuthority, request)
				if err != nil {
					testingT.Fatalf("issue server certificate nearing expiry: %v", err)
				}
				if material.TLSCertificate.SerialNumber.Cmp(newMaterial.TLSCertificate.SerialNumber) == 0 {
					testingT.Fatalf("expected rotation when certificate nears expiry")
				}
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(testingT *testing.T) {
			certificateAuthority, issuer, request, clock := testCase.setup(testingT)
			material, err := issuer.IssueServerCertificate(ctx, certificateAuthority, request)
			if err != nil {
				testingT.Fatalf("issue server certificate: %v", err)
			}
			testCase.validate(testingT, issuer, request, certificateAuthority, clock, material, ctx)
		})
	}
}

func containsIPAddress(addresses []net.IP, candidate net.IP) bool {
	for _, address := range addresses {
		if address.Equal(candidate) {
			return true
		}
	}
	return false
}

func TestIssuedCertificateSupportsHTTPS(t *testing.T) {
	t.TempDir()
	ctx := context.Background()
	clock := newControllableClock(time.Now().UTC())
	fileSystem := NewOperatingSystemFileSystem()
	certificateDirectory := t.TempDir()
	certificateAuthorityConfiguration := CertificateAuthorityConfiguration{
		DirectoryPath:                    certificateDirectory,
		CertificateFileName:              "root_ca.pem",
		PrivateKeyFileName:               "root_ca.key",
		DirectoryPermissions:             0o700,
		CertificateFilePermissions:       0o600,
		PrivateKeyFilePermissions:        0o600,
		RSAKeyBitSize:                    2048,
		CertificateValidityDuration:      90 * 24 * time.Hour,
		CertificateRenewalWindowDuration: 24 * time.Hour,
		SubjectCommonName:                DefaultCertificateAuthorityCommonName,
		SubjectOrganizationalUnit:        DefaultCertificateAuthorityOrganizationalUnit,
		SubjectOrganization:              DefaultCertificateAuthorityOrganization,
	}
	certificateAuthorityManager := NewCertificateAuthorityManager(fileSystem, clock, rand.Reader, certificateAuthorityConfiguration)
	certificateAuthorityMaterial, err := certificateAuthorityManager.EnsureCertificateAuthority(ctx)
	if err != nil {
		t.Fatalf("ensure certificate authority: %v", err)
	}

	issuerConfiguration := ServerCertificateConfiguration{
		CertificateValidityDuration:      48 * time.Hour,
		CertificateRenewalWindowDuration: 12 * time.Hour,
		LeafPrivateKeyBitSize:            2048,
		CertificateFilePermissions:       0o600,
		PrivateKeyFilePermissions:        0o600,
	}
	issuer := NewServerCertificateIssuer(fileSystem, clock, rand.Reader, issuerConfiguration)
	leafCertificatePath := filepath.Join(certificateDirectory, "leaf_cert.pem")
	leafKeyPath := filepath.Join(certificateDirectory, "leaf_key.pem")
	serverCertificateRequest := ServerCertificateRequest{
		Hosts:                 []string{"localhost", "127.0.0.1"},
		CertificateOutputPath: leafCertificatePath,
		PrivateKeyOutputPath:  leafKeyPath,
	}
	leafMaterial, issueErr := issuer.IssueServerCertificate(ctx, certificateAuthorityMaterial, serverCertificateRequest)
	if issueErr != nil {
		t.Fatalf("issue server certificate: %v", issueErr)
	}

	tlsCertificate, parseErr := tls.X509KeyPair(leafMaterial.CertificateBytes, leafMaterial.PrivateKeyBytes)
	if parseErr != nil {
		t.Fatalf("parse tls certificate: %v", parseErr)
	}

	certificatePool := x509.NewCertPool()
	if !certificatePool.AppendCertsFromPEM(certificateAuthorityMaterial.CertificateBytes) {
		t.Fatalf("append certificate authority to pool")
	}

	handlerExecuted := false
	serverInstance := httptest.NewUnstartedServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		handlerExecuted = true
		_, _ = responseWriter.Write([]byte("ok"))
	}))
	serverInstance.TLS = &tls.Config{Certificates: []tls.Certificate{tlsCertificate}}
	serverInstance.StartTLS()
	defer serverInstance.Close()

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: certificatePool},
		},
	}
	response, requestErr := httpClient.Get(serverInstance.URL)
	if requestErr != nil {
		t.Fatalf("perform https request: %v", requestErr)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code %d", response.StatusCode)
	}
	if !handlerExecuted {
		t.Fatalf("expected handler to execute")
	}
}
