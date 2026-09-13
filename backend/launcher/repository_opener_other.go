//go:build !windows

package launcher

import (
	"context"
	"errors"
)

type unsupportedRepositoryOpener struct{}

func NewRepositoryOpener() RepositoryOpener {
	return unsupportedRepositoryOpener{}
}

func (unsupportedRepositoryOpener) Supported() bool { return false }

func (unsupportedRepositoryOpener) OpenFolder(context.Context, string) error {
	return errors.New("opening repository folders is supported only on Windows")
}

func (unsupportedRepositoryOpener) OpenEditor(context.Context, string, string) error {
	return errors.New("opening repositories in an editor is supported only on Windows")
}
