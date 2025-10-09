package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/temirov/ghttp/internal/server"
	"github.com/temirov/ghttp/internal/serverdetails"
	"github.com/temirov/ghttp/pkg/logging"
)

const (
	environmentVariableDisableDirectoryListing = "GHTTPD_DISABLE_DIR_INDEX"
	logFieldSignal                             = "signal"
	logMessageReceivedSignal                   = "received signal"
)

var allowedInitialServeFileExtensions = map[string]struct{}{
	".html": {},
	".htm":  {},
	".md":   {},
}

type ServeConfiguration struct {
	BindAddress             string
	Port                    string
	DirectoryPath           string
	ProtocolVersion         string
	TLSCertificatePath      string
	TLSPrivateKeyPath       string
	DisableDirectoryListing bool
	EnableDynamicHTTPS      bool
	EnableMarkdown          bool
	BrowseDirectories       bool
	InitialFileRelativePath string
	LoggingType             string
}

func prepareServeConfiguration(cmd *cobra.Command, args []string, portConfigKey string, allowTLSFiles bool) error {
	resources, err := getApplicationResources(cmd)
	if err != nil {
		return err
	}
	configurationManager := resources.configurationManager

	bindAddress := strings.TrimSpace(configurationManager.GetString(configKeyServeBindAddress))
	directoryPath := strings.TrimSpace(configurationManager.GetString(configKeyServeDirectory))
	if directoryPath == "" {
		directoryPath = "."
	}

	initialFileRelativePath := ""
	portValue := strings.TrimSpace(configurationManager.GetString(portConfigKey))
	if len(args) == 1 {
		argumentValue := strings.TrimSpace(args[0])
		if argumentValue != "" {
			portCandidate, parseErr := strconv.Atoi(argumentValue)
			if parseErr == nil && portCandidate > 0 && portCandidate <= 65535 {
				portValue = argumentValue
			} else {
				resolvedDirectory, resolvedFile, resolveErr := resolveInitialServeFile(argumentValue)
				if resolveErr != nil {
					return resolveErr
				}
				directoryPath = resolvedDirectory
				initialFileRelativePath = resolvedFile
			}
		}
	}
	absoluteDirectory, absoluteErr := filepath.Abs(directoryPath)
	if absoluteErr != nil {
		return fmt.Errorf("resolve directory path: %w", absoluteErr)
	}
	statInfo, statErr := os.Stat(absoluteDirectory)
	if statErr != nil {
		return fmt.Errorf("stat directory: %w", statErr)
	}
	if !statInfo.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absoluteDirectory)
	}

	protocolValue := strings.ToUpper(strings.TrimSpace(configurationManager.GetString(configKeyServeProtocol)))
	if protocolValue != "HTTP/1.0" && protocolValue != "HTTP/1.1" {
		return fmt.Errorf("unsupported protocol %s", protocolValue)
	}

	if portValue == "" {
		portValue = defaultServePort
	}
	portNumber, portErr := strconv.Atoi(portValue)
	if portErr != nil || portNumber <= 0 || portNumber > 65535 {
		return fmt.Errorf("invalid port %s", portValue)
	}

	tlsCertificatePath := strings.TrimSpace(configurationManager.GetString(configKeyServeTLSCertificatePath))
	tlsKeyPath := strings.TrimSpace(configurationManager.GetString(configKeyServeTLSKeyPath))
	markdownDisabled := configurationManager.GetBool(configKeyServeNoMarkdown)
	browseDirectories := configurationManager.GetBool(configKeyServeBrowse)
	enableDynamicHTTPS := configurationManager.GetBool(configKeyServeHTTPS)
	loggingTypeValue, normalizeErr := logging.NormalizeType(configurationManager.GetString(configKeyServeLoggingType))
	if normalizeErr != nil {
		return normalizeErr
	}
	if !allowTLSFiles {
		enableDynamicHTTPS = false
	}
	if !allowTLSFiles {
		if tlsCertificatePath != "" || tlsKeyPath != "" {
			return errors.New("tls certificate flags are not supported for this command")
		}
		tlsCertificatePath = ""
		tlsKeyPath = ""
	}
	if (tlsCertificatePath == "") != (tlsKeyPath == "") {
		return errors.New("tls certificate and key must be provided together")
	}
	if enableDynamicHTTPS && (tlsCertificatePath != "" || tlsKeyPath != "") {
		return errors.New("cannot combine https flag with tls certificate flags")
	}
	if tlsCertificatePath != "" {
		if _, certErr := os.Stat(tlsCertificatePath); certErr != nil {
			return fmt.Errorf("stat tls certificate: %w", certErr)
		}
		if _, keyErr := os.Stat(tlsKeyPath); keyErr != nil {
			return fmt.Errorf("stat tls private key: %w", keyErr)
		}
	}

	disableDirectoryListing := os.Getenv(environmentVariableDisableDirectoryListing) == "1"
	if browseDirectories {
		disableDirectoryListing = false
	}
	serveConfiguration := ServeConfiguration{
		BindAddress:             bindAddress,
		Port:                    portValue,
		DirectoryPath:           absoluteDirectory,
		ProtocolVersion:         protocolValue,
		TLSCertificatePath:      tlsCertificatePath,
		TLSPrivateKeyPath:       tlsKeyPath,
		DisableDirectoryListing: disableDirectoryListing,
		EnableDynamicHTTPS:      enableDynamicHTTPS,
		EnableMarkdown:          !markdownDisabled,
		BrowseDirectories:       browseDirectories,
		InitialFileRelativePath: initialFileRelativePath,
		LoggingType:             loggingTypeValue,
	}

	if loggerErr := resources.updateLogger(loggingTypeValue); loggerErr != nil {
		return fmt.Errorf("configure logger: %w", loggerErr)
	}

	cmd.SetContext(context.WithValue(cmd.Context(), contextKeyServeConfiguration, serveConfiguration))
	return nil
}

func runServe(cmd *cobra.Command) error {
	resources, err := getApplicationResources(cmd)
	if err != nil {
		return err
	}
	serveConfigurationValue := cmd.Context().Value(contextKeyServeConfiguration)
	if serveConfigurationValue == nil {
		return errors.New("serve configuration not initialized")
	}
	serveConfiguration, ok := serveConfigurationValue.(ServeConfiguration)
	if !ok {
		return errors.New("serve configuration has unexpected type")
	}
	if serveConfiguration.EnableDynamicHTTPS {
		return serveWithDynamicHTTPS(cmd, resources, serveConfiguration)
	}

	fileServerConfiguration := server.FileServerConfiguration{
		BindAddress:             serveConfiguration.BindAddress,
		Port:                    serveConfiguration.Port,
		DirectoryPath:           serveConfiguration.DirectoryPath,
		ProtocolVersion:         serveConfiguration.ProtocolVersion,
		DisableDirectoryListing: serveConfiguration.DisableDirectoryListing,
		EnableMarkdown:          serveConfiguration.EnableMarkdown,
		BrowseDirectories:       serveConfiguration.BrowseDirectories,
		InitialFileRelativePath: serveConfiguration.InitialFileRelativePath,
		LoggingType:             serveConfiguration.LoggingType,
	}
	if serveConfiguration.TLSCertificatePath != "" {
		fileServerConfiguration.TLS = &server.TLSConfiguration{
			CertificatePath: serveConfiguration.TLSCertificatePath,
			PrivateKeyPath:  serveConfiguration.TLSPrivateKeyPath,
		}
	}

	servingAddressFormatter := serverdetails.NewServingAddressFormatter()
	fileServerInstance := server.NewFileServer(resources.loggingService, servingAddressFormatter)
	serveContext, cancel := createSignalContext(cmd.Context(), resources.loggingService)
	defer cancel()

	return fileServerInstance.Serve(serveContext, fileServerConfiguration)
}

func loadConfigurationFile(cmd *cobra.Command) error {
	resources, err := getApplicationResources(cmd)
	if err != nil {
		return err
	}
	configurationManager := resources.configurationManager
	configFilePath, flagErr := cmd.Flags().GetString(flagNameConfigFile)
	if flagErr != nil {
		return fmt.Errorf("read config flag: %w", flagErr)
	}
	if configFilePath != "" {
		configurationManager.SetConfigFile(configFilePath)
	} else {
		configurationManager.AddConfigPath(resources.defaultConfigDirPath)
		configurationManager.SetConfigName(defaultConfigFileName)
		configurationManager.SetConfigType(defaultConfigFileType)
	}
	if readErr := configurationManager.ReadInConfig(); readErr != nil {
		if _, notFound := readErr.(viper.ConfigFileNotFoundError); !notFound {
			return fmt.Errorf("read configuration: %w", readErr)
		}
	}
	return nil
}

func resolveInitialServeFile(candidatePath string) (string, string, error) {
	absolutePath, absoluteErr := filepath.Abs(candidatePath)
	if absoluteErr != nil {
		return "", "", fmt.Errorf("resolve initial file path: %w", absoluteErr)
	}
	fileInfo, statErr := os.Stat(absolutePath)
	if statErr != nil {
		return "", "", fmt.Errorf("stat initial file: %w", statErr)
	}
	if fileInfo.IsDir() {
		return "", "", fmt.Errorf("initial file is a directory: %s", absolutePath)
	}
	extension := strings.ToLower(filepath.Ext(fileInfo.Name()))
	if _, allowed := allowedInitialServeFileExtensions[extension]; !allowed {
		return "", "", fmt.Errorf("unsupported initial file extension %s", extension)
	}
	directory := filepath.Dir(absolutePath)
	return directory, fileInfo.Name(), nil
}

func getApplicationResources(cmd *cobra.Command) (*applicationResources, error) {
	resourceValue := cmd.Context().Value(contextKeyApplicationResources)
	if resourceValue == nil {
		return nil, errors.New("application resources not configured")
	}
	resources, ok := resourceValue.(*applicationResources)
	if !ok {
		return nil, errors.New("invalid application resources type")
	}
	return resources, nil
}

func serveWithDynamicHTTPS(cmd *cobra.Command, resources *applicationResources, serveConfiguration ServeConfiguration) error {
	if err := prepareHTTPSContext(cmd); err != nil {
		return err
	}
	setupErr := runHTTPSSetup(cmd)
	if setupErr != nil {
		return setupErr
	}

	hostsValue := cmd.Context().Value(contextKeyHTTPSHosts)
	if hostsValue == nil {
		return errors.New("https hosts missing")
	}
	hosts, ok := hostsValue.([]string)
	if !ok {
		return errors.New("https hosts type mismatch")
	}

	directoryValue := cmd.Context().Value(contextKeyHTTPSCertificateDir)
	if directoryValue == nil {
		return errors.New("certificate directory missing")
	}
	certificateDirectory, ok := directoryValue.(string)
	if !ok {
		return errors.New("certificate directory type mismatch")
	}

	serveErr := executeHTTPSServe(cmd, resources, serveConfiguration, hosts, certificateDirectory)
	uninstallErr := runHTTPSUninstall(cmd)
	if uninstallErr != nil {
		if serveErr != nil {
			return errors.Join(serveErr, uninstallErr)
		}
		return uninstallErr
	}
	return serveErr
}

func createSignalContext(parent context.Context, loggingService *logging.Service) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-ctx.Done():
			return
		case receivedSignal := <-signalChannel:
			if loggingService != nil {
				loggingService.Info(logMessageReceivedSignal, logging.String(logFieldSignal, receivedSignal.String()))
			}
			cancel()
		}
	}()

	return ctx, func() {
		signal.Stop(signalChannel)
		cancel()
	}
}
