package backend

func (a *App) ListTimeZones() ([]TimeZoneOption, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListTimeZonesContext(ctx), nil
}

func (a *App) ListWorldClocks() ([]WorldClock, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListWorldClocksContext(ctx)
}

func (a *App) CreateWorldClock(request WorldClockWriteRequest) (WorldClock, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return WorldClock{}, err
	}
	defer done()
	return service.CreateWorldClockContext(ctx, request)
}

func (a *App) UpdateWorldClock(request WorldClockWriteRequest) (WorldClock, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return WorldClock{}, err
	}
	defer done()
	return service.UpdateWorldClockContext(ctx, request)
}

func (a *App) DeleteWorldClock(id int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.DeleteWorldClockContext(ctx, id)
}

func (a *App) ReorderWorldClocks(request WorldClockOrderRequest) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.ReorderWorldClocksContext(ctx, request)
}

func (a *App) ConvertWorldTime(request WorldTimeConversionRequest) ([]WorldTimeConversion, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ConvertWorldTimeContext(ctx, request)
}
