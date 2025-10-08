package truststore

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"runtime"

	"github.com/temirov/ghttp/internal/certificates"
)

const (
	commandNameSecurity             = "security"
	commandNameCertutil             = "certutil"
	commandNameUpdateCaCertificates = "update-ca-certificates"
	commandNameTrust                = "trust"
)

// Installer provisions and removes certificates from operating system trust stores.
type Installer interface {
	Install(ctx context.Context, certificatePath string) error
	Uninstall(ctx context.Context) error
}

// Configuration controls installer behavior across platforms.
type Configuration struct {
	CertificateCommonName           string
	MacOSKeychainPath               string
	LinuxCertificateDestinationPath string
	LinuxCertificateFilePermissions fs.FileMode
	WindowsCertificateStoreName     string
}

type installerFactory func(commandRunner certificates.CommandRunner, fileSystem certificates.FileSystem, configuration Configuration) (Installer, error)

var supportedFactories = map[string]installerFactory{
	"darwin":  newMacOSInstaller,
	"linux":   newLinuxInstaller,
	"windows": newWindowsInstaller,
}

// NewInstaller constructs the platform-specific Installer.
func NewInstaller(commandRunner certificates.CommandRunner, fileSystem certificates.FileSystem, configuration Configuration) (Installer, error) {
	factory, found := supportedFactories[runtime.GOOS]
	if !found {
		return nil, fmt.Errorf("unsupported operating system %s", runtime.GOOS)
	}
	return factory(commandRunner, fileSystem, configuration)
}

type macOSInstaller struct {
	commandRunner certificates.CommandRunner
	configuration Configuration
}

func newMacOSInstaller(commandRunner certificates.CommandRunner, _ certificates.FileSystem, configuration Configuration) (Installer, error) {
	if configuration.CertificateCommonName == "" {
		return nil, errors.New("macos installer requires certificate common name")
	}
	keychainPath := configuration.MacOSKeychainPath
	if keychainPath == "" {
		keychainPath = "/Library/Keychains/System.keychain"
	}
	configuration.MacOSKeychainPath = keychainPath
	return macOSInstaller{
		commandRunner: commandRunner,
		configuration: configuration,
	}, nil
}

func (installer macOSInstaller) Install(ctx context.Context, certificatePath string) error {
	if certificatePath == "" {
		return errors.New("certificate path is required")
	}
	arguments := []string{"add-trusted-cert", "-d", "-r", "trustRoot", "-k", installer.configuration.MacOSKeychainPath, certificatePath}
	err := installer.commandRunner.Run(ctx, commandNameSecurity, arguments)
	if err != nil {
		return fmt.Errorf("install certificate in macos keychain: %w", err)
	}
	return nil
}

func (installer macOSInstaller) Uninstall(ctx context.Context) error {
	arguments := []string{"delete-certificate", "-c", installer.configuration.CertificateCommonName, installer.configuration.MacOSKeychainPath}
	err := installer.commandRunner.Run(ctx, commandNameSecurity, arguments)
	if err != nil {
		return fmt.Errorf("remove certificate from macos keychain: %w", err)
	}
	return nil
}

type linuxInstaller struct {
	commandRunner certificates.CommandRunner
	fileSystem    certificates.FileSystem
	configuration Configuration
}

func newLinuxInstaller(commandRunner certificates.CommandRunner, fileSystem certificates.FileSystem, configuration Configuration) (Installer, error) {
	if configuration.LinuxCertificateDestinationPath == "" {
		return nil, errors.New("linux installer requires destination path")
	}
	if configuration.LinuxCertificateFilePermissions == 0 {
		configuration.LinuxCertificateFilePermissions = 0o644
	}
	return linuxInstaller{
		commandRunner: commandRunner,
		fileSystem:    fileSystem,
		configuration: configuration,
	}, nil
}

func (installer linuxInstaller) Install(ctx context.Context, certificatePath string) error {
	if certificatePath == "" {
		return errors.New("certificate path is required")
	}
	certificateBytes, readErr := installer.fileSystem.ReadFile(certificatePath)
	if readErr != nil {
		return fmt.Errorf("read certificate for linux install: %w", readErr)
	}
	writeErr := installer.fileSystem.WriteFile(installer.configuration.LinuxCertificateDestinationPath, certificateBytes, installer.configuration.LinuxCertificateFilePermissions)
	if writeErr != nil {
		return fmt.Errorf("write linux trust store certificate: %w", writeErr)
	}
	err := installer.commandRunner.Run(ctx, commandNameUpdateCaCertificates, []string{})
	if err != nil {
		trustErr := installer.commandRunner.Run(ctx, commandNameTrust, []string{"anchor", installer.configuration.LinuxCertificateDestinationPath})
		if trustErr != nil {
			return fmt.Errorf("update linux trust store: %w", errors.Join(err, trustErr))
		}
	}
	return nil
}

func (installer linuxInstaller) Uninstall(ctx context.Context) error {
	removeErr := installer.fileSystem.Remove(installer.configuration.LinuxCertificateDestinationPath)
	if removeErr != nil {
		return fmt.Errorf("remove linux trust store certificate: %w", removeErr)
	}
	err := installer.commandRunner.Run(ctx, commandNameUpdateCaCertificates, []string{})
	if err != nil {
		trustErr := installer.commandRunner.Run(ctx, commandNameTrust, []string{"anchor", "--remove", installer.configuration.LinuxCertificateDestinationPath})
		if trustErr != nil {
			return fmt.Errorf("update linux trust store removal: %w", errors.Join(err, trustErr))
		}
	}
	return nil
}

type windowsInstaller struct {
	commandRunner certificates.CommandRunner
	configuration Configuration
}

func newWindowsInstaller(commandRunner certificates.CommandRunner, _ certificates.FileSystem, configuration Configuration) (Installer, error) {
	storeName := configuration.WindowsCertificateStoreName
	if storeName == "" {
		storeName = "Root"
	}
	if configuration.CertificateCommonName == "" {
		return nil, errors.New("windows installer requires certificate common name")
	}
	configuration.WindowsCertificateStoreName = storeName
	return windowsInstaller{
		commandRunner: commandRunner,
		configuration: configuration,
	}, nil
}

func (installer windowsInstaller) Install(ctx context.Context, certificatePath string) error {
	if certificatePath == "" {
		return errors.New("certificate path is required")
	}
	arguments := []string{"-addstore", "-f", installer.configuration.WindowsCertificateStoreName, certificatePath}
	err := installer.commandRunner.Run(ctx, commandNameCertutil, arguments)
	if err != nil {
		return fmt.Errorf("install certificate in windows store: %w", err)
	}
	return nil
}

func (installer windowsInstaller) Uninstall(ctx context.Context) error {
	arguments := []string{"-delstore", installer.configuration.WindowsCertificateStoreName, installer.configuration.CertificateCommonName}
	err := installer.commandRunner.Run(ctx, commandNameCertutil, arguments)
	if err != nil {
		return fmt.Errorf("remove certificate from windows store: %w", err)
	}
	return nil
}
