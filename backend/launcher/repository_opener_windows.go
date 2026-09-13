//go:build windows

package launcher

import (
	"context"
	"errors"
	"os/exec"

	"something/backend/fileexplorer"
)

type windowsRepositoryOpener struct{}

var startRepositoryEditor = func(ctx context.Context, executablePath string, repositoryPath string) error {
	return exec.CommandContext(ctx, executablePath, repositoryPath).Start()
}

func NewRepositoryOpener() RepositoryOpener {
	return windowsRepositoryOpener{}
}

func (windowsRepositoryOpener) Supported() bool { return true }

func (windowsRepositoryOpener) OpenFolder(ctx context.Context, path string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if path == "" {
		return errors.New("repository folder is unavailable")
	}
	if err := fileexplorer.OpenPath(path, path); err != nil {
		return errors.New("Windows could not open the repository folder")
	}
	return nil
}

func (windowsRepositoryOpener) OpenEditor(ctx context.Context, executablePath string, repositoryPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if executablePath == "" || repositoryPath == "" {
		return errors.New("the configured editor is unavailable")
	}
	// GUI editors are intentionally detached from the short-lived Wails call
	// context after the pre-launch cancellation check.
	if err := startRepositoryEditor(context.WithoutCancel(ctx), executablePath, repositoryPath); err != nil {
		return errors.New("the configured editor could not be opened")
	}
	return nil
}
