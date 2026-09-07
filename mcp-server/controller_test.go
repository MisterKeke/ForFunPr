package mcpserver

import (
	"errors"
	"net"
	"strings"
	"testing"
)

func TestControllerRequiresAPIConfiguration(t *testing.T) {
	controller := NewController("0.0.0.0:9000")
	state, address, lastError := controller.Snapshot()
	if state != stateOff || address != defaultAddress || lastError != "" {
		t.Fatalf("initial snapshot = %q, %q, %q", state, address, lastError)
	}
	if err := controller.Start(); err == nil {
		t.Fatal("controller started without an API address")
	}
	state, _, lastError = controller.Snapshot()
	if state != stateError || lastError == "" {
		t.Fatalf("failed-start snapshot = %q, %q", state, lastError)
	}
	if err := controller.ConfigureAPI("   "); err == nil {
		t.Fatal("blank API address was accepted")
	}
}

func TestControllerStartsStopsAndRestartsFreshServer(t *testing.T) {
	controller := NewController("127.0.0.1:0")
	if err := controller.ConfigureAPI(" http://127.0.0.1:8080/ "); err != nil {
		t.Fatal(err)
	}
	if controller.apiURL != "http://127.0.0.1:8080" {
		t.Fatalf("configured API URL = %q", controller.apiURL)
	}

	if err := controller.Start(); err != nil {
		t.Fatal(err)
	}
	first := controller.server
	state, _, lastError := controller.Snapshot()
	if state != stateOn || first == nil || lastError != "" {
		t.Fatalf("running snapshot = %q, %#v, %q", state, first, lastError)
	}
	if err := controller.ConfigureAPI("http://127.0.0.1:8082"); err == nil {
		t.Fatal("active controller accepted API reconfiguration")
	}
	if err := controller.Start(); err != nil {
		t.Fatalf("idempotent Start = %v", err)
	}
	if controller.server != first {
		t.Fatal("idempotent Start replaced the running server")
	}

	if err := controller.Stop(); err != nil {
		t.Fatal(err)
	}
	if state, _, lastError = controller.Snapshot(); state != stateOff || lastError != "" {
		t.Fatalf("stopped snapshot = %q, %q", state, lastError)
	}
	if err := controller.Stop(); err != nil {
		t.Fatalf("idempotent Stop = %v", err)
	}

	if err := controller.Start(); err != nil {
		t.Fatal(err)
	}
	if controller.server == nil || controller.server == first {
		t.Fatal("restart did not create a fresh HTTP server")
	}
	if err := controller.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestControllerReportsBindFailureWithoutLeakingDetails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	controller := NewController(listener.Addr().String())
	if err := controller.ConfigureAPI("http://127.0.0.1:8080"); err != nil {
		t.Fatal(err)
	}
	err = controller.Start()
	if err == nil || !strings.Contains(err.Error(), "bind MCP server") {
		t.Fatalf("bind error = %v", err)
	}
	state, _, lastError := controller.Snapshot()
	if state != stateError || lastError == "" || strings.Contains(lastError, err.Error()) {
		t.Fatalf("bind-failure snapshot = %q, %q", state, lastError)
	}
}

func TestControllerRejectsTransitionStates(t *testing.T) {
	controller := NewController("127.0.0.1:0")
	for _, state := range []string{stateStarting, stateStopping} {
		controller.state = state
		if err := controller.Start(); !errors.Is(err, ErrStateTransition) {
			t.Fatalf("Start in %s = %v", state, err)
		}
		if err := controller.Stop(); !errors.Is(err, ErrStateTransition) {
			t.Fatalf("Stop in %s = %v", state, err)
		}
		if err := controller.ConfigureAPI("http://127.0.0.1:8080"); err == nil {
			t.Fatalf("ConfigureAPI in %s succeeded", state)
		}
	}
}
