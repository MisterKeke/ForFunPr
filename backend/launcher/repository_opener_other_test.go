//go:build !windows

package launcher

import "testing"

func TestRepositoryOpenerIsUnsupportedOutsideWindows(t *testing.T) {
	if NewRepositoryOpener().Supported() {
		t.Fatal("repository opener unexpectedly reports support outside Windows")
	}
}
