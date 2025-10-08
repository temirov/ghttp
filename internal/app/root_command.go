package app

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func newRootCommand(resources applicationResources) *cobra.Command {
	rootCommand := &cobra.Command{
		Use:           fmt.Sprintf("%s [port]", defaultApplicationName),
		Short:         "Serve local directories over HTTP or HTTPS",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return loadConfigurationFile(cmd)
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareServeConfiguration(cmd, args, configKeyServePort, true)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(cmd)
		},
	}

	configureServeFlags(rootCommand.Flags(), resources.configurationManager, true)
	rootCommand.Flags().String(flagNameTLSCertificatePath, resources.configurationManager.GetString(configKeyServeTLSCertificatePath), "Path to TLS certificate (PEM)")
	rootCommand.Flags().String(flagNameTLSKeyPath, resources.configurationManager.GetString(configKeyServeTLSKeyPath), "Path to TLS private key (PEM)")
	_ = resources.configurationManager.BindPFlag(configKeyServeTLSCertificatePath, rootCommand.Flags().Lookup(flagNameTLSCertificatePath))
	_ = resources.configurationManager.BindPFlag(configKeyServeTLSKeyPath, rootCommand.Flags().Lookup(flagNameTLSKeyPath))

	rootCommand.PersistentFlags().String(flagNameConfigFile, "", "Path to configuration file")

	rootCommand.AddCommand(newHTTPSCommand(resources))

	return rootCommand
}

func configureServeFlags(flagSet *pflag.FlagSet, configurationManager *viper.Viper, includeHTTPSOptions bool) {
	flagSet.String(flagNameBindAddress, configurationManager.GetString(configKeyServeBindAddress), "Specify bind address")
	flagSet.String(flagNameDirectory, configurationManager.GetString(configKeyServeDirectory), "Serve files from this directory")
	flagSet.String(flagNameProtocol, configurationManager.GetString(configKeyServeProtocol), "HTTP protocol version (HTTP/1.0 or HTTP/1.1)")
	flagSet.Bool(flagNameNoMarkdown, configurationManager.GetBool(configKeyServeNoMarkdown), "Disable Markdown rendering")
	flagSet.String(flagNameLoggingType, configurationManager.GetString(configKeyServeLoggingType), "Logging type (CONSOLE or JSON)")
	_ = configurationManager.BindPFlag(configKeyServeBindAddress, flagSet.Lookup(flagNameBindAddress))
	_ = configurationManager.BindPFlag(configKeyServeDirectory, flagSet.Lookup(flagNameDirectory))
	_ = configurationManager.BindPFlag(configKeyServeProtocol, flagSet.Lookup(flagNameProtocol))
	_ = configurationManager.BindPFlag(configKeyServeNoMarkdown, flagSet.Lookup(flagNameNoMarkdown))
	_ = configurationManager.BindPFlag(configKeyServeLoggingType, flagSet.Lookup(flagNameLoggingType))
	if includeHTTPSOptions {
		flagSet.Bool(flagNameHTTPS, configurationManager.GetBool(configKeyServeHTTPS), "Serve over HTTPS using a self-signed certificate")
		flagSet.StringSlice(flagNameHTTPSHosts, configurationManager.GetStringSlice(configKeyHTTPSHosts), "Hostnames or IP addresses for automatic HTTPS certificates")
		_ = configurationManager.BindPFlag(configKeyServeHTTPS, flagSet.Lookup(flagNameHTTPS))
		_ = configurationManager.BindPFlag(configKeyHTTPSHosts, flagSet.Lookup(flagNameHTTPSHosts))
	}
}
