// Command ghttpd provides a minimal file server with a CLI compatible with
// `python -m http.server`: positional port, --bind/-b, --directory/-d,
// and --protocol/-p (HTTP/1.0 or HTTP/1.1). It also supports optional
// TLS via --tls-cert and --tls-key, which must be provided together.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/temirov/ghhtp/internal/serverdetails"
)

const (
	defaultPortString                       = "8000"
	defaultBindAddress                      = ""
	defaultDirectoryPath                    = "."
	defaultProtocolVersion                  = "HTTP/1.0"
	envVarNameGhttpdDisableDirIndex         = "GHTTPD_DISABLE_DIR_INDEX"
	headerNameServer                        = "Server"
	serverHeaderValue                       = "ghttpd"
	headerNameConnection                    = "Connection"
	headerValueClose                        = "close"
	logTimeLayout                           = "2006-01-02 15:04:05"
	logMessageCgiDeprecated                 = "--cgi is deprecated upstream and is not supported here"
	errorMessageInvalidProtocolFlag         = "unsupported --protocol value; use HTTP/1.0 or HTTP/1.1"
	errorMessageInvalidPortArgument         = "invalid port argument"
	errorMessageDirectoryDoesNotExist       = "directory does not exist"
	errorMessageTlsCertAndKeyMustBeTogether = "--tls-cert and --tls-key must be provided together"
	errorMessageTlsCertFileDoesNotExist     = "TLS certificate file does not exist"
	errorMessageTlsKeyFileDoesNotExist      = "TLS key file does not exist"
	logMessageServingHttp                   = "[%s] serving %s on http://%s (protocol %s)"
	logMessageServingHttps                  = "[%s] serving %s on https://%s (protocol %s)"
	logMessageReceivedSignal                = "received signal %s, shutting down"
	logMessageGracefulShutdownFailed        = "graceful shutdown failed: %v"
	logMessageServerError                   = "server error: %v"
)

type loggingHandler struct {
	innerHandler http.Handler
}

func (loggingHandlerInstance loggingHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	startTime := time.Now()
	log.Printf("%s %s %s from %s", request.Method, request.URL.Path, request.Proto, request.RemoteAddr)
	loggingHandlerInstance.innerHandler.ServeHTTP(responseWriter, request)
	duration := time.Since(startTime)
	log.Printf("served %s in %s", request.URL.Path, duration)
}

func main() {
	var bindAddressFlag string
	var directoryPathFlag string
	var protocolVersionFlag string
	var cgiEnabledFlag bool
	var tlsCertificatePathFlag string
	var tlsPrivateKeyPathFlag string

	flag.StringVar(&bindAddressFlag, "bind", defaultBindAddress, "Specify alternate bind address")
	flag.StringVar(&bindAddressFlag, "b", defaultBindAddress, "Specify alternate bind address (short)")
	flag.StringVar(&directoryPathFlag, "directory", defaultDirectoryPath, "Serve this directory")
	flag.StringVar(&directoryPathFlag, "d", defaultDirectoryPath, "Serve this directory (short)")
	flag.StringVar(&protocolVersionFlag, "protocol", defaultProtocolVersion, "HTTP/1.0 or HTTP/1.1")
	flag.StringVar(&protocolVersionFlag, "p", defaultProtocolVersion, "HTTP/1.0 or HTTP/1.1 (short)")
	flag.BoolVar(&cgiEnabledFlag, "cgi", false, "Enable CGI (deprecated/unsupported)")
	flag.StringVar(&tlsCertificatePathFlag, "tls-cert", "", "Path to TLS certificate (PEM)")
	flag.StringVar(&tlsPrivateKeyPathFlag, "tls-key", "", "Path to TLS private key (PEM)")
	flag.Parse()

	portString := defaultPortString
	if remainingArgs := flag.Args(); len(remainingArgs) > 0 {
		portCandidate := strings.TrimSpace(remainingArgs[0])
		if _, conversionErr := strconv.Atoi(portCandidate); conversionErr != nil {
			log.Fatalf("%s: %q", errorMessageInvalidPortArgument, portCandidate)
		}
		portString = portCandidate
	}

	if cgiEnabledFlag {
		log.Println(logMessageCgiDeprecated)
	}

	normalizedProtocol := strings.ToUpper(strings.TrimSpace(protocolVersionFlag))
	if normalizedProtocol != "HTTP/1.0" && normalizedProtocol != "HTTP/1.1" {
		log.Fatal(errorMessageInvalidProtocolFlag)
	}

	absoluteDirectoryPath, absoluteErr := filepath.Abs(directoryPathFlag)
	if absoluteErr != nil {
		log.Fatal(absoluteErr)
	}
	if statInfo, statErr := os.Stat(absoluteDirectoryPath); statErr != nil || !statInfo.IsDir() {
		log.Fatalf("%s: %s", errorMessageDirectoryDoesNotExist, absoluteDirectoryPath)
	}

	if (tlsCertificatePathFlag == "") != (tlsPrivateKeyPathFlag == "") {
		log.Fatal(errorMessageTlsCertAndKeyMustBeTogether)
	}
	if tlsCertificatePathFlag != "" {
		if _, certErr := os.Stat(tlsCertificatePathFlag); certErr != nil {
			log.Fatalf("%s: %s", errorMessageTlsCertFileDoesNotExist, tlsCertificatePathFlag)
		}
		if _, keyErr := os.Stat(tlsPrivateKeyPathFlag); keyErr != nil {
			log.Fatalf("%s: %s", errorMessageTlsKeyFileDoesNotExist, tlsPrivateKeyPathFlag)
		}
	}

	fileServerRoot := http.Dir(absoluteDirectoryPath)
	var baseFileHandler http.Handler = http.FileServer(fileServerRoot)

	if os.Getenv(envVarNameGhttpdDisableDirIndex) == "1" {
		baseFileHandler = http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			requestPath := request.URL.Path
			if strings.HasSuffix(requestPath, "/") {
				http.Error(responseWriter, "Directory listing disabled", http.StatusForbidden)
				return
			}
			http.FileServer(fileServerRoot).ServeHTTP(responseWriter, request)
		})
	}

	wrappingHandler := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set(headerNameServer, serverHeaderValue)
		if normalizedProtocol == "HTTP/1.0" {
			responseWriter.Header().Set(headerNameConnection, headerValueClose)
		}
		baseFileHandler.ServeHTTP(responseWriter, request)
	})

	listenAddress := net.JoinHostPort(bindAddressFlag, portString)
	servingAddressFormatter := serverdetails.NewServingAddressFormatter()
	displayAddress := servingAddressFormatter.FormatHostAndPortForLogging(bindAddressFlag, portString)

	httpServer := &http.Server{
		Addr:              listenAddress,
		Handler:           loggingHandler{innerHandler: wrappingHandler},
		ReadHeaderTimeout: 15 * time.Second,
	}

	if normalizedProtocol == "HTTP/1.0" {
		httpServer.DisableGeneralOptionsHandler = true
		httpServer.IdleTimeout = 0
		httpServer.ReadTimeout = 0
		httpServer.WriteTimeout = 0
		httpServer.SetKeepAlivesEnabled(false)
	}

	log.SetFlags(0)
	nowString := time.Now().Format(logTimeLayout)
	if tlsCertificatePathFlag == "" {
		log.Printf(logMessageServingHttp, nowString, absoluteDirectoryPath, displayAddress, normalizedProtocol)
	} else {
		log.Printf(logMessageServingHttps, nowString, absoluteDirectoryPath, displayAddress, normalizedProtocol)
	}

	terminationSignals := make(chan os.Signal, 1)
	signal.Notify(terminationSignals, syscall.SIGINT, syscall.SIGTERM)

	serverErrors := make(chan error, 1)
	go func() {
		if tlsCertificatePathFlag == "" {
			serverErrors <- httpServer.ListenAndServe()
		} else {
			serverErrors <- httpServer.ListenAndServeTLS(tlsCertificatePathFlag, tlsPrivateKeyPathFlag)
		}
	}()

	select {
	case terminateSignal := <-terminationSignals:
		log.Printf(logMessageReceivedSignal, terminateSignal.String())
		shutdownContext, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancelFunc()
		if shutdownErr := httpServer.Shutdown(shutdownContext); shutdownErr != nil {
			log.Printf(logMessageGracefulShutdownFailed, shutdownErr)
			_ = httpServer.Close()
		}
	case listenErr := <-serverErrors:
		if listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			log.Fatalf(logMessageServerError, listenErr)
		}
	}
}
