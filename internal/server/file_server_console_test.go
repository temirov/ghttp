package server

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestFormatConsoleStartMessage(t *testing.T) {
	configuration := FileServerConfiguration{
		BindAddress: "",
		Port:        "8000",
	}
	message := formatConsoleStartMessage(configuration, false, "localhost:8000")
	expected := "Serving HTTP on 0.0.0.0 port 8000 (http://localhost:8000/) ..."
	if message != expected {
		t.Fatalf("expected %s, got %s", expected, message)
	}

	configuration.BindAddress = "127.0.0.1"
	configuration.Port = "8443"
	message = formatConsoleStartMessage(configuration, true, "127.0.0.1:8443")
	expected = "Serving HTTPS on 127.0.0.1 port 8443 (https://127.0.0.1:8443/) ..."
	if message != expected {
		t.Fatalf("expected %s, got %s", expected, message)
	}
}

func TestFormatConsoleRequestLog(t *testing.T) {
	request := httptestNewRequest("GET", "/docs/index.html?version=1", "HTTP/1.1", "127.0.0.1:54321")
	startTime := time.Date(2025, time.October, 8, 12, 30, 0, 0, time.UTC)
	message := formatConsoleRequestLog(request, http.StatusOK, 512, startTime)
	expected := "127.0.0.1 - - [08/Oct/2025 12:30:00] \"GET /docs/index.html?version=1 HTTP/1.1\" 200 512"
	if message != expected {
		t.Fatalf("expected %s, got %s", expected, message)
	}

	request = httptestNewRequest("POST", "/submit", "HTTP/2", "::1")
	startTime = time.Date(2025, time.October, 8, 12, 45, 0, 0, time.UTC)
	message = formatConsoleRequestLog(request, http.StatusCreated, 0, startTime)
	expected = "::1 - - [08/Oct/2025 12:45:00] \"POST /submit HTTP/2\" 201 -"
	if message != expected {
		t.Fatalf("expected %s, got %s", expected, message)
	}
}

func httptestNewRequest(method, target, proto, remoteAddr string) *http.Request {
	requestURL, _ := url.Parse(target)
	request := &http.Request{
		Method:     method,
		Proto:      proto,
		URL:        requestURL,
		RemoteAddr: remoteAddr,
	}
	if request.URL != nil {
		request.RequestURI = request.URL.RequestURI()
	}
	if request.RequestURI == "" {
		request.RequestURI = target
	}
	return request
}
