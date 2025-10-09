package server

import (
	"html"
	"io/fs"
	"net/http"
	pathpkg "path"
	"slices"
	"strings"
)

const (
	directoryListingContentType    = "text/html; charset=utf-8"
	directoryListingHeaderName     = "Content-Type"
	directoryListingDocumentStart  = "<!DOCTYPE html><html lang=\"en\"><head><meta charset=\"utf-8\"><title>Index of "
	directoryListingDocumentMiddle = "</title></head><body><h1>Index of "
	directoryListingDocumentList   = "</h1><ul>"
	directoryListingItemStart      = "<li><a href=\""
	directoryListingItemMiddle     = "\">"
	directoryListingItemEnd        = "</a></li>"
	directoryListingDocumentEnd    = "</ul></body></html>"
)

type browseHandler struct {
	next       http.Handler
	fileSystem http.FileSystem
}

func newBrowseHandler(next http.Handler, fileSystem http.FileSystem) http.Handler {
	return browseHandler{
		next:       next,
		fileSystem: fileSystem,
	}
}

func (handler browseHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	if !strings.HasSuffix(request.URL.Path, "/") || request.URL.Path == "" {
		handler.next.ServeHTTP(responseWriter, request)
		return
	}

	directoryFile, openErr := handler.fileSystem.Open(request.URL.Path)
	if openErr != nil {
		handler.next.ServeHTTP(responseWriter, request)
		return
	}
	defer directoryFile.Close()

	directoryInfo, statErr := directoryFile.Stat()
	if statErr != nil || !directoryInfo.IsDir() {
		handler.next.ServeHTTP(responseWriter, request)
		return
	}

	entries, readErr := directoryFile.Readdir(-1)
	if readErr != nil {
		handler.next.ServeHTTP(responseWriter, request)
		return
	}

	handler.renderListing(responseWriter, request, entries)
}

func (handler browseHandler) renderListing(responseWriter http.ResponseWriter, request *http.Request, entries []fs.FileInfo) {
	slices.SortFunc(entries, func(left fs.FileInfo, right fs.FileInfo) int {
		leftName := left.Name()
		rightName := right.Name()
		return strings.Compare(strings.ToLower(leftName), strings.ToLower(rightName))
	})

	var builder strings.Builder
	builder.WriteString(directoryListingDocumentStart)
	builder.WriteString(html.EscapeString(request.URL.Path))
	builder.WriteString(directoryListingDocumentMiddle)
	builder.WriteString(html.EscapeString(request.URL.Path))
	builder.WriteString(directoryListingDocumentList)

	for index := range entries {
		entry := entries[index]
		name := entry.Name()
		displayName := name
		relativePath := name
		if entry.IsDir() {
			displayName += "/"
			relativePath += "/"
		}
		link := pathpkg.Join(request.URL.Path, relativePath)
		if entry.IsDir() && !strings.HasSuffix(link, "/") {
			link += "/"
		}
		builder.WriteString(directoryListingItemStart)
		builder.WriteString(html.EscapeString(link))
		builder.WriteString(directoryListingItemMiddle)
		builder.WriteString(html.EscapeString(displayName))
		builder.WriteString(directoryListingItemEnd)
	}

	builder.WriteString(directoryListingDocumentEnd)

	responseWriter.Header().Set(directoryListingHeaderName, directoryListingContentType)
	_, _ = responseWriter.Write([]byte(builder.String()))
}
