package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"github.com/temirov/ghttp/internal/certificates"
	"github.com/temirov/ghttp/pkg/logging"
)

type contextKey string

const (
	contextKeyApplicationResources contextKey = "application-resources"
	contextKeyServeConfiguration   contextKey = "serve-configuration"
	contextKeyHTTPSHosts           contextKey = "https-hosts"
	contextKeyHTTPSCertificateDir  contextKey = "https-certificate-directory"

	defaultServePort       = "8000"
	defaultHTTPSServePort  = "8443"
	defaultProtocolVersion = "HTTP/1.1"
	defaultConfigFileName  = "config"
	defaultConfigFileType  = "yaml"
	defaultApplicationName = "ghttp"

	flagNameConfigFile         = "config"
	flagNameBindAddress        = "bind"
	flagNameDirectory          = "directory"
	flagNameProtocol           = "protocol"
	flagNameTLSCertificatePath = "tls-cert"
	flagNameTLSKeyPath         = "tls-key"
	flagNameNoMarkdown         = "no-md"
	flagNameHTTPS              = "https"
	flagNameBrowse             = "browse"
	flagNameLoggingType        = "logging-type"
	flagNameCertificateDir     = "cert-dir"
	flagNameHTTPSHosts         = "host"

	configKeyServeBindAddress        = "serve.bind_address"
	configKeyServeDirectory          = "serve.directory"
	configKeyServeProtocol           = "serve.protocol"
	configKeyServePort               = "serve.port"
	configKeyServeTLSCertificatePath = "serve.tls_certificate"
	configKeyServeTLSKeyPath         = "serve.tls_private_key"
	configKeyServeNoMarkdown         = "serve.no_markdown"
	configKeyServeBrowse             = "serve.browse"
	configKeyServeHTTPS              = "serve.https"
	configKeyServeLoggingType        = "serve.logging_type"
	configKeyHTTPSCertificateDir     = "https.certificate_directory"
	configKeyHTTPSHosts              = "https.hosts"
	configKeyHTTPSPort               = "https.port"

	logMessageFailedInitializeLogger = "failed to initialize logger"
	logMessageResolveUserConfigDir   = "resolve user config directory"
	logMessageCommandExecutionFailed = "command execution failed"
)

type applicationResources struct {
	configurationManager *viper.Viper
	loggingService       *logging.Service
	defaultConfigDirPath string
}

func (resources *applicationResources) updateLogger(loggingType string) error {
	normalizedType, err := logging.NormalizeType(loggingType)
	if err != nil {
		return err
	}
	if resources.loggingService != nil && resources.loggingService.Type() == normalizedType {
		return nil
	}
	service, err := logging.NewService(normalizedType)
	if err != nil {
		return err
	}
	if resources.loggingService != nil {
		_ = resources.loggingService.Sync()
	}
	resources.loggingService = service
	return nil
}

func (resources *applicationResources) loggingType() string {
	if resources.loggingService == nil {
		return logging.TypeConsole
	}
	return resources.loggingService.Type()
}

// Execute runs the CLI using the provided context and arguments, returning an exit code.
func Execute(ctx context.Context, arguments []string) int {
	initialService, err := logging.NewService(logging.TypeConsole)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", logMessageFailedInitializeLogger, err)
		return 1
	}
	configurationManager := viper.New()
	configurationManager.SetEnvPrefix(strings.ToUpper(defaultApplicationName))
	configurationManager.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	configurationManager.AutomaticEnv()

	userConfigDir, userConfigErr := os.UserConfigDir()
	if userConfigErr != nil {
		initialService.Error(logMessageResolveUserConfigDir, userConfigErr)
		return 1
	}
	applicationConfigDir := filepath.Join(userConfigDir, defaultApplicationName)

	configurationManager.SetDefault(configKeyServeBindAddress, "")
	configurationManager.SetDefault(configKeyServeDirectory, ".")
	configurationManager.SetDefault(configKeyServeProtocol, defaultProtocolVersion)
	configurationManager.SetDefault(configKeyServePort, defaultServePort)
	configurationManager.SetDefault(configKeyServeTLSCertificatePath, "")
	configurationManager.SetDefault(configKeyServeTLSKeyPath, "")
	configurationManager.SetDefault(configKeyServeNoMarkdown, false)
	configurationManager.SetDefault(configKeyServeBrowse, false)
	configurationManager.SetDefault(configKeyServeHTTPS, false)
	configurationManager.SetDefault(configKeyServeLoggingType, logging.TypeConsole)
	configurationManager.SetDefault(configKeyHTTPSCertificateDir, filepath.Join(applicationConfigDir, certificates.DefaultCertificateDirectoryName))
	configurationManager.SetDefault(configKeyHTTPSHosts, []string{"localhost", "127.0.0.1", "::1"})
	configurationManager.SetDefault(configKeyHTTPSPort, defaultHTTPSServePort)
	resources := &applicationResources{
		configurationManager: configurationManager,
		loggingService:       initialService,
		defaultConfigDirPath: applicationConfigDir,
	}
	if err := resources.updateLogger(configurationManager.GetString(configKeyServeLoggingType)); err != nil {
		resources.loggingService = initialService
		resources.loggingService.Error(logMessageFailedInitializeLogger, err)
		return 1
	}
	defer func() {
		if resources.loggingService != nil {
			_ = resources.loggingService.Sync()
		}
	}()

	rootCommand := newRootCommand(resources)
	baseContext := context.WithValue(ctx, contextKeyApplicationResources, resources)
	rootCommand.SetContext(baseContext)
	rootCommand.SetArgs(arguments)

	if executionErr := rootCommand.Execute(); executionErr != nil {
		resources.loggingService.Error(logMessageCommandExecutionFailed, executionErr)
		return 1
	}

	return 0
}
