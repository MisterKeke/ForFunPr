package backend

import "something/backend/actions"

func (a *App) GetWebsiteSearchState() (WebsiteSearchState, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return WebsiteSearchState{}, err
	}
	defer done()
	return service.GetWebsiteSearchStateContext(ctx)
}

func (a *App) AddWebsiteSearchTarget(url string) (WebsiteSearchTarget, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return WebsiteSearchTarget{}, err
	}
	defer done()
	return service.AddWebsiteSearchTargetContext(ctx, url)
}

func (a *App) DeleteWebsiteSearchTarget(id int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.DeleteWebsiteSearchTargetContext(ctx, id)
}

func (a *App) SearchWebsites(request WebsiteSearchRequest) (WebsiteSearchResponse, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return WebsiteSearchResponse{}, err
	}
	defer done()
	return service.SearchWebsitesContext(ctx, request)
}

func (a *App) GetWebsiteSearchRun(id int) (WebsiteSearchResponse, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return WebsiteSearchResponse{}, err
	}
	defer done()
	return service.GetWebsiteSearchRunContext(ctx, id)
}

func (a *App) ClearWebsiteSearchHistory() error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.ClearWebsiteSearchHistoryContext(ctx)
}

func (a *App) OpenWebsiteSearchResult(url string) error {
	_, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return actions.OpenExternalURL(ctx, a.externalURLLauncher, url)
}
