package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/temirov/ghttp/internal/serverdetails"
)

const (
	defaultLogTimeLayout                 = "2006-01-02 15:04:05"
	serverHeaderName                     = "Server"
	serverHeaderValue                    = "ghttpd"
	connectionHeaderName                 = "Connection"
	connectionCloseValue                 = "close"
	httpProtocolVersionOneZero           = "HTTP/1.0"
	errorMessageDirectoryListingDisabled = "Directory listing disabled"
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

// TLSConfiguration describes transport layer security configuration.
type TLSConfiguration struct {
	CertificatePath   string
	PrivateKeyPath    string
	LoadedCertificate *tls.Certificate
}

// FileServerConfiguration declares how the HTTP server should run.
type FileServerConfiguration struct {
	BindAddress             string
	Port                    string
	DirectoryPath           string
	ProtocolVersion         string
	DisableDirectoryListing bool
	EnableMarkdown          bool
	TLS                     *TLSConfiguration
}

// FileServer serves files over HTTP or HTTPS.
type FileServer struct {
	logger                  *zap.Logger
	servingAddressFormatter serverdetails.ServingAddressFormatter
}

// NewFileServer constructs a FileServer.
func NewFileServer(logger *zap.Logger, servingAddressFormatter serverdetails.ServingAddressFormatter) FileServer {
	return FileServer{logger: logger, servingAddressFormatter: servingAddressFormatter}
}

// Serve runs the HTTP server until the context is cancelled or an error occurs.
func (fileServer FileServer) Serve(ctx context.Context, configuration FileServerConfiguration) error {
	listeningAddress := net.JoinHostPort(configuration.BindAddress, configuration.Port)
	displayAddress := fileServer.servingAddressFormatter.FormatHostAndPortForLogging(configuration.BindAddress, configuration.Port)
	fileHandler := fileServer.buildFileHandler(configuration)
	wrappedHandler := fileServer.wrapWithHeaders(fileHandler, configuration.ProtocolVersion)
	loggingHandler := fileServer.wrapWithLogging(wrappedHandler)

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
	if certificateConfigured {
		fileServer.logger.Info(logMessageServingHTTPS, zap.String(logFieldDirectory, configuration.DirectoryPath), zap.String(logFieldProtocol, configuration.ProtocolVersion), zap.String(logFieldURL, fmt.Sprintf("https://%s", displayAddress)), zap.String(logFieldTimestamp, currentTime))
	} else {
		fileServer.logger.Info(logMessageServingHTTP, zap.String(logFieldDirectory, configuration.DirectoryPath), zap.String(logFieldProtocol, configuration.ProtocolVersion), zap.String(logFieldURL, fmt.Sprintf("http://%s", displayAddress)), zap.String(logFieldTimestamp, currentTime))
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
		fileServer.logger.Info(logMessageShutdownInitiated)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer cancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			fileServer.logger.Error(logMessageShutdownFailed, zap.Error(shutdownErr))
			return fmt.Errorf("shutdown server: %w", shutdownErr)
		}
		fileServer.logger.Info(logMessageShutdownCompleted)
		return nil
	case serveErr := <-serverErrors:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			fileServer.logger.Error(logMessageServerError, zap.Error(serveErr))
			return fmt.Errorf("serve http: %w", serveErr)
		}
		return nil
	}
}

func (fileServer FileServer) buildFileHandler(configuration FileServerConfiguration) http.Handler {
	fileSystem := http.Dir(configuration.DirectoryPath)
	baseHandler := http.FileServer(fileSystem)
	if configuration.EnableMarkdown {
		baseHandler = newMarkdownHandler(baseHandler, fileSystem, configuration.DisableDirectoryListing)
	} else if configuration.DisableDirectoryListing {
		baseHandler = newDirectoryGuardHandler(baseHandler, fileSystem)
	}
	return baseHandler
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

func (fileServer FileServer) wrapWithLogging(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		recordedWriter := &statusRecorder{ResponseWriter: responseWriter, statusCode: http.StatusOK}
		startTime := time.Now()
		fileServer.logger.Info(logMessageRequestStarted, zap.String(logFieldMethod, request.Method), zap.String(logFieldPath, request.URL.Path), zap.String(logFieldProtocol, request.Proto), zap.String(logFieldRemote, request.RemoteAddr))
		handler.ServeHTTP(recordedWriter, request)
		duration := time.Since(startTime)
		fileServer.logger.Info(logMessageRequestCompleted, zap.String(logFieldMethod, request.Method), zap.String(logFieldPath, request.URL.Path), zap.Int(logFieldStatus, recordedWriter.statusCode), zap.Duration(logFieldDuration, duration), zap.String(logFieldRemote, request.RemoteAddr))
	})
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
	statusCode int
}

func (recorder *statusRecorder) WriteHeader(statusCode int) {
	recorder.statusCode = statusCode
	recorder.ResponseWriter.WriteHeader(statusCode)
}
