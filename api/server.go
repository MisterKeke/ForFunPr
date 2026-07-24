package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"currency-wails/backend"
)

const defaultAddress = "127.0.0.1:8080"

// Server hosts the HTTP API alongside the Wails application.
type Server struct {
	httpServer *http.Server
}

// NewServer creates an API server that uses the same backend application
// instance as the Wails frontend.
func NewServer(address string, app *backend.App) *Server {
	if address == "" {
		address = defaultAddress
	}

	return &Server{
		httpServer: &http.Server{
			Addr:              address,
			Handler:           newRouter(app),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      35 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

// Start blocks until the server is stopped or fails. Call it in a goroutine
// when starting it from the Wails startup callback.
func (s *Server) Start() error {
	if s == nil || s.httpServer == nil {
		return errors.New("API server is not initialized")
	}

	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown gracefully stops the server with a bounded timeout. It should run
// before the shared backend application closes its database connection.
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	return s.httpServer.Shutdown(ctx)
}
