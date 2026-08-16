//go:build windows

package launcher

import (
	"path/filepath"

	"something/backend/fileexplorer"
)

type windowsDesktopAppLauncher struct{}

func New() Launcher {
	return windowsDesktopAppLauncher{}
}

func (windowsDesktopAppLauncher) Launch(path string) error {
	return fileexplorer.OpenPath(path, filepath.Dir(path))
}
