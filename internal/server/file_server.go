package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/temirov/ghttp/internal/serverdetails"
	"github.com/temirov/ghttp/pkg/logging"
)

const (
	defaultLogTimeLayout                 = "2006-01-02 15:04:05"
	serverHeaderName                     = "Server"
	serverHeaderValue                    = "ghttpd"
	connectionHeaderName                 = "Connection"
	connectionCloseValue                 = "close"
	httpProtocolVersionOneZero           = "HTTP/1.0"
	errorMessageDirectoryListingDisabled = "Directory listing disabled"
	consoleRequestTimeLayout             = "02/Jan/2006 15:04:05"
	logFieldDirectory                    = "directory"
	logFieldProtocol                     = "protocol"
	logFieldURL                          = "url"
	logFieldMethod                       = "method"
	logFieldPath                         = "path"
	logFieldRemote                       = "remote"
	logFieldDuration                     = "duration"
	logFieldStatus                       = "status"
	logFieldTimestamp                    = "timestamp"
	logMessageServingHTTP                = "serving http"
	logMessageServingHTTPS               = "serving https"
	logMessageShutdownInitiated          = "shutdown initiated"
	logMessageShutdownCompleted          = "shutdown completed"
	logMessageShutdownFailed             = "shutdown failed"
	logMessageServerError                = "server error"
	logMessageRequestStarted             = "request started"
	logMessageRequestCompleted           = "request completed"
	shutdownGracePeriod                  = 3 * time.Second
)

type FileServerConfiguration struct {
	BindAddress             string
	Port                    string
	DirectoryPath           string
	ProtocolVersion         string
	DisableDirectoryListing bool
	EnableMarkdown          bool
	BrowseDirectories       bool
	InitialFileRelativePath string
	LoggingType             string
	TLS                     *TLSConfiguration
}

// TLSConfiguration describes transport layer security configuration.
type TLSConfiguration struct {
	CertificatePath   string
	PrivateKeyPath    string
	LoadedCertificate *tls.Certificate
}

// FileServer serves files over HTTP or HTTPS.
type FileServer struct {
	loggingService          *logging.Service
	servingAddressFormatter serverdetails.ServingAddressFormatter
}

// NewFileServer constructs a FileServer.
func NewFileServer(loggingService *logging.Service, servingAddressFormatter serverdetails.ServingAddressFormatter) FileServer {
	return FileServer{loggingService: loggingService, servingAddressFormatter: servingAddressFormatter}
}

// Serve runs the HTTP server until the context is cancelled or an error occurs.
func (fileServer FileServer) Serve(ctx context.Context, configuration FileServerConfiguration) error {
	if fileServer.loggingService == nil {
		return errors.New("logging service not configured")
	}
	listeningAddress := net.JoinHostPort(configuration.BindAddress, configuration.Port)
	displayAddress := fileServer.servingAddressFormatter.FormatHostAndPortForLogging(configuration.BindAddress, configuration.Port)
	fileHandler := fileServer.buildFileHandler(configuration)
	wrappedHandler := fileServer.wrapWithHeaders(fileHandler, configuration.ProtocolVersion)
	loggingType := fileServer.loggingService.Type()
	if configuration.LoggingType != "" {
		loggingType = configuration.LoggingType
	}
	normalizedLoggingType, normalizeErr := logging.NormalizeType(loggingType)
	if normalizeErr != nil {
		return fmt.Errorf("normalize logging type: %w", normalizeErr)
	}
	loggingType = normalizedLoggingType
	loggingHandler := fileServer.wrapWithLogging(wrappedHandler, loggingType)

	server := &http.Server{
		Addr:              listeningAddress,
		Handler:           loggingHandler,
		ReadHeaderTimeout: 15 * time.Second,
	}

	if configuration.ProtocolVersion == httpProtocolVersionOneZero {
		server.DisableGeneralOptionsHandler = true
		server.SetKeepAlivesEnabled(false)
	}

	certificateConfigured, configureErr := fileServer.configureTLS(server, configuration.TLS)
	if configureErr != nil {
		return fmt.Errorf("configure tls: %w", configureErr)
	}

	currentTime := time.Now().Format(defaultLogTimeLayout)
	if loggingType == logging.TypeConsole {
		startMessage := formatConsoleStartMessage(configuration, certificateConfigured, displayAddress)
		fileServer.loggingService.Info(startMessage)
	} else {
		fullURLScheme := "http"
		activeMessage := logMessageServingHTTP
		if certificateConfigured {
			fullURLScheme = "https"
			activeMessage = logMessageServingHTTPS
		}
		fullURL := fmt.Sprintf("%s://%s", fullURLScheme, displayAddress)
		fileServer.loggingService.Info(
			activeMessage,
			logging.String(logFieldDirectory, configuration.DirectoryPath),
			logging.String(logFieldProtocol, configuration.ProtocolVersion),
			logging.String(logFieldURL, fullURL),
			logging.String(logFieldTimestamp, currentTime),
		)
	}

	serverErrors := make(chan error, 1)
	go func() {
		var serveErr error
		if certificateConfigured {
			serveErr = server.ListenAndServeTLS("", "")
		} else {
			serveErr = server.ListenAndServe()
		}
		serverErrors <- serveErr
	}()

	select {
	case <-ctx.Done():
		fileServer.loggingService.Info(logMessageShutdownInitiated)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer cancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			fileServer.loggingService.Error(logMessageShutdownFailed, shutdownErr)
			return fmt.Errorf("shutdown server: %w", shutdownErr)
		}
		fileServer.loggingService.Info(logMessageShutdownCompleted)
		return nil
	case serveErr := <-serverErrors:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			if isAddressInUse(serveErr) {
				friendlyMessage := formatAddressInUseMessage(configuration)
				fileServer.loggingService.Error(friendlyMessage, serveErr)
				return fmt.Errorf("address in use: %s", friendlyMessage)
			}
			fileServer.loggingService.Error(logMessageServerError, serveErr)
			return fmt.Errorf("serve http: %w", serveErr)
		}
		return nil
	}
}

func (fileServer FileServer) buildFileHandler(configuration FileServerConfiguration) http.Handler {
	fileSystem := http.Dir(configuration.DirectoryPath)
	baseHandler := http.FileServer(fileSystem)
	handler := baseHandler
	if configuration.EnableMarkdown {
		handler = newMarkdownHandler(handler, fileSystem, configuration.DisableDirectoryListing, !configuration.BrowseDirectories)
	} else if configuration.DisableDirectoryListing && !configuration.BrowseDirectories {
		handler = newDirectoryGuardHandler(handler, fileSystem)
	}
	if configuration.BrowseDirectories {
		handler = newBrowseHandler(handler, fileSystem)
	}
	if configuration.InitialFileRelativePath != "" && !configuration.BrowseDirectories {
		handler = newInitialFileHandler(handler, configuration.InitialFileRelativePath)
	}
	return handler
}

func (fileServer FileServer) wrapWithHeaders(handler http.Handler, protocolVersion string) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set(serverHeaderName, serverHeaderValue)
		if protocolVersion == httpProtocolVersionOneZero {
			responseWriter.Header().Set(connectionHeaderName, connectionCloseValue)
		}
		handler.ServeHTTP(responseWriter, request)
	})
}

func (fileServer FileServer) wrapWithLogging(handler http.Handler, loggingType string) http.Handler {
	if fileServer.loggingService == nil {
		return handler
	}
	switch loggingType {
	case logging.TypeConsole:
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			recordedWriter := newStatusRecorder(responseWriter)
			startTime := time.Now()
			handler.ServeHTTP(recordedWriter, request)
			message := formatConsoleRequestLog(request, recordedWriter.statusCode, recordedWriter.bytesWritten, startTime)
			fileServer.loggingService.Info(message)
		})
	default:
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			recordedWriter := newStatusRecorder(responseWriter)
			startTime := time.Now()
			fileServer.loggingService.Info(
				logMessageRequestStarted,
				logging.String(logFieldMethod, request.Method),
				logging.String(logFieldPath, request.URL.Path),
				logging.String(logFieldProtocol, request.Proto),
				logging.String(logFieldRemote, request.RemoteAddr),
			)
			handler.ServeHTTP(recordedWriter, request)
			duration := time.Since(startTime)
			fileServer.loggingService.Info(
				logMessageRequestCompleted,
				logging.String(logFieldMethod, request.Method),
				logging.String(logFieldPath, request.URL.Path),
				logging.Int(logFieldStatus, recordedWriter.statusCode),
				logging.Duration(logFieldDuration, duration),
				logging.String(logFieldRemote, request.RemoteAddr),
			)
		})
	}
}

func formatConsoleStartMessage(configuration FileServerConfiguration, certificateConfigured bool, displayAddress string) string {
	bindAddress := configuration.BindAddress
	if strings.TrimSpace(bindAddress) == "" {
		bindAddress = "0.0.0.0"
	}
	port := configuration.Port
	scheme := "http"
	schemeLabel := "HTTP"
	if certificateConfigured {
		scheme = "https"
		schemeLabel = "HTTPS"
	}
	return fmt.Sprintf("Serving %s on %s port %s (%s://%s/) ...", schemeLabel, bindAddress, port, scheme, displayAddress)
}

func formatConsoleRequestLog(request *http.Request, statusCode int, bytesWritten int, startTime time.Time) string {
	clientAddress := request.RemoteAddr
	if host, _, err := net.SplitHostPort(clientAddress); err == nil {
		clientAddress = host
	}
	timestamp := startTime.Format(consoleRequestTimeLayout)
	requestTarget := request.URL.RequestURI()
	if requestTarget == "" {
		requestTarget = request.URL.Path
	}
	requestLine := fmt.Sprintf("%s %s %s", request.Method, requestTarget, request.Proto)
	sizeField := "-"
	if bytesWritten > 0 {
		sizeField = strconv.Itoa(bytesWritten)
	}
	return fmt.Sprintf("%s - - [%s] \"%s\" %d %s", clientAddress, timestamp, requestLine, statusCode, sizeField)
}

func (fileServer FileServer) configureTLS(server *http.Server, configuration *TLSConfiguration) (bool, error) {
	if configuration == nil {
		return false, nil
	}
	if configuration.LoadedCertificate != nil {
		server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{*configuration.LoadedCertificate}}
		return true, nil
	}
	if configuration.CertificatePath == "" || configuration.PrivateKeyPath == "" {
		return false, errors.New("both certificate and private key paths must be provided")
	}
	certificate, err := tls.LoadX509KeyPair(configuration.CertificatePath, configuration.PrivateKeyPath)
	if err != nil {
		return false, err
	}
	server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{certificate}}
	return true, nil
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (recorder *statusRecorder) WriteHeader(statusCode int) {
	recorder.statusCode = statusCode
	recorder.ResponseWriter.WriteHeader(statusCode)
}

func (recorder *statusRecorder) Write(content []byte) (int, error) {
	written, err := recorder.ResponseWriter.Write(content)
	recorder.bytesWritten += written
	return written, err
}

func newStatusRecorder(responseWriter http.ResponseWriter) *statusRecorder {
	recorder := &statusRecorder{ResponseWriter: responseWriter, statusCode: http.StatusOK}
	return recorder
}

func formatAddressInUseMessage(configuration FileServerConfiguration) string {
	bindAddress := configuration.BindAddress
	if strings.TrimSpace(bindAddress) == "" {
		bindAddress = "0.0.0.0"
	}
	return fmt.Sprintf("Address already in use: %s:%s", bindAddress, configuration.Port)
}

func isAddressInUse(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.EADDRINUSE) {
			return true
		}
		var syscallErr *os.SyscallError
		if errors.As(opErr.Err, &syscallErr) {
			return errors.Is(syscallErr.Err, syscall.EADDRINUSE)
		}
	}
	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		return errors.Is(syscallErr.Err, syscall.EADDRINUSE)
	}
	return errors.Is(err, syscall.EADDRINUSE)
}
