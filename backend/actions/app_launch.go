package actions

import (
	"context"
	"errors"

	backendservice "something/backend/service"
)

// DesktopAppLauncher is the narrow native capability required by application
// launch actions. Callers supply the platform implementation; API inputs never
// contain executable paths.
type DesktopAppLauncher interface {
	Launch(path string) error
}

// DesktopAppLaunchResult is the path-free result returned to automation
// interfaces after one saved application has been launched.
type DesktopAppLaunchResult struct {
	AppID    int    `json:"app_id"`
	AppName  string `json:"app_name"`
	Launched bool   `json:"launched"`
}

// LaunchDesktopApp resolves a saved database ID to its validated executable
// path and delegates the actual OS operation to the supplied launcher.
func LaunchDesktopApp(
	ctx context.Context,
	service *backendservice.Service,
	launcher DesktopAppLauncher,
	id int,
) (DesktopAppLaunchResult, error) {
	if service == nil || launcher == nil {
		return DesktopAppLaunchResult{}, errors.New("opening applications is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	app, err := service.GetDesktopAppContext(ctx, id)
	if err != nil {
		return DesktopAppLaunchResult{}, err
	}
	executablePath, err := service.DesktopAppExecutableContext(ctx, id)
	if err != nil {
		return DesktopAppLaunchResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return DesktopAppLaunchResult{}, err
	}
	if err := launcher.Launch(executablePath); err != nil {
		return DesktopAppLaunchResult{}, errors.New("Windows could not open that application")
	}
	return DesktopAppLaunchResult{
		AppID:    app.ID,
		AppName:  app.DisplayName,
		Launched: true,
	}, nil
}

// StartSetup launches every saved application in setup order. An unavailable
// application is reported as a per-item failure and does not prevent later
// applications from being attempted.
func StartSetup(
	ctx context.Context,
	service *backendservice.Service,
	launcher DesktopAppLauncher,
	id int,
) (backendservice.SetupStartResult, error) {
	if service == nil || launcher == nil {
		return backendservice.SetupStartResult{}, errors.New("opening applications is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	setup, err := service.GetSetupContext(ctx, id)
	if err != nil {
		return backendservice.SetupStartResult{}, err
	}
	if len(setup.AppIDs) == 0 {
		return backendservice.SetupStartResult{}, &backendservice.ValidationError{
			Field:   "setup",
			Message: "This setup has no applications.",
		}
	}

	apps, err := service.ListDesktopAppsContext(ctx)
	if err != nil {
		return backendservice.SetupStartResult{}, err
	}
	appNames := make(map[int]string, len(apps))
	for _, app := range apps {
		appNames[app.ID] = app.DisplayName
	}

	result := backendservice.SetupStartResult{
		Attempted: len(setup.AppIDs),
		Failures:  []backendservice.SetupLaunchFailure{},
	}
	for _, appID := range setup.AppIDs {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		failure := backendservice.SetupLaunchFailure{
			AppID:   appID,
			AppName: appNames[appID],
		}
		executablePath, err := service.DesktopAppExecutableContext(ctx, appID)
		if err != nil {
			failure.Error = safeSetupLaunchFailure(err)
			result.Failures = append(result.Failures, failure)
			continue
		}
		if err := launcher.Launch(executablePath); err != nil {
			failure.Error = "Windows could not open this application"
			result.Failures = append(result.Failures, failure)
			continue
		}
		result.Launched++
	}
	return result, nil
}

func safeSetupLaunchFailure(err error) string {
	var validation *backendservice.ValidationError
	if errors.As(err, &validation) {
		return validation.Message
	}
	var notFound *backendservice.NotFoundError
	if errors.As(err, &notFound) {
		return "This application is no longer saved."
	}
	return "This application could not be prepared for launch."
}
