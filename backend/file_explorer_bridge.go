package backend

import (
	"errors"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) GetFileExplorerPlaces() ([]FileExplorerPlace, error) {
	_, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	if a.fileExplorer == nil {
		return nil, errors.New("file explorer is unavailable")
	}
	return a.fileExplorer.Places(ctx)
}

// ChooseFileExplorerFolder grants this app session access to a user-selected
// directory. A nil result means the native folder picker was cancelled.
func (a *App) ChooseFileExplorerFolder() (*FileExplorerListing, error) {
	_, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	if a.fileExplorer == nil {
		return nil, errors.New("file explorer is unavailable")
	}

	selectedPath, err := runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "Choose a folder to browse",
	})
	if err != nil {
		return nil, errors.New("the folder picker could not be opened")
	}
	if selectedPath == "" {
		return nil, nil
	}

	place, err := a.fileExplorer.RegisterChosenRoot(ctx, selectedPath)
	if err != nil {
		return nil, err
	}
	return a.fileExplorer.ListDirectory(ctx, FileExplorerDirectoryRequest{
		RootID: place.RootID,
		Limit:  fileExplorerDefaultPageSize,
	})
}

func (a *App) ListFileExplorerDirectory(
	request FileExplorerDirectoryRequest,
) (*FileExplorerListing, error) {
	_, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	if a.fileExplorer == nil {
		return nil, errors.New("file explorer is unavailable")
	}
	return a.fileExplorer.ListDirectory(ctx, request)
}

func (a *App) OpenFileExplorerFile(request FileExplorerFileRequest) error {
	_, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	if a.fileExplorer == nil {
		return errors.New("file explorer is unavailable")
	}
	return a.fileExplorer.OpenFile(ctx, request)
}

func (a *App) DeleteFileExplorerFile(request FileExplorerFileRequest) error {
	_, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	if a.fileExplorer == nil {
		return errors.New("file explorer is unavailable")
	}
	return a.fileExplorer.DeleteFile(ctx, request)
}
