package api

import (
	"context"
	"testing"
)

func TestServerReportsLoopbackPortConflictAtBindTime(t *testing.T) {
	first := NewServer("127.0.0.1:0", nil)
	listener, err := first.Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer first.Shutdown()

	second := NewServer(listener.Addr().String(), nil)
	if _, err := second.Listen(); err == nil {
		_ = second.Shutdown()
		t.Fatal("second listener unexpectedly acquired an occupied API address")
	}
}

func TestServerRestrictsListenerAddressToLoopback(t *testing.T) {
	for raw, want := range map[string]string{
		"":                    defaultAddress,
		"0.0.0.0:9000":        defaultAddress,
		"192.0.2.1:9000":      defaultAddress,
		"localhost:9000":      defaultAddress,
		"missing-port":        defaultAddress,
		"127.0.0.1:0":         "127.0.0.1:0",
		"[::1]:0":             "[::1]:0",
		"[0:0:0:0:0:0:0:1]:0": "[0:0:0:0:0:0:0:1]:0",
	} {
		t.Run(raw, func(t *testing.T) {
			if got := NewServer(raw, nil).httpServer.Addr; got != want {
				t.Fatalf("server address = %q, want %q", got, want)
			}
		})
	}
}

func TestServerListenAndShutdownAreIdempotent(t *testing.T) {
	server := NewServer("127.0.0.1:0", nil)
	first, err := server.Listen()
	if err != nil {
		t.Fatal(err)
	}
	second, err := server.Listen()
	if err != nil || second != first {
		t.Fatalf("second Listen = %#v, %v", second, err)
	}
	if err := server.ShutdownContext(nil); err != nil {
		t.Fatal(err)
	}
	if server.listener != nil {
		t.Fatal("shutdown retained listener")
	}
	if err := server.Shutdown(); err != nil {
		t.Fatalf("idempotent Shutdown = %v", err)
	}
}

func TestNilServerLifecycleIsSafe(t *testing.T) {
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
