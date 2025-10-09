package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/temirov/ghttp/internal/serverdetails"
	"github.com/temirov/ghttp/pkg/logging"
)

func TestIntegrationFileServerServesMarkdownAsHTML(t *testing.T) {
	temporaryDirectory := t.TempDir()
	writeFile(t, filepath.Join(temporaryDirectory, "hello.md"), "# Heading\n\nParagraph.")

	handler := newTestFileServerHandler(temporaryDirectory, true, false, false, "")

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

func TestIntegrationFileServerServesDirectoryReadmeAutomatically(t *testing.T) {
	temporaryDirectory := t.TempDir()
	docsDirectory := filepath.Join(temporaryDirectory, "docs")
	mustMkDir(t, docsDirectory)
	writeFile(t, filepath.Join(docsDirectory, "README.md"), "# Docs\n")

	handler := newTestFileServerHandler(temporaryDirectory, true, false, false, "")

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

func TestIntegrationFileServerServesSingleMarkdownFromDirectory(t *testing.T) {
	temporaryDirectory := t.TempDir()
	singleDirectory := filepath.Join(temporaryDirectory, "single")
	mustMkDir(t, singleDirectory)
	writeFile(t, filepath.Join(singleDirectory, "only.md"), "# Solo\n")

	handler := newTestFileServerHandler(temporaryDirectory, true, false, false, "")

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

func TestIntegrationFileServerDisablesDirectoryListingWithoutMarkdown(t *testing.T) {
	temporaryDirectory := t.TempDir()
	emptyDirectory := filepath.Join(temporaryDirectory, "empty")
	mustMkDir(t, emptyDirectory)

	handler := newTestFileServerHandler(temporaryDirectory, false, true, false, "")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/empty/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 status, got %d", recorder.Code)
	}
}

func TestIntegrationFileServerPrefersIndexHtmlOverReadme(t *testing.T) {
	temporaryDirectory := t.TempDir()
	sectionDirectory := filepath.Join(temporaryDirectory, "section")
	mustMkDir(t, sectionDirectory)
	writeFile(t, filepath.Join(sectionDirectory, "index.html"), "<html><body>Index page</body></html>")
	writeFile(t, filepath.Join(sectionDirectory, "README.md"), "# Section\n")

	handler := newTestFileServerHandler(temporaryDirectory, true, false, false, "")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/section/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", recorder.Code)
	}
	bodyBytes, readErr := io.ReadAll(recorder.Result().Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	responseBody := string(bodyBytes)
	if !strings.Contains(responseBody, "Index page") {
		t.Fatalf("expected index page content, body: %s", responseBody)
	}
	if strings.Contains(responseBody, "<h1>Section</h1>") {
		t.Fatalf("expected markdown rendering to be skipped when index is present, body: %s", responseBody)
	}
}

func TestIntegrationFileServerServesMarkdownVerbatimWhenRenderingDisabled(t *testing.T) {
	temporaryDirectory := t.TempDir()
	writeFile(t, filepath.Join(temporaryDirectory, "notes.md"), "# Notes\n\nContent.")

	handler := newTestFileServerHandler(temporaryDirectory, false, false, false, "")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/notes.md", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", recorder.Code)
	}
	bodyBytes, readErr := io.ReadAll(recorder.Result().Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	responseBody := string(bodyBytes)
	if !strings.Contains(responseBody, "# Notes") {
		t.Fatalf("expected markdown content to be served verbatim, body: %s", responseBody)
	}
	if strings.Contains(responseBody, "<h1>Notes</h1>") {
		t.Fatalf("expected markdown rendering to be disabled, body: %s", responseBody)
	}
}

func TestIntegrationFileServerBrowseModeListsDirectory(t *testing.T) {
	temporaryDirectory := t.TempDir()
	exampleDirectory := filepath.Join(temporaryDirectory, "example")
	mustMkDir(t, exampleDirectory)
	writeFile(t, filepath.Join(exampleDirectory, "index.html"), "<html><body>Index</body></html>")
	writeFile(t, filepath.Join(exampleDirectory, "README.md"), "# Example\n")

	handler := newTestFileServerHandler(temporaryDirectory, true, false, true, "")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/example/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", recorder.Code)
	}
	bodyBytes, readErr := io.ReadAll(recorder.Result().Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	responseBody := string(bodyBytes)
	if !strings.Contains(responseBody, "index.html") {
		t.Fatalf("expected index.html link in directory listing, body: %s", responseBody)
	}
	if !strings.Contains(responseBody, "README.md") {
		t.Fatalf("expected README.md link in directory listing, body: %s", responseBody)
	}
	if strings.Contains(responseBody, "<h1>Example</h1>") {
		t.Fatalf("expected markdown rendering to be bypassed for directory view, body: %s", responseBody)
	}
}

func TestIntegrationFileServerBrowseModeRendersMarkdownOnDirectRequest(t *testing.T) {
	temporaryDirectory := t.TempDir()
	exampleDirectory := filepath.Join(temporaryDirectory, "example")
	mustMkDir(t, exampleDirectory)
	writeFile(t, filepath.Join(exampleDirectory, "README.md"), "# Example\n")

	handler := newTestFileServerHandler(temporaryDirectory, true, false, true, "")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/example/README.md", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", recorder.Code)
	}
	bodyBytes, readErr := io.ReadAll(recorder.Result().Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	responseBody := string(bodyBytes)
	if !strings.Contains(responseBody, "<h1>Example</h1>") {
		t.Fatalf("expected markdown rendering on direct request, body: %s", responseBody)
	}
}

func TestIntegrationFileServerBrowseModeListsRootDirectory(t *testing.T) {
	temporaryDirectory := t.TempDir()
	writeFile(t, filepath.Join(temporaryDirectory, "index.html"), "<html><body>Index page</body></html>")
	writeFile(t, filepath.Join(temporaryDirectory, "README.md"), "# Root\n")

	handler := newTestFileServerHandler(temporaryDirectory, true, false, true, "")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", recorder.Code)
	}
	bodyBytes, readErr := io.ReadAll(recorder.Result().Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	responseBody := string(bodyBytes)
	if !strings.Contains(responseBody, "index.html") {
		t.Fatalf("expected index.html link in directory listing, body: %s", responseBody)
	}
	if strings.Contains(responseBody, "Index page") {
		t.Fatalf("expected directory listing instead of index file content, body: %s", responseBody)
	}
}

func TestIntegrationFileServerServesInitialHtmlFileAtRoot(t *testing.T) {
	temporaryDirectory := t.TempDir()
	writeFile(t, filepath.Join(temporaryDirectory, "cat.html"), "<html><body>Cat</body></html>")

	handler := newTestFileServerHandler(temporaryDirectory, false, false, false, "cat.html")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", recorder.Code)
	}
	bodyBytes, readErr := io.ReadAll(recorder.Result().Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	responseBody := string(bodyBytes)
	if !strings.Contains(responseBody, "Cat") {
		t.Fatalf("expected initial HTML file content, body: %s", responseBody)
	}
}

func TestIntegrationFileServerServesInitialMarkdownFileAtRoot(t *testing.T) {
	temporaryDirectory := t.TempDir()
	writeFile(t, filepath.Join(temporaryDirectory, "guide.md"), "# Guide\n")

	handler := newTestFileServerHandler(temporaryDirectory, true, false, false, "guide.md")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", recorder.Code)
	}
	bodyBytes, readErr := io.ReadAll(recorder.Result().Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	responseBody := string(bodyBytes)
	if !strings.Contains(responseBody, "<h1>Guide</h1>") {
		t.Fatalf("expected rendered markdown content, body: %s", responseBody)
	}
}

func newTestFileServerHandler(rootDirectory string, enableMarkdown bool, disableDirectoryListing bool, browseDirectories bool, initialFileRelativePath string) http.Handler {
	fileServerInstance := NewFileServer(logging.NewTestService(logging.TypeConsole), serverdetails.NewServingAddressFormatter())
	configuration := FileServerConfiguration{
		DirectoryPath:           rootDirectory,
		EnableMarkdown:          enableMarkdown,
		DisableDirectoryListing: disableDirectoryListing,
		BrowseDirectories:       browseDirectories,
		InitialFileRelativePath: initialFileRelativePath,
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
