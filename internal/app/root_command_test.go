package app

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/temirov/ghttp/pkg/logging"
)

func TestNewRootCommandProvidesHTTPSFlagOnce(t *testing.T) {
	configurationManager := viper.New()
	resources := &applicationResources{
		configurationManager: configurationManager,
		loggingService:       logging.NewTestService(logging.TypeConsole),
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

	httpsCommand := newHTTPSCommand(resources)
	if httpsCommand.Use != "https" {
		t.Fatalf("unexpected https command use: %s", httpsCommand.Use)
	}
}
