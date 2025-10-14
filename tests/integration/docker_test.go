package integration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	dockerImageName      = "ghttp-test"
	dockerBuildTimeout   = 300 * time.Second
	dockerStartupTimeout = 30 * time.Second
	httpRequestTimeout   = 10 * time.Second
	containerPort        = "8080"
	hostPort             = "18080"
)

// TestDockerBuild verifies that the Docker image builds successfully
func TestDockerBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker integration test in short mode")
	}

	requireDockerPrerequisites(t, nil)

	repositoryRoot := getRepositoryRoot(t)

	testCases := []struct {
		name          string
		buildArgs     []string
		expectedError bool
	}{
		{
			name: "standard build",
			buildArgs: []string{
				"build",
				"-t", dockerImageName,
				"-f", "Dockerfile",
				".",
			},
			expectedError: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			buildError := executeDockerCommand(t, dockerBuildTimeout, repositoryRoot, testCase.buildArgs)

			if testCase.expectedError && buildError == nil {
				t.Error("Expected build to fail, but it succeeded")
			}

			if !testCase.expectedError && buildError != nil {
				t.Errorf("Docker build failed: %v", buildError)
			}
		})
	}

	t.Cleanup(func() {
		cleanupCommand := exec.Command("docker", "rmi", dockerImageName)
		_ = cleanupCommand.Run()
	})
}

// TestDockerRun verifies that the Docker container runs and serves files correctly
func TestDockerRun(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker integration test in short mode")
	}

	requireDockerPrerequisites(t, nil)

	repositoryRoot := getRepositoryRoot(t)

	buildArguments := []string{"build", "-t", dockerImageName, "-f", "Dockerfile", "."}
	if buildError := executeDockerCommand(t, dockerBuildTimeout, repositoryRoot, buildArguments); buildError != nil {
		t.Fatalf("Failed to build Docker image: %v", buildError)
	}

	t.Cleanup(func() {
		cleanupCommand := exec.Command("docker", "rmi", dockerImageName)
		_ = cleanupCommand.Run()
	})

	temporaryDirectory := t.TempDir()
	testFileContent := "Hello from gHTTP Docker test"
	testFilePath := filepath.Join(temporaryDirectory, "test.txt")

	if writeError := os.WriteFile(testFilePath, []byte(testFileContent), 0644); writeError != nil {
		t.Fatalf("Failed to create test file: %v", writeError)
	}

	containerName := fmt.Sprintf("ghttp-test-%d", time.Now().Unix())

	runArguments := []string{
		"run",
		"--rm",
		"-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%s:%s", hostPort, containerPort),
		"-v", fmt.Sprintf("%s:/data", temporaryDirectory),
		dockerImageName,
		"--directory", "/data",
		containerPort,
	}

	if runError := executeDockerCommand(t, dockerStartupTimeout, "", runArguments); runError != nil {
		t.Fatalf("Failed to run Docker container: %v", runError)
	}

	t.Cleanup(func() {
		stopCommand := exec.Command("docker", "stop", containerName)
		_ = stopCommand.Run()
	})

	time.Sleep(3 * time.Second)

	requestContext, requestCancel := context.WithTimeout(context.Background(), httpRequestTimeout)
	defer requestCancel()

	testURL := fmt.Sprintf("http://localhost:%s/test.txt", hostPort)
	httpRequest, requestError := http.NewRequestWithContext(requestContext, http.MethodGet, testURL, nil)
	if requestError != nil {
		t.Fatalf("Failed to create HTTP request: %v", requestError)
	}

	httpClient := &http.Client{Timeout: httpRequestTimeout}
	httpResponse, responseError := httpClient.Do(httpRequest)
	if responseError != nil {
		logsCommand := exec.Command("docker", "logs", containerName)
		logsOutput, _ := logsCommand.CombinedOutput()
		t.Logf("Container logs:\n%s", string(logsOutput))
		t.Fatalf("Failed to fetch file from container: %v", responseError)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, httpResponse.StatusCode)
	}

	responseBody, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		t.Fatalf("Failed to read response body: %v", readError)
	}

	if string(responseBody) != testFileContent {
		t.Errorf("Expected content %q, got %q", testFileContent, string(responseBody))
	}
}

// TestDockerMultiPlatformBuild verifies that the Dockerfile supports multi-platform builds
func TestDockerMultiPlatformBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker integration test in short mode")
	}

	requireDockerPrerequisites(t, nil)

	repositoryRoot := getRepositoryRoot(t)

	buildxArguments := []string{"buildx", "version"}
	if buildxError := executeDockerCommand(t, 10*time.Second, "", buildxArguments); buildxError != nil {
		t.Skip("Docker buildx not available, skipping multi-platform test")
	}

	platforms := []string{
		"linux/amd64",
		"linux/arm64",
	}

	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			commandArguments := []string{
				"buildx", "build",
				"--platform", platform,
				"-t", fmt.Sprintf("%s-%s", dockerImageName, strings.ReplaceAll(platform, "/", "-")),
				"-f", "Dockerfile",
				".",
			}

			if buildError := executeDockerCommand(t, dockerBuildTimeout, repositoryRoot, commandArguments); buildError != nil {
				t.Errorf("Failed to build for platform %s: %v", platform, buildError)
			}
		})
	}

	windowsDockerfilePath := filepath.Join(repositoryRoot, "docker", "Dockerfile.windows")
	if _, statErr := os.Stat(windowsDockerfilePath); statErr != nil {
		if !os.IsNotExist(statErr) {
			t.Fatalf("Failed to stat Windows Dockerfile: %v", statErr)
		}
		return
	}

	if runtime.GOOS == "windows" || isTruthy(os.Getenv("GHTTP_ENABLE_WINDOWS_DOCKER_TESTS")) {
		requireDockerPrerequisites(t, []string{
			"golang:1.25-windowsservercore-ltsc2022",
			"mcr.microsoft.com/windows/nanoserver:ltsc2022",
		})
		windowsTag := fmt.Sprintf("%s-windows", dockerImageName)
		buildArguments := []string{
			"build",
			"-t", windowsTag,
			"-f", filepath.Join("docker", "Dockerfile.windows"),
			".",
		}
		if buildError := executeDockerCommand(t, dockerBuildTimeout, repositoryRoot, buildArguments); buildError != nil {
			t.Errorf("Failed to build Windows Docker image: %v", buildError)
		}
	} else {
		t.Log("Skipping Windows Docker build; enable by setting GHTTP_ENABLE_WINDOWS_DOCKER_TESTS=1 or running on Windows.")
	}
}

func executeDockerCommand(testInstance *testing.T, timeoutDuration time.Duration, workingDirectory string, arguments []string) error {
	testInstance.Helper()

	if len(arguments) == 0 {
		return errors.New("docker command requires at least one argument")
	}

	commandContext, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	command := exec.CommandContext(commandContext, dockerExecutableName, arguments...)
	if strings.TrimSpace(workingDirectory) != "" {
		command.Dir = workingDirectory
	}
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

// getRepositoryRoot walks up the directory tree to find the repository root
func getRepositoryRoot(testInstance *testing.T) string {
	testInstance.Helper()

	currentDirectory, directoryError := os.Getwd()
	if directoryError != nil {
		testInstance.Fatalf("Failed to get working directory: %v", directoryError)
	}

	for {
		dockerfilePath := filepath.Join(currentDirectory, "Dockerfile")
		if _, statError := os.Stat(dockerfilePath); statError == nil {
			return currentDirectory
		}

		parentDirectory := filepath.Dir(currentDirectory)
		if parentDirectory == currentDirectory {
			testInstance.Fatal("Could not find repository root (no Dockerfile found)")
		}
		currentDirectory = parentDirectory
	}
}
