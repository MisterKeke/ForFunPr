//go:build windows

package clipboard

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"
)

func TestClipboardProcessMemoryRoundTrip(t *testing.T) {
	for _, value := range []string{"", "plain text", "İstanbul 日本語 🚀"} {
		t.Run(value, func(t *testing.T) {
			encoded, err := encodeClipboardText(value)
			if err != nil {
				t.Fatal(err)
			}
			destination := make([]uint16, len(encoded))
			pointer := uintptr(unsafe.Pointer(&destination[0]))
			if err := writeClipboardMemory(pointer, encoded); err != nil {
				t.Fatal(err)
			}
			readBack, err := readClipboardMemory(pointer, uintptr(len(destination)*2))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(readBack, encoded) {
				t.Fatalf("read memory = %#v, want %#v", readBack, encoded)
			}
			if got := decodeClipboardText(readBack); got != value {
				t.Fatalf("decoded clipboard text = %q, want %q", got, value)
			}
		})
	}
}

func TestClipboardMemoryHelpersRejectInvalidInputs(t *testing.T) {
	if _, err := encodeClipboardText("before\x00after"); err == nil {
		t.Fatal("embedded NUL was accepted")
	}
	if err := writeClipboardMemory(0, []uint16{0}); err == nil {
		t.Fatal("zero write pointer was accepted")
	}
	if err := writeClipboardMemory(1, nil); err == nil {
		t.Fatal("empty write was accepted")
	}
	if _, err := readClipboardMemory(0, 2); err == nil {
		t.Fatal("zero read pointer was accepted")
	}
	if _, err := readClipboardMemory(1, 1); err == nil {
		t.Fatal("undersized read was accepted")
	}
	if got := decodeClipboardText([]uint16{'a', 'b', 0, 'c'}); got != "ab" {
		t.Fatalf("NUL-terminated decode = %q", got)
	}
}

func TestOpenClipboardRetryStopsOnSuccess(t *testing.T) {
	originalAttempt := openClipboardAttempt
	originalSleep := clipboardRetrySleep
	t.Cleanup(func() {
		openClipboardAttempt = originalAttempt
		clipboardRetrySleep = originalSleep
	})

	attempts := 0
	sleeps := 0
	openClipboardAttempt = func() (uintptr, error) {
		attempts++
		if attempts == 3 {
			return 1, nil
		}
		return 0, errors.New("busy")
	}
	clipboardRetrySleep = func(time.Duration) { sleeps++ }

	if err := openClipboardWithRetry(); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 || sleeps != 2 {
		t.Fatalf("attempts = %d, sleeps = %d", attempts, sleeps)
	}
}

func TestOpenClipboardRetryReturnsLastFailure(t *testing.T) {
	originalAttempt := openClipboardAttempt
	originalSleep := clipboardRetrySleep
	t.Cleanup(func() {
		openClipboardAttempt = originalAttempt
		clipboardRetrySleep = originalSleep
	})

	attempts := 0
	sleeps := 0
	openClipboardAttempt = func() (uintptr, error) {
		attempts++
		return 0, errors.New("clipboard busy")
	}
	clipboardRetrySleep = func(time.Duration) { sleeps++ }

	err := openClipboardWithRetry()
	if err == nil || !strings.Contains(err.Error(), "clipboard busy") {
		t.Fatalf("retry error = %v", err)
	}
	if attempts != 8 || sleeps != 7 {
		t.Fatalf("attempts = %d, sleeps = %d", attempts, sleeps)
	}
}

func TestWindowsControllerLifecycleIsIdempotent(t *testing.T) {
	controller := &windowsController{}
	if err := controller.Start(context.Background(), nil); err == nil {
		t.Fatal("nil callback was accepted")
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := controller.Start(ctx, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if !controller.Running() {
		t.Fatal("controller did not report running after Start")
	}
	if err := controller.Start(ctx, func(string) {}); err != nil {
		t.Fatalf("idempotent Start = %v", err)
	}
	cancel()
	controller.Stop()
	controller.Stop()
	if controller.Running() {
		t.Fatal("controller remained running after Stop")
	}
}
