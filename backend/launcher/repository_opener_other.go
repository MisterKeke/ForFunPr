//go:build !windows

package launcher

import "errors"

type unsupportedRepositoryOpener struct{}

func NewRepositoryOpener() RepositoryOpener {
	return unsupportedRepositoryOpener{}
}

func (unsupportedRepositoryOpener) Supported() bool { return false }

func (unsupportedRepositoryOpener) OpenFolder(string) error {
	return errors.New("opening repository folders is supported only on Windows")
}

func (unsupportedRepositoryOpener) OpenEditor(string, string) error {
	return errors.New("opening repositories in an editor is supported only on Windows")
}
