package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"currency-wails/backend"
	"currency-wails/internal/policy"
)

const defaultAddress = "127.0.0.1:8080"

// Server hosts the HTTP API alongside the Wails application.
type Server struct {
	httpServer *http.Server
	mu         sync.Mutex
	listener   net.Listener
}

// NewServer creates an API server that uses the same backend application
// instance as the Wails frontend.
func NewServer(address string, app *backend.Service) *Server {
	if address == "" {
		address = defaultAddress
	}
	address = loopbackAddress(address)

	return &Server{
		httpServer: &http.Server{
			Addr:              address,
			Handler:           newRouter(app),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      policy.APIWriteTimeout,
			IdleTimeout:       60 * time.Second,
		},
	}
}

func loopbackAddress(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil { return defaultAddress }
	parsed := net.ParseIP(host)
	if parsed == nil || !parsed.IsLoopback() { return defaultAddress }
	return address
}

// Listen binds the configured loopback address synchronously so startup can
// fail before any dependent MCP listener is made available.
func (s *Server) Listen() (net.Listener, error) {
	if s == nil || s.httpServer == nil { return nil, errors.New("API server is not initialized") }
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil { return s.listener, nil }
	listener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil { return nil, err }
	s.listener = listener
	return listener, nil
}

func (s *Server) Serve(listener net.Listener) error {
	if s == nil || s.httpServer == nil || listener == nil { return errors.New("API server is not initialized") }
	err := s.httpServer.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) { return nil }
	return err
}

// Start blocks until the server is stopped or fails. Call it in a goroutine
// when starting it from the Wails startup callback.
func (s *Server) Start() error {
	if s == nil || s.httpServer == nil {
		return errors.New("API server is not initialized")
	}

	listener, err := s.Listen()
	if err != nil { return err }
	return s.Serve(listener)
}

// Shutdown gracefully stops the server with a bounded timeout. It should run
// before the shared backend application closes its database connection.
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), policy.GracefulShutdownTimeout)
	defer cancel()

	return s.ShutdownContext(ctx)
}

// ShutdownContext gracefully stops the server using the caller's context.
func (s *Server) ShutdownContext(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	err := s.httpServer.Shutdown(ctx)
	s.mu.Lock()
	listener := s.listener
	s.listener = nil
	s.mu.Unlock()
	if listener != nil { _ = listener.Close() }
	return err
}
