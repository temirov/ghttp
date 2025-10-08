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
	flagNameCertificateDir     = "cert-dir"
	flagNameHTTPSHosts         = "host"

	configKeyServeBindAddress        = "serve.bind_address"
	configKeyServeDirectory          = "serve.directory"
	configKeyServeProtocol           = "serve.protocol"
	configKeyServePort               = "serve.port"
	configKeyServeTLSCertificatePath = "serve.tls_certificate"
	configKeyServeTLSKeyPath         = "serve.tls_private_key"
	configKeyHTTPSCertificateDir     = "https.certificate_directory"
	configKeyHTTPSHosts              = "https.hosts"
	configKeyHTTPSPort               = "https.port"
)

type applicationResources struct {
	configurationManager *viper.Viper
	logger               *zap.Logger
	defaultConfigDirPath string
}

// Execute runs the CLI using the provided context and arguments, returning an exit code.
func Execute(ctx context.Context, arguments []string) int {
	logger, loggerErr := zap.NewProduction()
	if loggerErr != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", loggerErr)
		return 1
	}
	defer func() {
		_ = logger.Sync()
	}()

	configurationManager := viper.New()
	configurationManager.SetEnvPrefix(strings.ToUpper(defaultApplicationName))
	configurationManager.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	configurationManager.AutomaticEnv()

	userConfigDir, userConfigErr := os.UserConfigDir()
	if userConfigErr != nil {
		logger.Error("resolve user config directory", zap.Error(userConfigErr))
		return 1
	}
	applicationConfigDir := filepath.Join(userConfigDir, defaultApplicationName)

	configurationManager.SetDefault(configKeyServeBindAddress, "")
	configurationManager.SetDefault(configKeyServeDirectory, ".")
	configurationManager.SetDefault(configKeyServeProtocol, defaultProtocolVersion)
	configurationManager.SetDefault(configKeyServePort, defaultServePort)
	configurationManager.SetDefault(configKeyServeTLSCertificatePath, "")
	configurationManager.SetDefault(configKeyServeTLSKeyPath, "")
	configurationManager.SetDefault(configKeyHTTPSCertificateDir, filepath.Join(applicationConfigDir, certificates.DefaultCertificateDirectoryName))
	configurationManager.SetDefault(configKeyHTTPSHosts, []string{"localhost", "127.0.0.1", "::1"})
	configurationManager.SetDefault(configKeyHTTPSPort, defaultHTTPSServePort)

	resources := applicationResources{
		configurationManager: configurationManager,
		logger:               logger,
		defaultConfigDirPath: applicationConfigDir,
	}

	rootCommand := newRootCommand(resources)
	baseContext := context.WithValue(ctx, contextKeyApplicationResources, resources)
	rootCommand.SetContext(baseContext)
	rootCommand.SetArgs(arguments)

	if executionErr := rootCommand.Execute(); executionErr != nil {
		logger.Error("command execution failed", zap.Error(executionErr))
		return 1
	}

	return 0
}
