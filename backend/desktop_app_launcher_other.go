//go:build !windows

package backend

import "errors"

type unsupportedDesktopAppLauncher struct{}

func newDesktopAppLauncher() desktopAppLauncher {
	return unsupportedDesktopAppLauncher{}
}

func (unsupportedDesktopAppLauncher) Launch(string) error {
	return errors.New("opening Windows applications is supported only on Windows")
}
