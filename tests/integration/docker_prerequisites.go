package integration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	dockerExecutableName              = "docker"
	dockerPrerequisiteTimeout         = 15 * time.Second
	dockerSkipEnvironmentVariableName = "GHTTP_SKIP_DOCKER_TESTS"
	dockerForceEnvironmentVariable    = "GHTTP_FORCE_DOCKER_TESTS"
)

var (
	defaultDockerRequiredImages = []string{
		"golang:1.25",
		"gcr.io/distroless/base-debian12",
	}
	dockerVersionCommandArguments = []string{
		"version",
		"--format",
		"{{.Server.Version}}",
	}
)

type dockerPrerequisiteChecker struct {
	lookupExecutable func(string) (string, error)
	runCommand       func(context.Context, string, ...string) error
	readEnvironment  func(string) string
}

func newDockerPrerequisiteChecker() dockerPrerequisiteChecker {
	return dockerPrerequisiteChecker{
		lookupExecutable: exec.LookPath,
		runCommand:       runSystemCommand,
		readEnvironment:  os.Getenv,
	}
}

func runSystemCommand(ctx context.Context, executableName string, arguments ...string) error {
	command := exec.CommandContext(ctx, executableName, arguments...)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	return command.Run()
}

func requireDockerPrerequisites(testInstance testingT, additionalImages []string) {
	testInstance.Helper()
	checker := newDockerPrerequisiteChecker()
	contextWithTimeout, cancel := context.WithTimeout(context.Background(), dockerPrerequisiteTimeout)
	defer cancel()
	requiredImages := append([]string{}, defaultDockerRequiredImages...)
	requiredImages = append(requiredImages, additionalImages...)
	skipReason, err := checker.evaluate(contextWithTimeout, requiredImages)
	if err != nil {
		testInstance.Fatalf("docker prerequisite evaluation failed: %v", err)
	}
	if skipReason != "" {
		testInstance.Skip(skipReason)
	}
}

type testingT interface {
	Helper()
	Fatalf(string, ...interface{})
	Skip(...interface{})
}

func (checker dockerPrerequisiteChecker) evaluate(ctx context.Context, requiredImages []string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if isTruthy(checker.readEnvironment(dockerForceEnvironmentVariable)) {
		return "", nil
	}
	if isTruthy(checker.readEnvironment(dockerSkipEnvironmentVariableName)) {
		return fmt.Sprintf("Docker integration tests disabled via %s.", dockerSkipEnvironmentVariableName), nil
	}
	if checker.lookupExecutable == nil || checker.runCommand == nil || checker.readEnvironment == nil {
		return "", errors.New("docker prerequisite checker not configured")
	}
	if _, lookupErr := checker.lookupExecutable(dockerExecutableName); lookupErr != nil {
		return fmt.Sprintf("Docker integration tests require the %s executable: %v", dockerExecutableName, lookupErr), nil
	}
	versionErr := checker.runCommand(ctx, dockerExecutableName, dockerVersionCommandArguments...)
	if versionErr != nil {
		return fmt.Sprintf("Docker daemon is unavailable: %v", versionErr), nil
	}
	for _, imageName := range requiredImages {
		inspectErr := checker.runCommand(ctx, dockerExecutableName, "image", "inspect", imageName)
		if inspectErr != nil {
			return fmt.Sprintf("Docker image %s is not available locally: %v", imageName, inspectErr), nil
		}
	}
	return "", nil
}

func isTruthy(rawValue string) bool {
	normalizedValue := strings.ToLower(strings.TrimSpace(rawValue))
	switch normalizedValue {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
