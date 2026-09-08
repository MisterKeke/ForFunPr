//go:build !windows

package runningapps

import (
	"context"
	"errors"
	"testing"
)

func TestUnsupportedProviderReportsUnsupported(t *testing.T) {
	provider := New()
	if provider.Supported() {
		t.Fatal("non-Windows provider reported support")
	}
	if _, err := provider.List(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("List error = %v, want ErrUnsupported", err)
	}
	if err := provider.Activate(context.Background(), "1"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Activate error = %v, want ErrUnsupported", err)
	}
}
