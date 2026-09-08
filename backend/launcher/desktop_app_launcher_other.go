//go:build !windows

package launcher

import "errors"

type unsupportedDesktopAppLauncher struct{}

func New() Launcher {
	return unsupportedDesktopAppLauncher{}
}

func (unsupportedDesktopAppLauncher) Supported() bool { return false }

func (unsupportedDesktopAppLauncher) Launch(string) error {
	return errors.New("opening Windows applications is supported only on Windows")
}
