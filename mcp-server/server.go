// Package mcpserver hosts the Something MCP protocol server over Streamable
// HTTP alongside the desktop application.
package mcpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"currency-wails/mcp-server/tools"
	readtools "currency-wails/mcp-server/tools/read"
	writetools "currency-wails/mcp-server/tools/write"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultAddress        = "127.0.0.1:8081"
	maximumRequestBytes   = 1 << 20
	gracefulShutdownDelay = 5 * time.Second
)

const serverInstructions = "Use read tools before mutation tools when practical. Task dates use YYYY-MM-DD. Mutations affect the user's running Something desktop application. Call destructive tools only when the user clearly requests that destructive action."

// Server owns the Streamable HTTP MCP listener.
type Server struct {
	httpServer *http.Server
}

// NewServer creates a stateless JSON-response MCP server. Invalid or
// non-loopback addresses are replaced with the safe default address.
func NewServer(address string) *Server {
	address = loopbackAddress(address)
	logger := slog.Default()

	protocolServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "something",
			Version: "1.0.0",
		},
		&mcp.ServerOptions{
			Instructions: serverInstructions,
			Logger:       logger,
			Capabilities: &mcp.ServerCapabilities{},
		},
	)
	runner := tools.NewRunner(logger)
	readtools.Register(protocolServer, runner)
	writetools.Register(protocolServer, runner)

	protocolHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server {
			return protocolServer
		},
		&mcp.StreamableHTTPOptions{
			Stateless:    true,
			JSONResponse: true,
			Logger:       logger,
		},
	)

	mux := http.NewServeMux()
	mux.Handle(
		"/mcp",
		secureRequestHandler(
			protocolHandler,
			os.Getenv("SOMETHING_MCP_TOKEN"),
		),
	)

	return &Server{
		httpServer: &http.Server{
			Addr:              address,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

// Start blocks until the MCP server is stopped or fails.
func (server *Server) Start() error {
	if server == nil || server.httpServer == nil {
		return errors.New("MCP server is not initialized")
	}

	err := server.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown gracefully stops the MCP server with a bounded timeout.
func (server *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		gracefulShutdownDelay,
	)
	defer cancel()

	return server.ShutdownContext(ctx)
}

// ShutdownContext gracefully stops the MCP server using the caller's context.
func (server *Server) ShutdownContext(ctx context.Context) error {
	if server == nil || server.httpServer == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return server.httpServer.Shutdown(ctx)
}

func loopbackAddress(address string) string {
	if address == "" {
		return defaultAddress
	}

	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" {
		return defaultAddress
	}
	return address
}

func secureRequestHandler(next http.Handler, bearerToken string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" {
			http.Error(
				w,
				"Browser-originated requests are not allowed.",
				http.StatusForbidden,
			)
			return
		}

		if bearerToken != "" {
			if !validBearerToken(
				r.Header.Get("Authorization"),
				bearerToken,
			) {
				w.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(w, "Unauthorized.", http.StatusUnauthorized)
				return
			}
		} else if !requestIsLoopback(r) {
			http.Error(w, "Loopback access is required.", http.StatusForbidden)
			return
		}

		if !limitRequestBody(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func validBearerToken(header string, token string) bool {
	provided := sha256.Sum256([]byte(header))
	expected := sha256.Sum256([]byte("Bearer " + token))
	return subtle.ConstantTimeCompare(provided[:], expected[:]) == 1
}

func requestIsLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func limitRequestBody(w http.ResponseWriter, r *http.Request) bool {
	if r.Body == nil || r.Method != http.MethodPost {
		return true
	}
	if r.ContentLength > maximumRequestBytes {
		http.Error(w, "Request body is too large.", http.StatusRequestEntityTooLarge)
		return false
	}

	body, err := io.ReadAll(io.LimitReader(
		r.Body,
		maximumRequestBytes+1,
	))
	if err != nil {
		http.Error(w, "Request body could not be read.", http.StatusBadRequest)
		return false
	}
	if len(body) > maximumRequestBytes {
		http.Error(w, "Request body is too large.", http.StatusRequestEntityTooLarge)
		return false
	}

	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	return true
}
