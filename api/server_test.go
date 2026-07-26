package api

import "testing"

func TestServerReportsLoopbackPortConflictAtBindTime(t *testing.T) {
	first := NewServer("127.0.0.1:0", nil)
	listener, err := first.Listen()
	if err != nil { t.Fatal(err) }
	defer first.Shutdown()

	second := NewServer(listener.Addr().String(), nil)
	if _, err := second.Listen(); err == nil {
		_ = second.Shutdown()
		t.Fatal("second listener unexpectedly acquired an occupied API address")
	}
}
