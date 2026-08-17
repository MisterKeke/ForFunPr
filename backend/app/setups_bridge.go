package backend

import (
	"errors"

	backendservice "something/backend/service"
)

func (a *App) ListSetups() ([]Setup, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListSetupsContext(ctx)
}

func (a *App) CreateSetup(request SetupCreateRequest) (Setup, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Setup{}, err
	}
	defer done()
	return service.CreateSetupContext(ctx, request)
}

func (a *App) UpdateSetup(request SetupUpdateRequest) (Setup, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Setup{}, err
	}
	defer done()
	return service.UpdateSetupContext(ctx, request)
}

func (a *App) DeleteSetup(id int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.DeleteSetupContext(ctx, id)
}

func (a *App) StartSetup(id int) (SetupStartResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return SetupStartResult{}, err
	}
	defer done()
	if a.desktopAppLauncher == nil {
		return SetupStartResult{}, errors.New("opening applications is unavailable")
	}

	setup, err := service.GetSetupContext(ctx, id)
	if err != nil {
		return SetupStartResult{}, err
	}
	if len(setup.AppIDs) == 0 {
		return SetupStartResult{}, &backendservice.ValidationError{
			Field:   "setup",
			Message: "This setup has no applications.",
		}
	}

	apps, err := service.ListDesktopAppsContext(ctx)
	if err != nil {
		return SetupStartResult{}, err
	}
	appNames := make(map[int]string, len(apps))
	for _, app := range apps {
		appNames[app.ID] = app.DisplayName
	}

	result := SetupStartResult{
		Attempted: len(setup.AppIDs),
		Failures:  []SetupLaunchFailure{},
	}
	for _, appID := range setup.AppIDs {
		failure := SetupLaunchFailure{AppID: appID, AppName: appNames[appID]}
		executablePath, err := service.DesktopAppExecutableContext(ctx, appID)
		if err != nil {
			failure.Error = err.Error()
			result.Failures = append(result.Failures, failure)
			continue
		}
		if err := a.desktopAppLauncher.Launch(executablePath); err != nil {
			failure.Error = "Windows could not open this application"
			result.Failures = append(result.Failures, failure)
			continue
		}
		result.Launched++
	}
	return result, nil
}
