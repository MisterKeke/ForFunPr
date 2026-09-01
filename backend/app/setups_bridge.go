package backend

import "something/backend/actions"

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
	return actions.StartSetup(ctx, service, a.desktopAppLauncher, id)
}
