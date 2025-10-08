package truststore

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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

func TestInstallerFactories(t *testing.T) {
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
				CertificateCommonName: certificates.DefaultCertificateAuthorityCommonName,
			},
			certificatePath: "/tmp/certificate.pem",
			validateAfterUninstall: func(testingT *testing.T, commandRunner *recordingCommandRunner, configuration Configuration, destinationPath string) {
				testingT.Helper()
				if len(commandRunner.executed) != 2 {
					testingT.Fatalf("expected two commands, got %d", len(commandRunner.executed))
				}
				if !commandRunner.executed[0].privileged || !commandRunner.executed[1].privileged {
					testingT.Fatalf("expected privileged execution for macos commands")
				}
				if commandRunner.executed[0].executable != commandNameSecurity {
					testingT.Fatalf("expected security command, got %s", commandRunner.executed[0].executable)
				}
				if commandRunner.executed[0].arguments[0] != "add-trusted-cert" {
					testingT.Fatalf("unexpected install arguments %v", commandRunner.executed[0].arguments)
				}
				if commandRunner.executed[1].arguments[0] != "delete-certificate" {
					testingT.Fatalf("unexpected uninstall arguments %v", commandRunner.executed[1].arguments)
				}
			},
		},
		{
			name:       "windows installer runs certutil commands",
			factoryKey: "windows",
			configuration: Configuration{
				CertificateCommonName:       certificates.DefaultCertificateAuthorityCommonName,
				WindowsCertificateStoreName: "Root",
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
				if commandRunner.executed[0].arguments[0] != "-addstore" {
					testingT.Fatalf("unexpected install arguments %v", commandRunner.executed[0].arguments)
				}
				if commandRunner.executed[1].arguments[0] != "-delstore" {
					testingT.Fatalf("unexpected uninstall arguments %v", commandRunner.executed[1].arguments)
				}
			},
		},
		{
			name:       "linux installer copies certificate and updates trust store",
			factoryKey: "linux",
			configuration: Configuration{
				LinuxCertificateDestinationPath: linuxDestinationPath,
				LinuxCertificateFilePermissions: 0o644,
			},
			certificatePath: linuxSourcePath,
			destinationPath: linuxDestinationPath,
			skip: func() bool {
				return runtime.GOOS == "windows"
			},
			validateAfterInstall: func(testingT *testing.T, commandRunner *recordingCommandRunner, configuration Configuration, destinationPath string) {
				testingT.Helper()
				if len(commandRunner.executed) < 2 {
					testingT.Fatalf("expected privileged commands for linux install")
				}
				if commandRunner.executed[0].executable != commandNameInstall || !commandRunner.executed[0].privileged {
					testingT.Fatalf("expected install command with privileges, got %v", commandRunner.executed[0])
				}
			},
			validateAfterUninstall: func(testingT *testing.T, commandRunner *recordingCommandRunner, configuration Configuration, destinationPath string) {
				testingT.Helper()
				if len(commandRunner.executed) == 0 {
					testingT.Fatalf("expected commands during uninstall")
				}
				lastIndex := len(commandRunner.executed) - 1
				if !commandRunner.executed[0].privileged {
					testingT.Fatalf("expected privileged uninstall commands")
				}
				if commandRunner.executed[lastIndex].executable != commandNameUpdateCaCertificates {
					testingT.Fatalf("expected final update-ca-certificates command, got %s", commandRunner.executed[lastIndex].executable)
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
