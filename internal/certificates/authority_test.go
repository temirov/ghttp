package certificates

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type controllableClock struct {
	currentTime time.Time
}

func newControllableClock(initialTime time.Time) *controllableClock {
	return &controllableClock{currentTime: initialTime}
}

func (clock *controllableClock) Now() time.Time {
	return clock.currentTime
}

func (clock *controllableClock) Advance(duration time.Duration) {
	clock.currentTime = clock.currentTime.Add(duration)
}

func TestEnsureCertificateAuthority(t *testing.T) {
	ctx := context.Background()
	testCases := []struct {
		name     string
		setup    func() (CertificateAuthorityManager, *controllableClock, CertificateAuthorityConfiguration)
		validate func(testingT *testing.T, manager CertificateAuthorityManager, clock *controllableClock, configuration CertificateAuthorityConfiguration, material CertificateAuthorityMaterial, ctx context.Context)
	}{
		{
			name: "creates certificate authority when missing",
			setup: func() (CertificateAuthorityManager, *controllableClock, CertificateAuthorityConfiguration) {
				initialTime := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
				clock := newControllableClock(initialTime)
				temporaryDirectory := t.TempDir()
				configuration := CertificateAuthorityConfiguration{
					DirectoryPath:                    temporaryDirectory,
					CertificateFileName:              "root_ca.pem",
					PrivateKeyFileName:               "root_ca.key",
					DirectoryPermissions:             0o700,
					CertificateFilePermissions:       0o600,
					PrivateKeyFilePermissions:        0o600,
					RSAKeyBitSize:                    2048,
					CertificateValidityDuration:      24 * time.Hour,
					CertificateRenewalWindowDuration: 6 * time.Hour,
					SubjectCommonName:                DefaultCertificateAuthorityCommonName,
					SubjectOrganizationalUnit:        DefaultCertificateAuthorityOrganizationalUnit,
					SubjectOrganization:              DefaultCertificateAuthorityOrganization,
				}
				fileSystem := NewOperatingSystemFileSystem()
				manager := NewCertificateAuthorityManager(fileSystem, clock, rand.Reader, configuration)
				return manager, clock, configuration
			},
			validate: func(testingT *testing.T, manager CertificateAuthorityManager, clock *controllableClock, configuration CertificateAuthorityConfiguration, material CertificateAuthorityMaterial, ctx context.Context) {
				testingT.Helper()
				if len(material.CertificateBytes) == 0 {
					testingT.Fatalf("expected certificate bytes to be written")
				}
				if material.Certificate == nil {
					testingT.Fatalf("expected certificate to be parsed")
				}
				expectedMinimumValidity := configuration.CertificateValidityDuration
				actualValidity := material.Certificate.NotAfter.Sub(material.Certificate.NotBefore)
				if actualValidity < expectedMinimumValidity {
					testingT.Fatalf("expected certificate validity of at least %s, got %s", expectedMinimumValidity, actualValidity)
				}
				certificatePath := filepath.Join(configuration.DirectoryPath, configuration.CertificateFileName)
				privateKeyPath := filepath.Join(configuration.DirectoryPath, configuration.PrivateKeyFileName)
				assertFilePermissions(testingT, certificatePath, configuration.CertificateFilePermissions)
				assertFilePermissions(testingT, privateKeyPath, configuration.PrivateKeyFilePermissions)
			},
		},
		{
			name: "reuses existing certificate authority when valid",
			setup: func() (CertificateAuthorityManager, *controllableClock, CertificateAuthorityConfiguration) {
				initialTime := time.Date(2025, 2, 1, 9, 0, 0, 0, time.UTC)
				clock := newControllableClock(initialTime)
				temporaryDirectory := t.TempDir()
				configuration := CertificateAuthorityConfiguration{
					DirectoryPath:                    temporaryDirectory,
					CertificateFileName:              "root_ca.pem",
					PrivateKeyFileName:               "root_ca.key",
					DirectoryPermissions:             0o700,
					CertificateFilePermissions:       0o600,
					PrivateKeyFilePermissions:        0o600,
					RSAKeyBitSize:                    2048,
					CertificateValidityDuration:      48 * time.Hour,
					CertificateRenewalWindowDuration: 12 * time.Hour,
					SubjectCommonName:                DefaultCertificateAuthorityCommonName,
					SubjectOrganizationalUnit:        DefaultCertificateAuthorityOrganizationalUnit,
					SubjectOrganization:              DefaultCertificateAuthorityOrganization,
				}
				fileSystem := NewOperatingSystemFileSystem()
				manager := NewCertificateAuthorityManager(fileSystem, clock, rand.Reader, configuration)
				return manager, clock, configuration
			},
			validate: func(testingT *testing.T, manager CertificateAuthorityManager, clock *controllableClock, configuration CertificateAuthorityConfiguration, material CertificateAuthorityMaterial, ctx context.Context) {
				testingT.Helper()
				clock.Advance(configuration.CertificateValidityDuration / 4)
				secondMaterial, err := manager.EnsureCertificateAuthority(ctx)
				if err != nil {
					testingT.Fatalf("ensure certificate authority second time: %v", err)
				}
				if material.Certificate.SerialNumber.Cmp(secondMaterial.Certificate.SerialNumber) != 0 {
					testingT.Fatalf("expected identical serial numbers for reused certificate")
				}
			},
		},
		{
			name: "rotates certificate authority near expiry",
			setup: func() (CertificateAuthorityManager, *controllableClock, CertificateAuthorityConfiguration) {
				initialTime := time.Date(2025, 3, 1, 8, 0, 0, 0, time.UTC)
				clock := newControllableClock(initialTime)
				temporaryDirectory := t.TempDir()
				configuration := CertificateAuthorityConfiguration{
					DirectoryPath:                    temporaryDirectory,
					CertificateFileName:              "root_ca.pem",
					PrivateKeyFileName:               "root_ca.key",
					DirectoryPermissions:             0o700,
					CertificateFilePermissions:       0o600,
					PrivateKeyFilePermissions:        0o600,
					RSAKeyBitSize:                    2048,
					CertificateValidityDuration:      36 * time.Hour,
					CertificateRenewalWindowDuration: 6 * time.Hour,
					SubjectCommonName:                DefaultCertificateAuthorityCommonName,
					SubjectOrganizationalUnit:        DefaultCertificateAuthorityOrganizationalUnit,
					SubjectOrganization:              DefaultCertificateAuthorityOrganization,
				}
				fileSystem := NewOperatingSystemFileSystem()
				manager := NewCertificateAuthorityManager(fileSystem, clock, rand.Reader, configuration)
				return manager, clock, configuration
			},
			validate: func(testingT *testing.T, manager CertificateAuthorityManager, clock *controllableClock, configuration CertificateAuthorityConfiguration, material CertificateAuthorityMaterial, ctx context.Context) {
				testingT.Helper()
				clock.Advance(configuration.CertificateValidityDuration - configuration.CertificateRenewalWindowDuration + time.Minute)
				newMaterial, err := manager.EnsureCertificateAuthority(ctx)
				if err != nil {
					testingT.Fatalf("ensure certificate authority after rotation: %v", err)
				}
				if material.Certificate.SerialNumber.Cmp(newMaterial.Certificate.SerialNumber) == 0 {
					testingT.Fatalf("expected rotated certificate to use different serial number")
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(testingT *testing.T) {
			manager, clock, configuration := testCase.setup()
			material, err := manager.EnsureCertificateAuthority(ctx)
			if err != nil {
				testingT.Fatalf("ensure certificate authority: %v", err)
			}
			testCase.validate(testingT, manager, clock, configuration, material, ctx)
		})
	}
}

func assertFilePermissions(testingT *testing.T, path string, expectedPermissions os.FileMode) {
	testingT.Helper()
	fileInfo, err := os.Stat(path)
	if err != nil {
		testingT.Fatalf("stat %s: %v", path, err)
	}
	if fileInfo.Mode().Perm() != expectedPermissions {
		testingT.Fatalf("expected permissions %v, got %v", expectedPermissions, fileInfo.Mode().Perm())
	}
}
