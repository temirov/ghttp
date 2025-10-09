package app

import (
	"context"
	"os"
	pathpkg "path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/temirov/ghttp/pkg/logging"
)

func TestPrepareServeConfigurationRejectsHTTPSWithTLSFiles(t *testing.T) {
	temporaryDirectory := t.TempDir()
	configurationManager := viper.New()
	configurationManager.Set(configKeyServeBindAddress, "")
	configurationManager.Set(configKeyServeDirectory, temporaryDirectory)
	configurationManager.Set(configKeyServeProtocol, "HTTP/1.1")
	configurationManager.Set(configKeyServePort, "8080")
	configurationManager.Set(configKeyServeTLSCertificatePath, "cert.pem")
	configurationManager.Set(configKeyServeTLSKeyPath, "key.pem")
	configurationManager.Set(configKeyServeHTTPS, true)

	resources := &applicationResources{
		configurationManager: configurationManager,
		loggingService:       logging.NewTestService(logging.TypeConsole),
		defaultConfigDirPath: temporaryDirectory,
	}

	command := &cobra.Command{}
	command.SetContext(context.WithValue(context.Background(), contextKeyApplicationResources, resources))

	err := prepareServeConfiguration(command, nil, configKeyServePort, true)
	if err == nil {
		t.Fatalf("expected error when https flag is combined with tls certificate paths")
	}
	if !strings.Contains(err.Error(), "cannot combine https flag") {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestPrepareServeConfigurationNoMarkdownFlagDisablesRendering(t *testing.T) {
	temporaryDirectory := t.TempDir()
	configurationManager := viper.New()
	configurationManager.Set(configKeyServeBindAddress, "")
	configurationManager.Set(configKeyServeDirectory, temporaryDirectory)
	configurationManager.Set(configKeyServeProtocol, "HTTP/1.1")
	configurationManager.Set(configKeyServePort, "8000")
	configurationManager.Set(configKeyServeNoMarkdown, true)
	configurationManager.Set(configKeyServeLoggingType, logging.TypeConsole)

	resources := &applicationResources{
		configurationManager: configurationManager,
		loggingService:       logging.NewTestService(logging.TypeConsole),
		defaultConfigDirPath: temporaryDirectory,
	}

	command := &cobra.Command{}
	command.SetContext(context.WithValue(context.Background(), contextKeyApplicationResources, resources))

	err := prepareServeConfiguration(command, nil, configKeyServePort, true)
	if err != nil {
		t.Fatalf("prepare serve configuration: %v", err)
	}

	configurationValue := command.Context().Value(contextKeyServeConfiguration)
	serveConfiguration, ok := configurationValue.(ServeConfiguration)
	if !ok {
		t.Fatalf("serve configuration stored with unexpected type")
	}
	if serveConfiguration.EnableMarkdown {
		t.Fatalf("expected markdown rendering to be disabled")
	}
	if serveConfiguration.LoggingType != logging.TypeConsole {
		t.Fatalf("expected logging type console, got %s", serveConfiguration.LoggingType)
	}
}

func TestPrepareServeConfigurationRejectsInvalidLoggingType(t *testing.T) {
	temporaryDirectory := t.TempDir()
	configurationManager := viper.New()
	configurationManager.Set(configKeyServeBindAddress, "")
	configurationManager.Set(configKeyServeDirectory, temporaryDirectory)
	configurationManager.Set(configKeyServeProtocol, "HTTP/1.1")
	configurationManager.Set(configKeyServePort, "8000")
	configurationManager.Set(configKeyServeLoggingType, "xml")

	resources := &applicationResources{
		configurationManager: configurationManager,
		loggingService:       logging.NewTestService(logging.TypeConsole),
		defaultConfigDirPath: temporaryDirectory,
	}

	command := &cobra.Command{}
	command.SetContext(context.WithValue(context.Background(), contextKeyApplicationResources, resources))

	err := prepareServeConfiguration(command, nil, configKeyServePort, true)
	if err == nil {
		t.Fatalf("expected error for invalid logging type")
	}
	if !strings.Contains(err.Error(), "unsupported logging type") {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestPrepareServeConfigurationBrowseOverridesDirectoryListing(t *testing.T) {
	temporaryDirectory := t.TempDir()
	configurationManager := viper.New()
	configurationManager.Set(configKeyServeBindAddress, "")
	configurationManager.Set(configKeyServeDirectory, temporaryDirectory)
	configurationManager.Set(configKeyServeProtocol, "HTTP/1.1")
	configurationManager.Set(configKeyServePort, "8000")
	configurationManager.Set(configKeyServeBrowse, true)

	resources := &applicationResources{
		configurationManager: configurationManager,
		loggingService:       logging.NewTestService(logging.TypeConsole),
		defaultConfigDirPath: temporaryDirectory,
	}

	command := &cobra.Command{}
	command.SetContext(context.WithValue(context.Background(), contextKeyApplicationResources, resources))

	t.Setenv(environmentVariableDisableDirectoryListing, "1")

	err := prepareServeConfiguration(command, nil, configKeyServePort, true)
	if err != nil {
		t.Fatalf("prepare serve configuration: %v", err)
	}

	configurationValue := command.Context().Value(contextKeyServeConfiguration)
	serveConfiguration, ok := configurationValue.(ServeConfiguration)
	if !ok {
		t.Fatalf("serve configuration stored with unexpected type")
	}
	if !serveConfiguration.BrowseDirectories {
		t.Fatalf("expected browse directories to be enabled")
	}
	if serveConfiguration.DisableDirectoryListing {
		t.Fatalf("expected directory listing to remain enabled for browse mode")
	}
}

func TestPrepareServeConfigurationAcceptsInitialFileArgument(t *testing.T) {
	temporaryDirectory := t.TempDir()
	initialFilePath := pathpkg.Join(temporaryDirectory, "cat.html")
	writeErr := os.WriteFile(initialFilePath, []byte("<html></html>"), 0o600)
	if writeErr != nil {
		t.Fatalf("write initial file: %v", writeErr)
	}

	configurationManager := viper.New()
	configurationManager.Set(configKeyServeBindAddress, "")
	configurationManager.Set(configKeyServeDirectory, temporaryDirectory)
	configurationManager.Set(configKeyServeProtocol, "HTTP/1.1")
	configurationManager.Set(configKeyServePort, "")

	resources := &applicationResources{
		configurationManager: configurationManager,
		loggingService:       logging.NewTestService(logging.TypeConsole),
		defaultConfigDirPath: temporaryDirectory,
	}

	command := &cobra.Command{}
	command.SetContext(context.WithValue(context.Background(), contextKeyApplicationResources, resources))

	err := prepareServeConfiguration(command, []string{initialFilePath}, configKeyServePort, true)
	if err != nil {
		t.Fatalf("prepare serve configuration: %v", err)
	}

	configurationValue := command.Context().Value(contextKeyServeConfiguration)
	serveConfiguration, ok := configurationValue.(ServeConfiguration)
	if !ok {
		t.Fatalf("serve configuration stored with unexpected type")
	}
	if serveConfiguration.DirectoryPath != temporaryDirectory {
		t.Fatalf("expected directory path %s, got %s", temporaryDirectory, serveConfiguration.DirectoryPath)
	}
	if serveConfiguration.InitialFileRelativePath != pathpkg.Base(initialFilePath) {
		t.Fatalf("expected initial file cat.html, got %s", serveConfiguration.InitialFileRelativePath)
	}
	if serveConfiguration.Port != defaultServePort {
		t.Fatalf("expected default port %s, got %s", defaultServePort, serveConfiguration.Port)
	}
}
