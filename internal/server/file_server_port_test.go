package server

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/temirov/ghttp/internal/serverdetails"
	"github.com/temirov/ghttp/pkg/logging"
)

func TestServeReturnsFriendlyErrorWhenPortInUse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	tcpAddr := listener.Addr().(*net.TCPAddr)
	portString := strconv.Itoa(tcpAddr.Port)

	fileServer := NewFileServer(logging.NewTestService(logging.TypeConsole), serverdetails.NewServingAddressFormatter())
	configuration := FileServerConfiguration{
		BindAddress:             "127.0.0.1",
		Port:                    portString,
		DirectoryPath:           t.TempDir(),
		ProtocolVersion:         "HTTP/1.1",
		DisableDirectoryListing: false,
		EnableMarkdown:          true,
		LoggingType:             "CONSOLE",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = fileServer.Serve(ctx, configuration)
	if err == nil {
		t.Fatalf("expected error when port is in use")
	}
	if !strings.Contains(err.Error(), "address in use") {
		t.Fatalf("expected address in use error, got %v", err)
	}

	listener.Close()
}
