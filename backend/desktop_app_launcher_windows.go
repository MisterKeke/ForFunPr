//go:build windows

package backend

import "path/filepath"

type windowsDesktopAppLauncher struct{}

func newDesktopAppLauncher() desktopAppLauncher {
	return windowsDesktopAppLauncher{}
}

func (windowsDesktopAppLauncher) Launch(path string) error {
	return openWindowsPath(path, filepath.Dir(path))
}
