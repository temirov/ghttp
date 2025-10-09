package server

import (
	"net/http"
	pathpkg "path"
	"strings"
)

const (
	initialFileRootRequestPath = "/"
)

type initialFileHandler struct {
	next               http.Handler
	initialRequestPath string
}

func newInitialFileHandler(next http.Handler, initialFileRelativePath string) http.Handler {
	cleanPath := pathpkg.Clean(pathpkg.Join(initialFileRootRequestPath, strings.ReplaceAll(initialFileRelativePath, "\\", "/")))
	if !strings.HasPrefix(cleanPath, initialFileRootRequestPath) {
		cleanPath = initialFileRootRequestPath
	}
	return initialFileHandler{
		next:               next,
		initialRequestPath: cleanPath,
	}
}

func (handler initialFileHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path != initialFileRootRequestPath {
		handler.next.ServeHTTP(responseWriter, request)
		return
	}
	clonedRequest := request.Clone(request.Context())
	urlCopy := *request.URL
	urlCopy.Path = handler.initialRequestPath
	urlCopy.RawPath = handler.initialRequestPath
	clonedRequest.URL = &urlCopy
	clonedRequest.RequestURI = handler.initialRequestPath
	handler.next.ServeHTTP(responseWriter, clonedRequest)
}
