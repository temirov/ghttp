package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/temirov/ghttp/internal/certificates"
	"github.com/temirov/ghttp/internal/certificates/truststore"
	"github.com/temirov/ghttp/internal/server"
	"github.com/temirov/ghttp/internal/serverdetails"
)

const (
	certificateAuthorityKeyBits          = 4096
	leafCertificateKeyBits               = 2048
	certificateAuthorityValidityDuration = 5 * 365 * 24 * time.Hour
	certificateAuthorityRenewalWindow    = 30 * 24 * time.Hour
	leafCertificateValidityDuration      = 30 * 24 * time.Hour
	leafCertificateRenewalWindow         = 72 * time.Hour
	linuxTrustedCertificatePath          = "/usr/local/share/ca-certificates/ghttp-development-ca.crt"
)

func newHTTPSCommand(resources applicationResources) *cobra.Command {
	httpsCommand := &cobra.Command{
		Use:   "https",
		Short: "Manage self-signed HTTPS certificates",
	}

	certificateDirDefault := resources.configurationManager.GetString(configKeyHTTPSCertificateDir)
	httpsCommand.PersistentFlags().String(flagNameCertificateDir, certificateDirDefault, "Directory for generated certificates")
	_ = resources.configurationManager.BindPFlag(configKeyHTTPSCertificateDir, httpsCommand.PersistentFlags().Lookup(flagNameCertificateDir))

	httpsCommand.AddCommand(newHTTPSSetupCommand(resources))
	httpsCommand.AddCommand(newHTTPSServeCommand(resources))
	httpsCommand.AddCommand(newHTTPSUninstallCommand(resources))

	return httpsCommand
}

func newHTTPSSetupCommand(resources applicationResources) *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Generate and install the development certificate authority",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHTTPSSetup(cmd)
		},
	}
}

func newHTTPSServeCommand(resources applicationResources) *cobra.Command {
	httpsServeCommand := &cobra.Command{
		Use:           "serve [port]",
		Short:         "Serve HTTPS using the generated certificates",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := prepareServeConfiguration(cmd, args, configKeyHTTPSPort, false); err != nil {
				return err
			}
			return prepareHTTPSContext(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHTTPSServe(cmd)
		},
	}

	configureServeFlags(httpsServeCommand.Flags(), resources.configurationManager)
	hostsDefault := resources.configurationManager.GetStringSlice(configKeyHTTPSHosts)
	httpsServeCommand.Flags().StringSlice(flagNameHTTPSHosts, hostsDefault, "Hostnames or IPs to include in the certificate SAN")
	_ = resources.configurationManager.BindPFlag(configKeyHTTPSHosts, httpsServeCommand.Flags().Lookup(flagNameHTTPSHosts))

	return httpsServeCommand
}

func newHTTPSUninstallCommand(resources applicationResources) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the development certificate authority from the trust store",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHTTPSUninstall(cmd)
		},
	}
}

func runHTTPSSetup(cmd *cobra.Command) error {
	resources, err := getApplicationResources(cmd)
	if err != nil {
		return err
	}
	certificateDirectory, err := resolveCertificateDirectory(resources.configurationManager)
	if err != nil {
		return err
	}

	fileSystem := certificates.NewOperatingSystemFileSystem()
	certificateConfiguration := buildCertificateAuthorityConfiguration(certificateDirectory)
	manager := certificates.NewCertificateAuthorityManager(fileSystem, certificates.NewSystemClock(), rand.Reader, certificateConfiguration)
	_, ensureErr := manager.EnsureCertificateAuthority(cmd.Context())
	if ensureErr != nil {
		return fmt.Errorf("ensure certificate authority: %w", ensureErr)
	}

	installer, installerErr := buildTrustStoreInstaller(fileSystem)
	if installerErr != nil {
		return installerErr
	}
	installErr := installer.Install(cmd.Context(), filepath.Join(certificateDirectory, certificates.DefaultRootCertificateFileName))
	if installErr != nil {
		return fmt.Errorf("install certificate authority: %w", installErr)
	}

	resources.logger.Info("certificate authority installed", zapCertificateDirectory(certificateDirectory))
	return nil
}

func runHTTPSServe(cmd *cobra.Command) error {
	resources, err := getApplicationResources(cmd)
	if err != nil {
		return err
	}
	serveConfigurationValue := cmd.Context().Value(contextKeyServeConfiguration)
	if serveConfigurationValue == nil {
		return errors.New("serve configuration missing")
	}
	serveConfiguration, ok := serveConfigurationValue.(ServeConfiguration)
	if !ok {
		return errors.New("serve configuration type mismatch")
	}

	hostValue := cmd.Context().Value(contextKeyHTTPSHosts)
	if hostValue == nil {
		return errors.New("https hosts missing")
	}
	hosts, ok := hostValue.([]string)
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

	fileSystem := certificates.NewOperatingSystemFileSystem()
	certificateAuthorityConfiguration := buildCertificateAuthorityConfiguration(certificateDirectory)
	certificateAuthorityManager := certificates.NewCertificateAuthorityManager(fileSystem, certificates.NewSystemClock(), rand.Reader, certificateAuthorityConfiguration)
	certificateAuthorityMaterial, ensureErr := certificateAuthorityManager.EnsureCertificateAuthority(cmd.Context())
	if ensureErr != nil {
		return fmt.Errorf("ensure certificate authority: %w", ensureErr)
	}

	issuerConfiguration := certificates.ServerCertificateConfiguration{
		CertificateValidityDuration:      leafCertificateValidityDuration,
		CertificateRenewalWindowDuration: leafCertificateRenewalWindow,
		LeafPrivateKeyBitSize:            leafCertificateKeyBits,
		CertificateFilePermissions:       0o600,
		PrivateKeyFilePermissions:        0o600,
	}
	issuer := certificates.NewServerCertificateIssuer(fileSystem, certificates.NewSystemClock(), rand.Reader, issuerConfiguration)
	leafCertificatePath := filepath.Join(certificateDirectory, certificates.DefaultLeafCertificateFileName)
	leafKeyPath := filepath.Join(certificateDirectory, certificates.DefaultLeafPrivateKeyFileName)
	serverCertificateRequest := certificates.ServerCertificateRequest{
		Hosts:                 hosts,
		CertificateOutputPath: leafCertificatePath,
		PrivateKeyOutputPath:  leafKeyPath,
	}
	leafMaterial, issueErr := issuer.IssueServerCertificate(cmd.Context(), certificateAuthorityMaterial, serverCertificateRequest)
	if issueErr != nil {
		return fmt.Errorf("issue server certificate: %w", issueErr)
	}

	tlsCertificate, parseErr := tls.X509KeyPair(leafMaterial.CertificateBytes, leafMaterial.PrivateKeyBytes)
	if parseErr != nil {
		return fmt.Errorf("parse server certificate: %w", parseErr)
	}

	fileServerConfiguration := server.FileServerConfiguration{
		BindAddress:             serveConfiguration.BindAddress,
		Port:                    serveConfiguration.Port,
		DirectoryPath:           serveConfiguration.DirectoryPath,
		ProtocolVersion:         serveConfiguration.ProtocolVersion,
		DisableDirectoryListing: serveConfiguration.DisableDirectoryListing,
		TLS: &server.TLSConfiguration{
			LoadedCertificate: &tlsCertificate,
		},
	}

	resources.logger.Info("serving https", zapCertificateDirectory(certificateDirectory), zap.Strings("hosts", hosts))
	servingAddressFormatter := serverdetails.NewServingAddressFormatter()
	fileServerInstance := server.NewFileServer(resources.logger, servingAddressFormatter)
	serveContext, cancel := createSignalContext(cmd.Context(), resources.logger)
	defer cancel()

	return fileServerInstance.Serve(serveContext, fileServerConfiguration)
}

func runHTTPSUninstall(cmd *cobra.Command) error {
	resources, err := getApplicationResources(cmd)
	if err != nil {
		return err
	}
	certificateDirectory, err := resolveCertificateDirectory(resources.configurationManager)
	if err != nil {
		return err
	}

	fileSystem := certificates.NewOperatingSystemFileSystem()
	installer, installerErr := buildTrustStoreInstaller(fileSystem)
	if installerErr != nil {
		return installerErr
	}
	uninstallErr := installer.Uninstall(cmd.Context())
	if uninstallErr != nil {
		return fmt.Errorf("uninstall certificate authority: %w", uninstallErr)
	}
	removalErrors := []error{}
	removalTargets := []string{
		filepath.Join(certificateDirectory, certificates.DefaultRootCertificateFileName),
		filepath.Join(certificateDirectory, certificates.DefaultRootPrivateKeyFileName),
		filepath.Join(certificateDirectory, certificates.DefaultLeafCertificateFileName),
		filepath.Join(certificateDirectory, certificates.DefaultLeafPrivateKeyFileName),
	}
	for _, target := range removalTargets {
		if err := fileSystem.Remove(target); err != nil {
			removalErrors = append(removalErrors, err)
		}
	}
	if len(removalErrors) > 0 {
		return errors.Join(removalErrors...)
	}
	resources.logger.Info("certificate authority uninstalled", zapCertificateDirectory(certificateDirectory))
	return nil
}

func prepareHTTPSContext(cmd *cobra.Command) error {
	resources, err := getApplicationResources(cmd)
	if err != nil {
		return err
	}
	hosts := sanitizeHosts(resources.configurationManager.GetStringSlice(configKeyHTTPSHosts))
	if len(hosts) == 0 {
		return errors.New("at least one host must be specified")
	}
	certificateDirectory, err := resolveCertificateDirectory(resources.configurationManager)
	if err != nil {
		return err
	}
	updatedContext := context.WithValue(cmd.Context(), contextKeyHTTPSHosts, hosts)
	updatedContext = context.WithValue(updatedContext, contextKeyHTTPSCertificateDir, certificateDirectory)
	cmd.SetContext(updatedContext)
	return nil
}

func resolveCertificateDirectory(configurationManager *viper.Viper) (string, error) {
	directoryValue := strings.TrimSpace(configurationManager.GetString(configKeyHTTPSCertificateDir))
	if directoryValue == "" {
		return "", errors.New("certificate directory is not configured")
	}
	absoluteDirectory, err := filepath.Abs(directoryValue)
	if err != nil {
		return "", fmt.Errorf("resolve certificate directory: %w", err)
	}
	return absoluteDirectory, nil
}

func buildCertificateAuthorityConfiguration(certificateDirectory string) certificates.CertificateAuthorityConfiguration {
	return certificates.CertificateAuthorityConfiguration{
		DirectoryPath:                    certificateDirectory,
		CertificateFileName:              certificates.DefaultRootCertificateFileName,
		PrivateKeyFileName:               certificates.DefaultRootPrivateKeyFileName,
		DirectoryPermissions:             0o700,
		CertificateFilePermissions:       0o600,
		PrivateKeyFilePermissions:        0o600,
		RSAKeyBitSize:                    certificateAuthorityKeyBits,
		CertificateValidityDuration:      certificateAuthorityValidityDuration,
		CertificateRenewalWindowDuration: certificateAuthorityRenewalWindow,
		SubjectCommonName:                certificates.DefaultCertificateAuthorityCommonName,
		SubjectOrganizationalUnit:        certificates.DefaultCertificateAuthorityOrganizationalUnit,
		SubjectOrganization:              certificates.DefaultCertificateAuthorityOrganization,
	}
}

func buildTrustStoreInstaller(fileSystem certificates.FileSystem) (truststore.Installer, error) {
	commandRunner := certificates.NewExecutableRunner()
	configuration := truststore.Configuration{
		CertificateCommonName:           certificates.DefaultCertificateAuthorityCommonName,
		LinuxCertificateDestinationPath: linuxTrustedCertificatePath,
		LinuxCertificateFilePermissions: 0o644,
		WindowsCertificateStoreName:     "Root",
	}
	return truststore.NewInstaller(commandRunner, fileSystem, configuration)
}

func sanitizeHosts(hosts []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(hosts))
	for _, host := range hosts {
		normalizedHost := strings.TrimSpace(host)
		if normalizedHost == "" {
			continue
		}
		if _, exists := seen[normalizedHost]; exists {
			continue
		}
		seen[normalizedHost] = struct{}{}
		result = append(result, normalizedHost)
	}
	return result
}

func zapCertificateDirectory(path string) zap.Field {
	return zap.String("certificate_directory", path)
}
