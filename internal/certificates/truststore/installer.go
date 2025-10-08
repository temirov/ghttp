package truststore

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/temirov/ghttp/internal/certificates"
)

const (
	commandNameSecurity = "security"
	commandNameCertutil = "certutil"
	commandNameTrust    = "trust"
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
	FirefoxProfileDirectories       []string
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
	fileSystem    certificates.FileSystem
	configuration Configuration
}

func newMacOSInstaller(commandRunner certificates.CommandRunner, fileSystem certificates.FileSystem, configuration Configuration) (Installer, error) {
	if configuration.CertificateCommonName == "" {
		return nil, errors.New("macos installer requires certificate common name")
	}
	keychainPath := configuration.MacOSKeychainPath
	if keychainPath == "" {
		homeDirectory, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return nil, fmt.Errorf("resolve home directory: %w", homeErr)
		}
		keychainPath = filepath.Join(homeDirectory, "Library", "Keychains", "login.keychain-db")
	}
	configuration.MacOSKeychainPath = keychainPath
	return &macOSInstaller{
		commandRunner: commandRunner,
		fileSystem:    fileSystem,
		configuration: configuration,
	}, nil
}

func (installer *macOSInstaller) Install(ctx context.Context, certificatePath string) error {
	if certificatePath == "" {
		return errors.New("certificate path is required")
	}
	arguments := []string{"add-trusted-cert", "-r", "trustRoot", "-k", installer.configuration.MacOSKeychainPath, certificatePath}
	err := installer.commandRunner.Run(ctx, commandNameSecurity, arguments)
	if err != nil {
		return fmt.Errorf("install certificate in macos keychain: %w", err)
	}
	firefoxErr := integrateFirefoxCertificates(ctx, installer.commandRunner, installer.fileSystem, installer.configuration, certificatePath)
	if firefoxErr != nil {
		return fmt.Errorf("configure firefox trust stores: %w", firefoxErr)
	}
	return nil
}

func (installer *macOSInstaller) Uninstall(ctx context.Context) error {
	arguments := []string{"delete-certificate", "-c", installer.configuration.CertificateCommonName, installer.configuration.MacOSKeychainPath}
	err := installer.commandRunner.Run(ctx, commandNameSecurity, arguments)
	if err != nil {
		return fmt.Errorf("remove certificate from macos keychain: %w", err)
	}
	firefoxErr := removeFirefoxCertificates(ctx, installer.commandRunner, installer.fileSystem, installer.configuration)
	if firefoxErr != nil {
		return fmt.Errorf("remove firefox trust stores: %w", firefoxErr)
	}
	return nil
}

type linuxInstaller struct {
	commandRunner certificates.CommandRunner
	fileSystem    certificates.FileSystem
	configuration Configuration
	installedPath string
}

func newLinuxInstaller(commandRunner certificates.CommandRunner, fileSystem certificates.FileSystem, configuration Configuration) (Installer, error) {
	if configuration.LinuxCertificateFilePermissions == 0 {
		configuration.LinuxCertificateFilePermissions = 0o644
	}
	return &linuxInstaller{
		commandRunner: commandRunner,
		fileSystem:    fileSystem,
		configuration: configuration,
	}, nil
}

func (installer *linuxInstaller) Install(ctx context.Context, certificatePath string) error {
	if certificatePath == "" {
		return errors.New("certificate path is required")
	}
	exists, existsErr := installer.fileSystem.FileExists(certificatePath)
	if existsErr != nil {
		return fmt.Errorf("check certificate path: %w", existsErr)
	}
	if !exists {
		return fmt.Errorf("certificate path does not exist: %s", certificatePath)
	}
	anchorPath := certificatePath
	if installer.configuration.LinuxCertificateDestinationPath != "" {
		destinationDirectory := filepath.Dir(installer.configuration.LinuxCertificateDestinationPath)
		if err := installer.fileSystem.EnsureDirectory(destinationDirectory, 0o755); err != nil {
			return fmt.Errorf("ensure linux certificate directory: %w", err)
		}
		content, readErr := installer.fileSystem.ReadFile(certificatePath)
		if readErr != nil {
			return fmt.Errorf("read certificate: %w", readErr)
		}
		if err := installer.fileSystem.WriteFile(installer.configuration.LinuxCertificateDestinationPath, content, installer.configuration.LinuxCertificateFilePermissions); err != nil {
			return fmt.Errorf("write linux trust certificate: %w", err)
		}
		anchorPath = installer.configuration.LinuxCertificateDestinationPath
	}
	installErr := installer.commandRunner.Run(ctx, commandNameTrust, []string{"anchor", anchorPath})
	if installErr != nil {
		return fmt.Errorf("configure linux trust store: %w", installErr)
	}
	installer.installedPath = anchorPath
	firefoxErr := integrateFirefoxCertificates(ctx, installer.commandRunner, installer.fileSystem, installer.configuration, certificatePath)
	if firefoxErr != nil {
		return fmt.Errorf("configure firefox trust stores: %w", firefoxErr)
	}
	return nil
}

func (installer *linuxInstaller) Uninstall(ctx context.Context) error {
	anchorPath := installer.installedPath
	if anchorPath == "" {
		anchorPath = installer.configuration.LinuxCertificateDestinationPath
	}
	if anchorPath == "" {
		return errors.New("linux installer has no recorded certificate path")
	}
	removeErr := installer.commandRunner.Run(ctx, commandNameTrust, []string{"anchor", "--remove", anchorPath})
	if removeErr != nil {
		return fmt.Errorf("remove linux trust store certificate: %w", removeErr)
	}
	if installer.configuration.LinuxCertificateDestinationPath != "" {
		if err := installer.fileSystem.Remove(installer.configuration.LinuxCertificateDestinationPath); err != nil {
			return fmt.Errorf("remove linux certificate file: %w", err)
		}
	}
	firefoxErr := removeFirefoxCertificates(ctx, installer.commandRunner, installer.fileSystem, installer.configuration)
	if firefoxErr != nil {
		return fmt.Errorf("remove firefox trust stores: %w", firefoxErr)
	}
	return nil
}

type windowsInstaller struct {
	commandRunner certificates.CommandRunner
	fileSystem    certificates.FileSystem
	configuration Configuration
}

func newWindowsInstaller(commandRunner certificates.CommandRunner, fileSystem certificates.FileSystem, configuration Configuration) (Installer, error) {
	storeName := configuration.WindowsCertificateStoreName
	if storeName == "" {
		storeName = "Root"
	}
	if configuration.CertificateCommonName == "" {
		return nil, errors.New("windows installer requires certificate common name")
	}
	configuration.WindowsCertificateStoreName = storeName
	return &windowsInstaller{
		commandRunner: commandRunner,
		fileSystem:    fileSystem,
		configuration: configuration,
	}, nil
}

func (installer *windowsInstaller) Install(ctx context.Context, certificatePath string) error {
	if certificatePath == "" {
		return errors.New("certificate path is required")
	}
	arguments := []string{"-user", "-addstore", "-f", installer.configuration.WindowsCertificateStoreName, certificatePath}
	err := installer.commandRunner.Run(ctx, commandNameCertutil, arguments)
	if err != nil {
		return fmt.Errorf("install certificate in windows store: %w", err)
	}
	firefoxErr := integrateFirefoxCertificates(ctx, installer.commandRunner, installer.fileSystem, installer.configuration, certificatePath)
	if firefoxErr != nil {
		return fmt.Errorf("configure firefox trust stores: %w", firefoxErr)
	}
	return nil
}

func (installer *windowsInstaller) Uninstall(ctx context.Context) error {
	arguments := []string{"-user", "-delstore", installer.configuration.WindowsCertificateStoreName, installer.configuration.CertificateCommonName}
	err := installer.commandRunner.Run(ctx, commandNameCertutil, arguments)
	if err != nil {
		return fmt.Errorf("remove certificate from windows store: %w", err)
	}
	firefoxErr := removeFirefoxCertificates(ctx, installer.commandRunner, installer.fileSystem, installer.configuration)
	if firefoxErr != nil {
		return fmt.Errorf("remove firefox trust stores: %w", firefoxErr)
	}
	return nil
}
