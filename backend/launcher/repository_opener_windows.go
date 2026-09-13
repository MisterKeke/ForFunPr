//go:build windows

package launcher

import (
	"errors"
	"os/exec"

	"something/backend/fileexplorer"
)

type windowsRepositoryOpener struct{}

var startRepositoryEditor = func(executablePath string, repositoryPath string) error {
	return exec.Command(executablePath, repositoryPath).Start()
}

func NewRepositoryOpener() RepositoryOpener {
	return windowsRepositoryOpener{}
}

func (windowsRepositoryOpener) Supported() bool { return true }

func (windowsRepositoryOpener) OpenFolder(path string) error {
	if path == "" {
		return errors.New("repository folder is unavailable")
	}
	if err := fileexplorer.OpenPath(path, path); err != nil {
		return errors.New("Windows could not open the repository folder")
	}
	return nil
}

func (windowsRepositoryOpener) OpenEditor(executablePath string, repositoryPath string) error {
	if executablePath == "" || repositoryPath == "" {
		return errors.New("the configured editor is unavailable")
	}
	if err := startRepositoryEditor(executablePath, repositoryPath); err != nil {
		return errors.New("the configured editor could not be opened")
	}
	return nil
}
