package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/temirov/ghttp/internal/serverdetails"
)

func TestMarkdownFilesServedAsHTML(t *testing.T) {
	temporaryDirectory := t.TempDir()
	writeFile(t, filepath.Join(temporaryDirectory, "hello.md"), "# Heading\n\nParagraph.")

	handler := newTestFileServerHandler(temporaryDirectory, true, false)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/hello.md", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", recorder.Code)
	}
	contentType := recorder.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected HTML content type, got %s", contentType)
	}
	bodyBytes, readErr := io.ReadAll(recorder.Result().Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	if !strings.Contains(string(bodyBytes), "<h1>Heading</h1>") {
		t.Fatalf("expected rendered markdown heading, body: %s", string(bodyBytes))
	}
}

func TestDirectoryReadmeServedAutomatically(t *testing.T) {
	temporaryDirectory := t.TempDir()
	docsDirectory := filepath.Join(temporaryDirectory, "docs")
	mustMkDir(t, docsDirectory)
	writeFile(t, filepath.Join(docsDirectory, "README.md"), "# Docs\n")

	handler := newTestFileServerHandler(temporaryDirectory, true, false)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/docs/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", recorder.Code)
	}
	bodyBytes, readErr := io.ReadAll(recorder.Result().Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	if !strings.Contains(string(bodyBytes), "<h1>Docs</h1>") {
		t.Fatalf("expected README content in body: %s", string(bodyBytes))
	}
}

func TestSingleMarkdownInDirectoryServed(t *testing.T) {
	temporaryDirectory := t.TempDir()
	singleDirectory := filepath.Join(temporaryDirectory, "single")
	mustMkDir(t, singleDirectory)
	writeFile(t, filepath.Join(singleDirectory, "only.md"), "# Solo\n")

	handler := newTestFileServerHandler(temporaryDirectory, true, false)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/single/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", recorder.Code)
	}
	bodyBytes, readErr := io.ReadAll(recorder.Result().Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	if !strings.Contains(string(bodyBytes), "<h1>Solo</h1>") {
		t.Fatalf("expected markdown content served, body: %s", string(bodyBytes))
	}
}

func TestDirectoryListingDisabledWithoutMarkdown(t *testing.T) {
	temporaryDirectory := t.TempDir()
	emptyDirectory := filepath.Join(temporaryDirectory, "empty")
	mustMkDir(t, emptyDirectory)

	handler := newTestFileServerHandler(temporaryDirectory, false, true)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/empty/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 status, got %d", recorder.Code)
	}
}

func newTestFileServerHandler(rootDirectory string, enableMarkdown bool, disableDirectoryListing bool) http.Handler {
	fileServerInstance := NewFileServer(zap.NewNop(), serverdetails.NewServingAddressFormatter())
	configuration := FileServerConfiguration{
		DirectoryPath:           rootDirectory,
		EnableMarkdown:          enableMarkdown,
		DisableDirectoryListing: disableDirectoryListing,
	}
	return fileServerInstance.buildFileHandler(configuration)
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	writeErr := os.WriteFile(path, []byte(content), 0o600)
	if writeErr != nil {
		t.Fatalf("write file %s: %v", path, writeErr)
	}
}

func mustMkDir(t *testing.T, path string) {
	t.Helper()
	makeErr := os.MkdirAll(path, 0o755)
	if makeErr != nil {
		t.Fatalf("mkdir %s: %v", path, makeErr)
	}
}
