package server

import (
	"bytes"
	"html"
	"io"
	"io/fs"
	"net/http"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/temirov/ghttp/internal/markdown"
)

var directoryIndexCandidates = []string{"index.html", "index.htm"}

type markdownHandler struct {
	next                    http.Handler
	fileSystem              http.FileSystem
	disableDirectoryListing bool
	enableDirectoryMarkdown bool
}

func newMarkdownHandler(next http.Handler, fileSystem http.FileSystem, disableDirectoryListing bool, enableDirectoryMarkdown bool) http.Handler {
	return markdownHandler{
		next:                    next,
		fileSystem:              fileSystem,
		disableDirectoryListing: disableDirectoryListing,
		enableDirectoryMarkdown: enableDirectoryMarkdown,
	}
}

func (handler markdownHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	file, openErr := handler.fileSystem.Open(request.URL.Path)
	if openErr != nil {
		handler.next.ServeHTTP(responseWriter, request)
		return
	}

	fileInfo, statErr := file.Stat()
	file.Close()
	if statErr != nil {
		handler.next.ServeHTTP(responseWriter, request)
		return
	}

	if fileInfo.IsDir() {
		handler.serveDirectory(responseWriter, request)
		return
	}

	if !isMarkdownFile(fileInfo.Name()) {
		handler.next.ServeHTTP(responseWriter, request)
		return
	}

	handler.serveMarkdownFile(responseWriter, request, request.URL.Path, fileInfo)
}

func (handler markdownHandler) serveDirectory(responseWriter http.ResponseWriter, request *http.Request) {
	if !strings.HasSuffix(request.URL.Path, "/") {
		handler.next.ServeHTTP(responseWriter, request)
		return
	}

	if !handler.enableDirectoryMarkdown {
		handler.next.ServeHTTP(responseWriter, request)
		return
	}

	if handler.directoryIndexExists(request.URL.Path) {
		handler.next.ServeHTTP(responseWriter, request)
		return
	}

	candidatePath, candidateInfo, candidateErr := handler.selectMarkdownCandidate(request.URL.Path)
	if candidateErr == nil && candidatePath != "" {
		handler.serveMarkdownFile(responseWriter, request, candidatePath, candidateInfo)
		return
	}

	if handler.disableDirectoryListing {
		http.Error(responseWriter, errorMessageDirectoryListingDisabled, http.StatusForbidden)
		return
	}

	handler.next.ServeHTTP(responseWriter, request)
}

func (handler markdownHandler) serveMarkdownFile(responseWriter http.ResponseWriter, request *http.Request, markdownPath string, markdownInfo fs.FileInfo) {
	file, openErr := handler.fileSystem.Open(markdownPath)
	if openErr != nil {
		handler.next.ServeHTTP(responseWriter, request)
		return
	}
	defer file.Close()

	contentBytes, readErr := io.ReadAll(file)
	if readErr != nil {
		handler.next.ServeHTTP(responseWriter, request)
		return
	}

	renderedHTML, renderErr := markdown.ToHTML(contentBytes)
	if renderErr != nil {
		handler.next.ServeHTTP(responseWriter, request)
		return
	}

	documentTitle := strings.TrimSuffix(markdownInfo.Name(), filepath.Ext(markdownInfo.Name()))
	document := buildHTMLDocument(documentTitle, renderedHTML)
	reader := bytes.NewReader(document)

	documentName := documentTitle + ".html"
	http.ServeContent(responseWriter, request, documentName, markdownInfo.ModTime(), reader)
}

func (handler markdownHandler) selectMarkdownCandidate(directoryPath string) (string, fs.FileInfo, error) {
	directoryHandle, openErr := handler.fileSystem.Open(directoryPath)
	if openErr != nil {
		return "", nil, openErr
	}
	defer directoryHandle.Close()

	entries, readErr := directoryHandle.Readdir(-1)
	if readErr != nil {
		return "", nil, readErr
	}

	var markdownEntries []fs.FileInfo
	for index := range entries {
		entry := entries[index]
		if entry.IsDir() {
			continue
		}
		if !isMarkdownFile(entry.Name()) {
			continue
		}
		if strings.EqualFold(entry.Name(), "README.md") {
			candidate := pathpkg.Join(directoryPath, entry.Name())
			return candidate, entry, nil
		}
		markdownEntries = append(markdownEntries, entry)
	}

	if len(markdownEntries) == 1 {
		candidate := pathpkg.Join(directoryPath, markdownEntries[0].Name())
		return candidate, markdownEntries[0], nil
	}

	return "", nil, nil
}

func (handler markdownHandler) directoryIndexExists(directoryPath string) bool {
	for index := range directoryIndexCandidates {
		candidateName := directoryIndexCandidates[index]
		candidatePath := pathpkg.Join(directoryPath, candidateName)
		fileHandle, openErr := handler.fileSystem.Open(candidatePath)
		if openErr != nil {
			continue
		}
		candidateInfo, statErr := fileHandle.Stat()
		fileHandle.Close()
		if statErr != nil {
			continue
		}
		if candidateInfo.IsDir() {
			continue
		}
		return true
	}
	return false
}

func buildHTMLDocument(title string, body []byte) []byte {
	var builder strings.Builder
	builder.WriteString("<!DOCTYPE html><html lang=\"en\"><head><meta charset=\"utf-8\"><title>")
	builder.WriteString(html.EscapeString(title))
	builder.WriteString("</title></head><body>")
	builder.Write(body)
	builder.WriteString("</body></html>")
	return []byte(builder.String())
}

func isMarkdownFile(fileName string) bool {
	return strings.EqualFold(filepath.Ext(fileName), ".md")
}

func newDirectoryGuardHandler(next http.Handler, _ http.FileSystem) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/") {
			http.Error(responseWriter, errorMessageDirectoryListingDisabled, http.StatusForbidden)
			return
		}
		next.ServeHTTP(responseWriter, request)
	})
}
