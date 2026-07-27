package mcpserver

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
)

const (
	stateOff      = "off"
	stateStarting = "starting"
	stateOn       = "on"
	stateStopping = "stopping"
	stateError    = "error"
)

var ErrStateTransition = errors.New("MCP server state is changing")

// Controller owns the restartable runtime lifecycle for the MCP listener.
type Controller struct {
	mu         sync.Mutex
	address    string
	apiURL     string
	server     *Server
	state      string
	lastError  string
	generation uint64
}

// NewController creates an off controller for a validated loopback address.
func NewController(address string) *Controller {
	return &Controller{
		address: loopbackAddress(address),
		state:   stateOff,
	}
}

// ConfigureAPI should be called after the REST listener has been bound and its
// actual address is known.
func (c *Controller) ConfigureAPI(apiURL string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state == stateStarting ||
		c.state == stateOn ||
		c.state == stateStopping {
		return errors.New("cannot change the API address while MCP is active")
	}

	apiURL = strings.TrimRight(strings.TrimSpace(apiURL), "/")
	if apiURL == "" {
		return errors.New("desktop API address is required")
	}

	c.apiURL = apiURL
	return nil
}

// Snapshot returns all fields under the same lock so the UI never receives a
// mixture of states from different moments.
func (c *Controller) Snapshot() (
	state string,
	address string,
	lastError string,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.state, c.address, c.lastError
}

// Start binds and serves a fresh MCP server instance.
func (c *Controller) Start() error {
	c.mu.Lock()

	switch c.state {
	case stateOn:
		c.mu.Unlock()
		return nil
	case stateStarting, stateStopping:
		state := c.state
		c.mu.Unlock()
		return fmt.Errorf("%w: currently %s", ErrStateTransition, state)
	}

	if c.apiURL == "" {
		c.state = stateError
		c.lastError = "The desktop API is not available."
		c.mu.Unlock()
		return errors.New("desktop API address is not configured")
	}

	address := c.address
	apiURL := c.apiURL
	c.state = stateStarting
	c.lastError = ""
	c.mu.Unlock()

	// This must be a fresh instance. A previously shut-down http.Server cannot
	// be started again.
	server := NewServer(address, apiURL)
	listener, err := server.Listen()
	if err != nil {
		c.mu.Lock()
		c.state = stateError
		c.lastError = "MCP could not listen on " + address + "."
		c.mu.Unlock()
		return fmt.Errorf("bind MCP server: %w", err)
	}

	c.mu.Lock()
	c.generation++
	generation := c.generation
	c.server = server
	c.state = stateOn
	c.mu.Unlock()

	go c.serve(generation, server, listener)
	return nil
}

func (c *Controller) serve(
	generation uint64,
	server *Server,
	listener net.Listener,
) {
	err := server.Serve(listener)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Stop increments generation and takes responsibility for the final state.
	if generation != c.generation || c.server != server {
		return
	}

	c.server = nil
	if err != nil {
		c.state = stateError
		c.lastError = "The MCP server stopped unexpectedly."
		return
	}

	c.state = stateOff
}

// Stop gracefully shuts down the active MCP server, if any.
func (c *Controller) Stop() error {
	c.mu.Lock()

	if c.state == stateOff {
		c.mu.Unlock()
		return nil
	}

	if c.state == stateStarting || c.state == stateStopping {
		state := c.state
		c.mu.Unlock()
		return fmt.Errorf("%w: currently %s", ErrStateTransition, state)
	}

	server := c.server
	if server == nil {
		c.state = stateOff
		c.lastError = ""
		c.mu.Unlock()
		return nil
	}

	c.state = stateStopping

	// Invalidate the running serve callback. Stop will set the final state.
	c.generation++
	c.mu.Unlock()

	err := server.Shutdown()

	c.mu.Lock()
	if c.server == server {
		c.server = nil
	}
	if err != nil {
		c.state = stateError
		c.lastError = "MCP did not stop cleanly."
	} else {
		c.state = stateOff
		c.lastError = ""
	}
	c.mu.Unlock()

	return err
}
