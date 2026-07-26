package backend

import "context"

// App is the deliberately narrow Wails binding surface. Lifecycle, database,
// listener, and cache-maintenance methods stay on the unbound Service type.
// The name App preserves the existing window.go.backend.App JavaScript path.
type App struct {
	service *Service
}

func NewApp(service *Service) *App {
	return &App{service: service}
}

func (a *App) begin() (*Service, context.Context, func(), error) {
	if a == nil || a.service == nil {
		return nil, nil, nil, ErrBackendNotReady
	}
	ctx, done, err := a.service.BeginOperation(a.service.requestContext())
	if err != nil {
		return nil, nil, nil, err
	}
	return a.service, ctx, done, nil
}

func (a *App) GetRate(base string, target string) (*RateResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.GetRateContext(ctx, base, target)
}

func (a *App) GetAllRates(base string) (*AllRatesResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.GetAllRatesContext(ctx, base)
}

func (a *App) AddFavorite(name string) (AddFavoriteResult, error) {
	service, _, done, err := a.begin()
	if err != nil { return AddFavoriteResult{}, err }
	defer done()
	return service.AddFavorite(name)
}

func (a *App) RemoveFavorite(name string) (string, error) {
	service, _, done, err := a.begin()
	if err != nil { return "", err }
	defer done()
	return service.RemoveFavorite(name)
}

func (a *App) ListFavorites() ([]string, error) {
	service, _, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.ListFavorites()
}

func (a *App) GetFavoritesWithRates() (FavoritesWithRatesResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return FavoritesWithRatesResult{}, err }
	defer done()
	return service.GetFavoritesWithRatesContext(ctx)
}

func (a *App) GetWeather(latitude float64, longitude float64) (*WeatherResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.GetWeatherContext(ctx, latitude, longitude)
}

func (a *App) GetStoredLocationWeather() (*StoredLocationWeatherResult, error) {
	service, _, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.GetStoredLocationWeather()
}

func (a *App) RefreshStoredLocationWeather() (*StoredLocationWeatherResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.RefreshStoredLocationWeatherContext(ctx)
}

func (a *App) GetWeatherForCity(city string) (*CityWeatherResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.GetWeatherForCityContext(ctx, city)
}

func (a *App) ListFavoriteCategories(source string) ([]FavoriteCategory, error) {
	service, _, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.ListFavoriteCategories(source)
}

func (a *App) CreateFavoriteCategory(name string, source string) (FavoriteCategory, error) {
	service, _, done, err := a.begin()
	if err != nil { return FavoriteCategory{}, err }
	defer done()
	return service.CreateFavoriteCategory(name, source)
}

func (a *App) GetTodos() ([]Todo, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.GetTodosContext(ctx)
}

func (a *App) GetTodayIncompleteTodos() ([]Todo, error) {
	service, _, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.GetTodayIncompleteTodos()
}

func (a *App) GetThisWeekIncompleteTodos() ([]Todo, error) {
	service, _, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.GetThisWeekIncompleteTodos()
}

func (a *App) GetTodosByDueDate(dueDate string) ([]Todo, error) {
	service, _, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.GetTodosByDueDate(dueDate)
}

func (a *App) CreateTodo(request TodoCreateRequest) ([]Todo, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.CreateTodoContext(ctx, request)
}

func (a *App) UpdateTodo(request TodoUpdateRequest) ([]Todo, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.UpdateTodoContext(ctx, request)
}

func (a *App) ToggleTodo(request TodoIDRequest) ([]Todo, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.ToggleTodoContext(ctx, request)
}

func (a *App) DeleteTodo(request TodoIDRequest) ([]Todo, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.DeleteTodoContext(ctx, request)
}

func (a *App) GetInitialFavoriteUpdates() (FavoriteUpdateScanResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return FavoriteUpdateScanResult{}, err }
	defer done()
	return service.GetInitialFavoriteUpdatesContext(ctx)
}

func (a *App) RefreshFavoriteUpdates() (FavoriteUpdateScanResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return FavoriteUpdateScanResult{}, err }
	defer done()
	return service.RefreshFavoriteUpdatesContext(ctx)
}

func (a *App) GetFavoriteUpdateState() (FavoriteUpdateState, error) {
	service, _, done, err := a.begin()
	if err != nil { return FavoriteUpdateState{}, err }
	defer done()
	return service.GetFavoriteUpdateState()
}

func (a *App) GetChannelPosts(channel string) ([]TelegramPost, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.GetChannelPostsContext(ctx, channel)
}

func (a *App) RefreshChannelPosts(channel string) ([]TelegramPost, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.RefreshChannelPostsContext(ctx, channel)
}

func (a *App) AddTelegramFavorite(username string) ([]string, error) {
	service, _, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.AddTelegramFavorite(username)
}

func (a *App) RemoveTelegramFavorite(username string) ([]string, error) {
	service, _, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.RemoveTelegramFavorite(username)
}

func (a *App) ListTelegramFavorites() ([]string, error) {
	service, _, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.ListTelegramFavorites()
}

func (a *App) AssignTelegramFavoriteCategory(username string, categoryID int) error {
	service, _, done, err := a.begin()
	if err != nil { return err }
	defer done()
	return service.AssignTelegramFavoriteCategory(username, categoryID)
}

func (a *App) ListTelegramFavoritesWithCategories() ([]FavoriteChannel, error) {
	service, _, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.ListTelegramFavoritesWithCategories()
}

func (a *App) GetChannelVideos(channel string) ([]YouTubeVideo, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.GetChannelVideosContext(ctx, channel)
}

func (a *App) RefreshChannelVideos(channel string) ([]YouTubeVideo, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.RefreshChannelVideosContext(ctx, channel)
}

func (a *App) AddYouTubeFavorite(channel string) ([]string, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.AddYouTubeFavoriteContext(ctx, channel)
}

func (a *App) RemoveYouTubeFavorite(channel string) ([]string, error) {
	service, ctx, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.RemoveYouTubeFavoriteContext(ctx, channel)
}

func (a *App) ListYouTubeFavorites() ([]string, error) {
	service, _, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.ListYouTubeFavorites()
}

func (a *App) AssignYouTubeFavoriteCategory(channel string, categoryID int) error {
	service, ctx, done, err := a.begin()
	if err != nil { return err }
	defer done()
	return service.AssignYouTubeFavoriteCategoryContext(ctx, channel, categoryID)
}

func (a *App) ListYouTubeFavoritesWithCategories() ([]FavoriteChannel, error) {
	service, _, done, err := a.begin()
	if err != nil { return nil, err }
	defer done()
	return service.ListYouTubeFavoritesWithCategories()
}
