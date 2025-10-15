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
		name              string
		relativePath      string
		shouldExist       bool
		expectedSnippets  []string
		forbiddenSnippets []string
	}{
		{
			name:         "windows dockerfile absent",
			relativePath: filepath.Join("docker", "Dockerfile.windows"),
			shouldExist:  false,
		},
		{
			name:              "docker publish workflow targets linux platforms",
			relativePath:      filepath.Join(".github", "workflows", "docker-publish.yml"),
			shouldExist:       true,
			expectedSnippets:  []string{"default_platforms: linux/amd64,linux/arm64"},
			forbiddenSnippets: []string{"windows/amd64", "Dockerfile.windows"},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			targetPath := filepath.Join(repositoryRoot, testCase.relativePath)
			fileInfo, statErr := os.Stat(targetPath)
			if !testCase.shouldExist {
				if statErr == nil {
					t.Fatalf("expected %s to be absent, but it exists", targetPath)
				}
				if !os.IsNotExist(statErr) {
					t.Fatalf("unexpected error while checking %s: %v", targetPath, statErr)
				}
				return
			}

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
			for _, snippet := range testCase.forbiddenSnippets {
				if strings.Contains(contentString, snippet) {
					t.Fatalf("did not expect snippet %q in %s", snippet, targetPath)
				}
			}
		})
	}
}
