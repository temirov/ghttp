package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDistributionArtifactsPresent(t *testing.T) {
	repositoryRoot := getRepositoryRoot(t)

	testCases := []struct {
		name             string
		relativePath     string
		expectedSnippets []string
	}{
		{
			name:         "windows dockerfile exists",
			relativePath: filepath.Join("docker", "Dockerfile.windows"),
			expectedSnippets: []string{
				"golang:1.25-windowsservercore-ltsc2022",
				"ARG GHTTP_WINDOWS_RUNTIME_IMAGE=mcr.microsoft.com/windows/nanoserver:ltsc2022",
				"ENTRYPOINT",
			},
		},
		{
			name:             "docker publish workflow covers windows",
			relativePath:     filepath.Join(".github", "workflows", "docker-publish.yml"),
			expectedSnippets: []string{"build-and-push-windows", "windows-latest", "docker/Dockerfile.windows", "windows/amd64"},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			targetPath := filepath.Join(repositoryRoot, testCase.relativePath)
			fileInfo, statErr := os.Stat(targetPath)
			if statErr != nil {
				t.Fatalf("expected file %s: %v", targetPath, statErr)
			}
			if fileInfo.IsDir() {
				t.Fatalf("expected file %s but found directory", targetPath)
			}
			fileContent, readErr := os.ReadFile(targetPath)
			if readErr != nil {
				t.Fatalf("read file %s: %v", targetPath, readErr)
			}
			contentString := string(fileContent)
			for _, snippet := range testCase.expectedSnippets {
				if !strings.Contains(contentString, snippet) {
					t.Fatalf("expected snippet %q in %s", snippet, targetPath)
				}
			}
		})
	}
}
