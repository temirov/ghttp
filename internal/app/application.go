package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/temirov/ghttp/internal/certificates"
	"github.com/temirov/ghttp/internal/logging"
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
	configKeyServeHTTPS              = "serve.https"
	configKeyServeLoggingType        = "serve.logging_type"
	configKeyHTTPSCertificateDir     = "https.certificate_directory"
	configKeyHTTPSHosts              = "https.hosts"
	configKeyHTTPSPort               = "https.port"
)

type applicationResources struct {
	configurationManager *viper.Viper
	logger               *zap.Logger
	defaultConfigDirPath string
	loggingType          string
}

func (resources *applicationResources) updateLogger(loggingType string) error {
	normalizedType, err := logging.NormalizeType(loggingType)
	if err != nil {
		return err
	}
	newLogger, err := logging.NewLogger(normalizedType)
	if err != nil {
		return err
	}
	if resources.logger != nil {
		_ = resources.logger.Sync()
	}
	resources.logger = newLogger
	resources.loggingType = normalizedType
	return nil
}

// Execute runs the CLI using the provided context and arguments, returning an exit code.
func Execute(ctx context.Context, arguments []string) int {
	initialLogger := logging.NewConsoleLogger()
	configurationManager := viper.New()
	configurationManager.SetEnvPrefix(strings.ToUpper(defaultApplicationName))
	configurationManager.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	configurationManager.AutomaticEnv()

	userConfigDir, userConfigErr := os.UserConfigDir()
	if userConfigErr != nil {
		initialLogger.Error("resolve user config directory", zap.Error(userConfigErr))
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
	configurationManager.SetDefault(configKeyServeHTTPS, false)
	configurationManager.SetDefault(configKeyServeLoggingType, logging.TypeConsole)
	configurationManager.SetDefault(configKeyHTTPSCertificateDir, filepath.Join(applicationConfigDir, certificates.DefaultCertificateDirectoryName))
	configurationManager.SetDefault(configKeyHTTPSHosts, []string{"localhost", "127.0.0.1", "::1"})
	configurationManager.SetDefault(configKeyHTTPSPort, defaultHTTPSServePort)
	resources := &applicationResources{
		configurationManager: configurationManager,
		logger:               initialLogger,
		defaultConfigDirPath: applicationConfigDir,
		loggingType:          logging.TypeConsole,
	}
	if err := resources.updateLogger(configurationManager.GetString(configKeyServeLoggingType)); err != nil {
		resources.logger = initialLogger
		resources.loggingType = logging.TypeConsole
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		return 1
	}
	defer func() {
		if resources.logger != nil {
			_ = resources.logger.Sync()
		}
	}()

	rootCommand := newRootCommand(resources)
	baseContext := context.WithValue(ctx, contextKeyApplicationResources, resources)
	rootCommand.SetContext(baseContext)
	rootCommand.SetArgs(arguments)

	if executionErr := rootCommand.Execute(); executionErr != nil {
		resources.logger.Error("command execution failed", zap.Error(executionErr))
		return 1
	}

	return 0
}
