package app

import (
	"context"
	"testing"

	"github.com/spf13/pflag"
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

	httpsResources := &applicationResources{
		configurationManager: viper.New(),
		loggingService:       logging.NewTestService(logging.TypeConsole),
		defaultConfigDirPath: t.TempDir(),
	}
	serveFlags := pflag.NewFlagSet("serve", pflag.ContinueOnError)
	configureServeFlags(serveFlags, httpsResources.configurationManager)
	httpsOptionFlags := pflag.NewFlagSet("serve-https-options", pflag.ContinueOnError)
	configureServeHTTPSOptions(httpsOptionFlags, httpsResources.configurationManager)

	httpsCommand := newHTTPSCommand(httpsResources, serveFlags, httpsOptionFlags)
	if httpsCommand.Use != "https" {
		t.Fatalf("unexpected https command use: %s", httpsCommand.Use)
	}
}

func TestRootCommandBindsBrowseFlag(t *testing.T) {
	configurationManager := viper.New()
	resources := &applicationResources{
		configurationManager: configurationManager,
		loggingService:       logging.NewTestService(logging.TypeConsole),
		defaultConfigDirPath: t.TempDir(),
	}

	rootCommand := newRootCommand(resources)
	rootCommand.SetArgs([]string{"--browse"})
	rootCommand.SetContext(context.WithValue(context.Background(), contextKeyApplicationResources, resources))

	parseErr := rootCommand.ParseFlags([]string{"--browse"})
	if parseErr != nil {
		t.Fatalf("parse flags: %v", parseErr)
	}

	if !configurationManager.GetBool(configKeyServeBrowse) {
		t.Fatalf("expected browse flag to bind configuration")
	}
}
