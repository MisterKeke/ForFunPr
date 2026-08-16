package backend

import (
	"context"
	"errors"
	"fmt"

	backendservice "something/backend/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) ListDesktopApps() ([]DesktopApp, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListDesktopAppsContext(ctx)
}

// AddDesktopApp opens the native file picker and stores the selected
// executable path. A nil result means the user cancelled the picker.
func (a *App) AddDesktopApp() (*DesktopApp, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()

	selectedPath, err := chooseDesktopExecutable(ctx, "Choose an application")
	if err != nil {
		return nil, err
	}
	if selectedPath == "" {
		return nil, nil
	}
	app, err := service.AddDesktopAppFromPathContext(ctx, selectedPath)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// RelocateDesktopApp lets the user repair or change a stored executable path.
// A nil result means the user cancelled the picker.
func (a *App) RelocateDesktopApp(id int) (*DesktopApp, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()

	if err := backendservice.ValidateDesktopAppID(id); err != nil {
		return nil, err
	}
	selectedPath, err := chooseDesktopExecutable(ctx, "Locate the application executable")
	if err != nil {
		return nil, err
	}
	if selectedPath == "" {
		return nil, nil
	}
	app, err := service.RelocateDesktopAppContext(ctx, id, selectedPath)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (a *App) RenameDesktopApp(id int, displayName string) (DesktopApp, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return DesktopApp{}, err
	}
	defer done()
	return service.RenameDesktopAppContext(ctx, id, displayName)
}

// ImportDesktopAppIcon opens the native image picker and copies the selected
// icon into the application-owned icons directory. A nil result means the
// user cancelled the picker.
func (a *App) ImportDesktopAppIcon(id int) (*DesktopApp, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	if err := backendservice.ValidateDesktopAppID(id); err != nil {
		return nil, err
	}

	selectedPath, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		Title: "Choose an application icon",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Icon images (*.ico;*.jpg;*.jpeg;*.png;*.webp)",
				Pattern:     "*.ico;*.jpg;*.jpeg;*.png;*.webp",
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("choose application icon: %w", err)
	}
	if selectedPath == "" {
		return nil, nil
	}
	app, err := service.ImportDesktopAppIconFromPathContext(ctx, id, selectedPath)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (a *App) DeleteDesktopAppIcon(id int) (DesktopApp, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return DesktopApp{}, err
	}
	defer done()
	return service.DeleteDesktopAppIconContext(ctx, id)
}

func (a *App) DeleteDesktopApp(id int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.DeleteDesktopAppContext(ctx, id)
}

func (a *App) LaunchDesktopApp(id int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	if a.desktopAppLauncher == nil {
		return errors.New("opening applications is unavailable")
	}
	executablePath, err := service.DesktopAppExecutableContext(ctx, id)
	if err != nil {
		return err
	}
	if err := a.desktopAppLauncher.Launch(executablePath); err != nil {
		return errors.New("Windows could not open that application")
	}
	return nil
}

func chooseDesktopExecutable(ctx context.Context, title string) (string, error) {
	// Wails runtime methods require context.Context. Keeping the picker in this
	// bridge ensures arbitrary frontend-provided paths are never accepted.
	selectedPath, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		Title: title,
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Windows applications (*.exe)",
				Pattern:     "*.exe",
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("choose application executable: %w", err)
	}
	return selectedPath, nil
}

