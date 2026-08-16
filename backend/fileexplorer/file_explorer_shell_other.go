//go:build !windows

package fileexplorer

import "errors"

type unsupportedFileExplorerShell struct{}

func newFileExplorerShell() fileExplorerShell {
	return unsupportedFileExplorerShell{}
}

func (unsupportedFileExplorerShell) OpenFile(string) error {
	return errors.New("opening files is supported only on Windows")
}

func (unsupportedFileExplorerShell) RecycleFile(string) error {
	return errors.New("Recycle Bin deletion is supported only on Windows")
}
