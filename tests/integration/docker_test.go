package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

	skipIfDockerUnavailable(t)

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
			contextWithTimeout, cancel := context.WithTimeout(context.Background(), dockerBuildTimeout)
			defer cancel()

			buildCommand := exec.CommandContext(contextWithTimeout, "docker", testCase.buildArgs...)
			buildCommand.Dir = repositoryRoot
			buildCommand.Stdout = os.Stdout
			buildCommand.Stderr = os.Stderr

			buildError := buildCommand.Run()

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

	skipIfDockerUnavailable(t)

	repositoryRoot := getRepositoryRoot(t)

	buildContext, buildCancel := context.WithTimeout(context.Background(), dockerBuildTimeout)
	defer buildCancel()

	buildCommand := exec.CommandContext(buildContext, "docker", "build", "-t", dockerImageName, "-f", "Dockerfile", ".")
	buildCommand.Dir = repositoryRoot
	buildCommand.Stdout = os.Stdout
	buildCommand.Stderr = os.Stderr

	if buildError := buildCommand.Run(); buildError != nil {
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

	runContext, runCancel := context.WithTimeout(context.Background(), dockerStartupTimeout)
	defer runCancel()

	runCommand := exec.CommandContext(
		runContext,
		"docker", "run",
		"--rm",
		"-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%s:%s", hostPort, containerPort),
		"-v", fmt.Sprintf("%s:/data", temporaryDirectory),
		dockerImageName,
		"--directory", "/data",
		containerPort,
	)
	runCommand.Stdout = os.Stdout
	runCommand.Stderr = os.Stderr

	if runError := runCommand.Run(); runError != nil {
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

	skipIfDockerUnavailable(t)

	repositoryRoot := getRepositoryRoot(t)

	buildxContext, buildxCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer buildxCancel()

	buildxCheckCommand := exec.CommandContext(buildxContext, "docker", "buildx", "version")
	if buildxError := buildxCheckCommand.Run(); buildxError != nil {
		t.Skip("Docker buildx not available, skipping multi-platform test")
	}

	platforms := []string{
		"linux/amd64",
		"linux/arm64",
	}

	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			contextWithTimeout, cancel := context.WithTimeout(context.Background(), dockerBuildTimeout)
			defer cancel()

			buildCommand := exec.CommandContext(
				contextWithTimeout,
				"docker", "buildx", "build",
				"--platform", platform,
				"-t", fmt.Sprintf("%s-%s", dockerImageName, strings.ReplaceAll(platform, "/", "-")),
				"-f", "Dockerfile",
				".",
			)
			buildCommand.Dir = repositoryRoot
			buildCommand.Stdout = os.Stdout
			buildCommand.Stderr = os.Stderr

			if buildError := buildCommand.Run(); buildError != nil {
				t.Errorf("Failed to build for platform %s: %v", platform, buildError)
			}
		})
	}
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

func skipIfDockerUnavailable(testInstance *testing.T) {
	testInstance.Helper()

	if _, lookupError := exec.LookPath("docker"); lookupError != nil {
		testInstance.Skipf("Docker binary not available: %v", lookupError)
	}

	availabilityContext, availabilityCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer availabilityCancel()

	availabilityCommand := exec.CommandContext(availabilityContext, "docker", "version", "--format", "{{.Server.Version}}")
	availabilityCommand.Stdout = io.Discard
	availabilityCommand.Stderr = io.Discard

	if availabilityError := availabilityCommand.Run(); availabilityError != nil {
		testInstance.Skipf("Docker not available: %v", availabilityError)
	}
}
