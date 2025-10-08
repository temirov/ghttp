package app

import (
	"testing"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func TestNewRootCommandProvidesHTTPSFlagOnce(t *testing.T) {
	configurationManager := viper.New()
	resources := &applicationResources{
		configurationManager: configurationManager,
		logger:               zap.NewNop(),
		defaultConfigDirPath: t.TempDir(),
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("newRootCommand panicked: %v", recovered)
		}
	}()

	rootCommand := newRootCommand(resources)
	if rootCommand.Flags().Lookup(flagNameHTTPSHosts) == nil {
		t.Fatalf("expected host flag to be registered")
	}

	// Constructing the HTTPS command should also avoid flag redefinition.
	httpsCommand := newHTTPSCommand(resources)
	if httpsCommand.Use != "https" {
		t.Fatalf("unexpected https command use: %s", httpsCommand.Use)
	}
}
