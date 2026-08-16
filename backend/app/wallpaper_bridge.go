package backend

import (
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) GetWallpaperSettings() (*WallpaperSettings, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.GetWallpaperSettingsContext(ctx)
}

// ImportWallpaper opens the native desktop file picker and copies the chosen
// image into the application-owned user-wallpapers directory. A nil result
// means the user cancelled the picker.
func (a *App) ImportWallpaper() (*UserWallpaper, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()

	sourcePath, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		Title: "Choose a wallpaper",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Images (*.jpg;*.jpeg;*.png;*.webp)",
				Pattern:     "*.jpg;*.jpeg;*.png;*.webp",
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("choose wallpaper: %w", err)
	}
	if sourcePath == "" {
		return nil, nil
	}

	return service.ImportWallpaperFromPathContext(ctx, sourcePath)
}

func (a *App) SelectWallpaper(selection string) (*WallpaperSettings, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.SelectWallpaperContext(ctx, selection)
}

func (a *App) DeleteUserWallpaper(id string) (*WallpaperSettings, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.DeleteUserWallpaperContext(ctx, id)
}
