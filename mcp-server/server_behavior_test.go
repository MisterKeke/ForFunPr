package mcpserver

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"something/internal/policy"
)

func TestSecureRequestHandlerEnforcesOriginTokenAndLoopbackPolicy(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		authorize  string
		origin     string
		remoteAddr string
		wantStatus int
		wantAuth   bool
	}{
		{name: "loopback IPv4 without configured token", remoteAddr: "127.0.0.1:4567", wantStatus: http.StatusNoContent},
		{name: "loopback IPv6 without configured token", remoteAddr: "[::1]:4567", wantStatus: http.StatusNoContent},
		{name: "non-loopback without configured token", remoteAddr: "192.0.2.10:4567", wantStatus: http.StatusForbidden},
		{name: "malformed remote address", remoteAddr: "not-an-address", wantStatus: http.StatusForbidden},
		{name: "valid bearer token", token: "secret", authorize: "Bearer secret", remoteAddr: "192.0.2.10:4567", wantStatus: http.StatusNoContent},
		{name: "wrong bearer token", token: "secret", authorize: "Bearer wrong", remoteAddr: "127.0.0.1:4567", wantStatus: http.StatusUnauthorized, wantAuth: true},
		{name: "browser origin is always rejected", token: "secret", authorize: "Bearer secret", origin: "https://example.com", remoteAddr: "127.0.0.1:4567", wantStatus: http.StatusForbidden},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			handler := secureRequestHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}), test.token)
			request := httptest.NewRequest(http.MethodGet, "/mcp", nil)
			request.RemoteAddr = test.remoteAddr
			request.Header.Set("Authorization", test.authorize)
			request.Header.Set("Origin", test.origin)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if called != (test.wantStatus == http.StatusNoContent) {
				t.Fatalf("downstream called = %v", called)
			}
			if got := response.Header().Get("WWW-Authenticate"); (got == "Bearer") != test.wantAuth {
				t.Fatalf("WWW-Authenticate = %q", got)
			}
		})
	}
}

func TestLimitRequestBodyAcceptsExactBoundaryAndRestoresBody(t *testing.T) {
	payload := bytes.Repeat([]byte{'x'}, maximumRequestBytes)
	request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(payload))
	response := httptest.NewRecorder()
	if !limitRequestBody(response, request) {
		t.Fatalf("exact-limit request rejected: %d %s", response.Code, response.Body.String())
	}
	readBack, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readBack, payload) || request.ContentLength != int64(len(payload)) {
		t.Fatalf("restored body length = %d, content length = %d", len(readBack), request.ContentLength)
	}
}

func TestLimitRequestBodyRejectsDeclaredStreamedAndReadFailures(t *testing.T) {
	t.Run("declared oversize", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("small"))
		request.ContentLength = maximumRequestBytes + 1
		response := httptest.NewRecorder()
		if limitRequestBody(response, request) || response.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("response = %d %s", response.Code, response.Body.String())
		}
	})

	t.Run("streamed oversize", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(bytes.Repeat([]byte{'x'}, maximumRequestBytes+1)))
		request.ContentLength = -1
		response := httptest.NewRecorder()
		if limitRequestBody(response, request) || response.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("response = %d %s", response.Code, response.Body.String())
		}
	})

	t.Run("read failure", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		request.Body = failingReadCloser{}
		request.ContentLength = -1
		response := httptest.NewRecorder()
		if limitRequestBody(response, request) || response.Code != http.StatusBadRequest {
			t.Fatalf("response = %d %s", response.Code, response.Body.String())
		}
	})

	t.Run("non-POST bypass", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		request.ContentLength = maximumRequestBytes + 1
		if !limitRequestBody(httptest.NewRecorder(), request) {
			t.Fatal("non-POST request was body-limited")
		}
	})
}

type failingReadCloser struct{}

func (failingReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (failingReadCloser) Close() error             { return nil }

func TestRequestTimeoutHandlerAddsBoundedContext(t *testing.T) {
	var remaining time.Duration
	handler := requestTimeoutHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("request context has no deadline")
		}
		remaining = time.Until(deadline)
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if response.Code != http.StatusNoContent || remaining <= 0 || remaining > policy.MCPRequestTimeout {
		t.Fatalf("status = %d, remaining timeout = %s", response.Code, remaining)
	}
}

func TestBearerAndLoopbackHelpers(t *testing.T) {
	if !validBearerToken("Bearer value", "value") || validBearerToken("bearer value", "value") || validBearerToken("Bearer value ", "value") {
		t.Fatal("bearer token comparison accepted an inexact header")
	}
	for _, remote := range []string{"127.0.0.1:1", "[::1]:1"} {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = remote
		if !requestIsLoopback(request) {
			t.Fatalf("loopback address rejected: %s", remote)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.1:1"
	if requestIsLoopback(request) {
		t.Fatal("non-loopback address accepted")
	}
}

func TestServerNilLifecycleIsSafe(t *testing.T) {
	var server *Server
	if _, err := server.Listen(); err == nil {
		t.Fatal("nil server Listen succeeded")
	}
	if err := server.Serve(nil); err == nil {
		t.Fatal("nil server Serve succeeded")
	}
	if err := server.Start(); err == nil {
		t.Fatal("nil server Start succeeded")
	}
	if err := server.ShutdownContext(context.Background()); err != nil {
		t.Fatalf("nil server shutdown = %v", err)
	}
}
