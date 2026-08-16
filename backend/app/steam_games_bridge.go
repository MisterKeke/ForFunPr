package backend

import "something/backend/service"

func (a *App) ListSteamGames() ([]SteamGame, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListSteamGamesContext(ctx)
}

func (a *App) AddSteamGame(storeURL string) (SteamGame, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return SteamGame{}, err
	}
	defer done()
	return service.AddSteamGameContext(ctx, storeURL)
}

func (a *App) DeleteSteamGame(id int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.DeleteSteamGameContext(ctx, id)
}

func (a *App) GetSteamGameSettings() (SteamGameSettings, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return SteamGameSettings{}, err
	}
	defer done()
	return service.GetSteamGameSettingsContext(ctx)
}

func (a *App) SetSteamGameCountry(code string) (SteamGameSettings, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return SteamGameSettings{}, err
	}
	defer done()
	return service.SetSteamGameCountryContext(ctx, code)
}

func (a *App) ListSteamCountries() ([]SteamCountry, error) {
	_, _, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListSupportedSteamCountries(), nil
}

func (a *App) RefreshSteamGamesOnOpen() (SteamGameRefreshResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return SteamGameRefreshResult{}, err
	}
	defer done()
	return service.RefreshSteamGamesOnOpenContext(ctx)
}

func (a *App) RefreshSteamGames() (SteamGameRefreshResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return SteamGameRefreshResult{}, err
	}
	defer done()
	return service.RefreshSteamGamesContext(ctx)
}
