package app

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
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

	resources := applicationResources{
		configurationManager: configurationManager,
		logger:               zap.NewNop(),
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
