package truststore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/temirov/ghttp/internal/certificates"
)

type executedCommand struct {
	executable string
	arguments  []string
	privileged bool
}

type recordingCommandRunner struct {
	executed []executedCommand
	errors   []error
}

func newRecordingCommandRunner(errors []error) *recordingCommandRunner {
	return &recordingCommandRunner{executed: []executedCommand{}, errors: errors}
}

func (runner *recordingCommandRunner) Run(ctx context.Context, executable string, arguments []string) error {
	runner.executed = append(runner.executed, executedCommand{executable: executable, arguments: append([]string{}, arguments...), privileged: false})
	if len(runner.errors) == 0 {
		return nil
	}
	nextError := runner.errors[0]
	runner.errors = runner.errors[1:]
	return nextError
}

func (runner *recordingCommandRunner) RunWithPrivileges(ctx context.Context, executable string, arguments []string) error {
	runner.executed = append(runner.executed, executedCommand{executable: executable, arguments: append([]string{}, arguments...), privileged: true})
	if len(runner.errors) == 0 {
		return nil
	}
	nextError := runner.errors[0]
	runner.errors = runner.errors[1:]
	return nextError
}

func TestIntegrationTrustStoreInstallerFactories(t *testing.T) {
	ctx := context.Background()
	temporaryDirectory := t.TempDir()
	linuxSourcePath := filepath.Join(temporaryDirectory, "root_ca.pem")
	writeErr := os.WriteFile(linuxSourcePath, []byte("certificate-data"), 0o600)
	if writeErr != nil {
		t.Fatalf("write linux certificate: %v", writeErr)
	}
	linuxDestinationPath := filepath.Join(temporaryDirectory, "installed_ca.crt")

	testCases := []struct {
		name                   string
		factoryKey             string
		configuration          Configuration
		certificatePath        string
		destinationPath        string
		skip                   func() bool
		validateAfterInstall   func(testingT *testing.T, commandRunner *recordingCommandRunner, configuration Configuration, destinationPath string)
		validateAfterUninstall func(testingT *testing.T, commandRunner *recordingCommandRunner, configuration Configuration, destinationPath string)
	}{
		{
			name:       "macos installer runs security commands",
			factoryKey: "darwin",
			configuration: Configuration{
				CertificateCommonName:     certificates.DefaultCertificateAuthorityCommonName,
				FirefoxProfileDirectories: []string{filepath.Join(temporaryDirectory, "no-firefox")},
			},
			certificatePath: "/tmp/certificate.pem",
			validateAfterUninstall: func(testingT *testing.T, commandRunner *recordingCommandRunner, configuration Configuration, destinationPath string) {
				testingT.Helper()
				if len(commandRunner.executed) != 2 {
					testingT.Fatalf("expected two commands, got %d", len(commandRunner.executed))
				}
				if commandRunner.executed[0].executable != commandNameSecurity {
					testingT.Fatalf("expected security command, got %s", commandRunner.executed[0].executable)
				}
				if commandRunner.executed[0].arguments[0] != "add-trusted-cert" {
					testingT.Fatalf("unexpected install arguments %v", commandRunner.executed[0].arguments)
				}
				if commandRunner.executed[0].privileged {
					testingT.Fatalf("expected install to run without privileges")
				}
				if commandRunner.executed[1].arguments[0] != "delete-certificate" {
					testingT.Fatalf("unexpected uninstall arguments %v", commandRunner.executed[1].arguments)
				}
				if commandRunner.executed[1].privileged {
					testingT.Fatalf("expected uninstall to run without privileges")
				}
			},
		},
		{
			name:       "windows installer runs certutil commands",
			factoryKey: "windows",
			configuration: Configuration{
				CertificateCommonName:       certificates.DefaultCertificateAuthorityCommonName,
				WindowsCertificateStoreName: "Root",
				FirefoxProfileDirectories:   []string{filepath.Join(temporaryDirectory, "no-firefox")},
			},
			certificatePath: "C:\\certificate.pem",
			validateAfterUninstall: func(testingT *testing.T, commandRunner *recordingCommandRunner, configuration Configuration, destinationPath string) {
				testingT.Helper()
				if len(commandRunner.executed) != 2 {
					testingT.Fatalf("expected two commands, got %d", len(commandRunner.executed))
				}
				if commandRunner.executed[0].executable != commandNameCertutil {
					testingT.Fatalf("expected certutil, got %s", commandRunner.executed[0].executable)
				}
				if len(commandRunner.executed[0].arguments) < 2 || commandRunner.executed[0].arguments[0] != "-user" || commandRunner.executed[0].arguments[1] != "-addstore" {
					testingT.Fatalf("unexpected install arguments %v", commandRunner.executed[0].arguments)
				}
				if len(commandRunner.executed[1].arguments) < 2 || commandRunner.executed[1].arguments[0] != "-user" || commandRunner.executed[1].arguments[1] != "-delstore" {
					testingT.Fatalf("unexpected uninstall arguments %v", commandRunner.executed[1].arguments)
				}
			},
		},
		{
			name:       "linux installer configures user trust store",
			factoryKey: "linux",
			configuration: Configuration{
				LinuxCertificateDestinationPath: linuxDestinationPath,
				LinuxCertificateFilePermissions: 0o644,
				FirefoxProfileDirectories:       []string{filepath.Join(temporaryDirectory, "no-firefox")},
			},
			certificatePath: linuxSourcePath,
			destinationPath: linuxDestinationPath,
			skip: func() bool {
				return runtime.GOOS == "windows"
			},
			validateAfterInstall: func(testingT *testing.T, commandRunner *recordingCommandRunner, configuration Configuration, destinationPath string) {
				testingT.Helper()
				if len(commandRunner.executed) == 0 {
					testingT.Fatalf("expected at least one command for linux install")
				}
				if commandRunner.executed[0].executable != commandNameTrust {
					testingT.Fatalf("expected trust command, got %s", commandRunner.executed[0].executable)
				}
				if commandRunner.executed[0].arguments[0] != "anchor" {
					testingT.Fatalf("unexpected trust arguments %v", commandRunner.executed[0].arguments)
				}
				if commandRunner.executed[0].privileged {
					testingT.Fatalf("expected trust command to run without privileges")
				}
				content, readErr := os.ReadFile(destinationPath)
				if readErr != nil {
					testingT.Fatalf("read destination certificate: %v", readErr)
				}
				if string(content) != "certificate-data" {
					testingT.Fatalf("unexpected certificate content %q", string(content))
				}
			},
			validateAfterUninstall: func(testingT *testing.T, commandRunner *recordingCommandRunner, configuration Configuration, destinationPath string) {
				testingT.Helper()
				if len(commandRunner.executed) == 0 {
					testingT.Fatalf("expected commands during uninstall")
				}
				lastIndex := len(commandRunner.executed) - 1
				if commandRunner.executed[lastIndex].executable != commandNameTrust {
					testingT.Fatalf("expected trust command for uninstall, got %s", commandRunner.executed[lastIndex].executable)
				}
				if commandRunner.executed[lastIndex].arguments[0] != "anchor" || len(commandRunner.executed[lastIndex].arguments) < 2 || commandRunner.executed[lastIndex].arguments[1] != "--remove" {
					testingT.Fatalf("unexpected uninstall arguments %v", commandRunner.executed[lastIndex].arguments)
				}
				if commandRunner.executed[lastIndex].privileged {
					testingT.Fatalf("expected uninstall to run without privileges")
				}
				if _, err := os.Stat(destinationPath); !errors.Is(err, os.ErrNotExist) {
					testingT.Fatalf("expected destination certificate to be removed, got err=%v", err)
				}
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(testingT *testing.T) {
			if testCase.skip != nil && testCase.skip() {
				testingT.Skip("skipping on current platform")
			}
			factory := supportedFactories[testCase.factoryKey]
			if factory == nil {
				testingT.Fatalf("factory for %s not registered", testCase.factoryKey)
			}
			commandRunner := newRecordingCommandRunner([]error{nil, nil, nil})
			fileSystem := certificates.NewOperatingSystemFileSystem()
			installer, err := factory(commandRunner, fileSystem, testCase.configuration)
			if err != nil {
				testingT.Fatalf("create installer: %v", err)
			}
			err = installer.Install(ctx, testCase.certificatePath)
			if err != nil {
				testingT.Fatalf("install certificate: %v", err)
			}
			if testCase.validateAfterInstall != nil {
				testCase.validateAfterInstall(testingT, commandRunner, testCase.configuration, testCase.destinationPath)
			}
			err = installer.Uninstall(ctx)
			if err != nil {
				testingT.Fatalf("uninstall certificate: %v", err)
			}
			if testCase.validateAfterUninstall != nil {
				testCase.validateAfterUninstall(testingT, commandRunner, testCase.configuration, testCase.destinationPath)
			}
		})
	}
}

func TestFirefoxIntegrationAddsUserPreferenceWhenCertutilMissing(t *testing.T) {
	temporaryDirectory := t.TempDir()
	profileDirectory := filepath.Join(temporaryDirectory, "profile.default")
	mustMkdir(t, profileDirectory)
	mustWriteFile(t, filepath.Join(profileDirectory, "cert9.db"), []byte(""))
	certificatePath := filepath.Join(temporaryDirectory, "ca.pem")
	mustWriteFile(t, certificatePath, []byte("certificate"))

	commandRunner := newRecordingCommandRunner([]error{fmt.Errorf("certutil missing")})
	configuration := Configuration{
		CertificateCommonName:           certificates.DefaultCertificateAuthorityCommonName,
		FirefoxProfileDirectories:       []string{temporaryDirectory},
		LinuxCertificateDestinationPath: filepath.Join(temporaryDirectory, "installed_ca.crt"),
	}
	installErr := integrateFirefoxCertificates(context.Background(), commandRunner, certificates.NewOperatingSystemFileSystem(), configuration, certificatePath)
	if installErr != nil {
		t.Fatalf("integrate firefox certificates: %v", installErr)
	}
	userPreferencesPath := filepath.Join(profileDirectory, firefoxUserPreferenceFile)
	preferencesContent, readErr := os.ReadFile(userPreferencesPath)
	if readErr != nil {
		t.Fatalf("read user.js: %v", readErr)
	}
	if !strings.Contains(string(preferencesContent), firefoxUserPreferenceLine) {
		t.Fatalf("expected enterprise roots preference in user.js, got %s", string(preferencesContent))
	}
}

func mustMkdir(t *testing.T, path string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, content []byte) {
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
