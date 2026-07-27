package backend

import (
	"context"
	"errors"
)

// MCPControl is the listener lifecycle surface required by the Wails facade.
// It avoids importing mcp-server into backend, which would create a cycle.
type MCPControl interface {
	Start() error
	Stop() error
	Snapshot() (state string, address string, lastError string)
}

// MCPServerStatus is the safe MCP runtime state returned to the frontend.
type MCPServerStatus struct {
	Running   bool   `json:"running"`
	State     string `json:"state"`
	Endpoint  string `json:"endpoint"`
	LastError string `json:"last_error,omitempty"`
}

// App is the deliberately narrow Wails binding surface. Lifecycle, database,
// listener, and cache-maintenance methods stay on unbound backend components.
// The name App preserves the existing window.go.backend.App JavaScript path.
type App struct {
	service *Service
	mcp     MCPControl
}

func NewApp(service *Service, mcp MCPControl) *App {
	return &App{
		service: service,
		mcp:     mcp,
	}
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
	if err != nil {
		return nil, err
	}
	defer done()
	return service.GetRateContext(ctx, base, target)
}

func (a *App) GetAllRates(base string) (*AllRatesResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.GetAllRatesContext(ctx, base)
}

func (a *App) AddFavorite(name string) (AddFavoriteResult, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return AddFavoriteResult{}, err
	}
	defer done()
	return service.AddFavorite(name)
}

func (a *App) RemoveFavorite(name string) (string, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return "", err
	}
	defer done()
	return service.RemoveFavorite(name)
}

func (a *App) ListFavorites() ([]string, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListFavorites()
}

func (a *App) GetFavoritesWithRates() (FavoritesWithRatesResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return FavoritesWithRatesResult{}, err
	}
	defer done()
	return service.GetFavoritesWithRatesContext(ctx)
}

func (a *App) GetWeather(latitude float64, longitude float64) (*WeatherResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.GetWeatherContext(ctx, latitude, longitude)
}

func (a *App) GetStoredLocationWeather() (*StoredLocationWeatherResult, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.GetStoredLocationWeather()
}

func (a *App) RefreshStoredLocationWeather() (*StoredLocationWeatherResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.RefreshStoredLocationWeatherContext(ctx)
}

func (a *App) GetWeatherForCity(city string) (*CityWeatherResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.GetWeatherForCityContext(ctx, city)
}

func (a *App) ListFavoriteCategories(source string) ([]FavoriteCategory, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListFavoriteCategories(source)
}

func (a *App) CreateFavoriteCategory(name string, source string) (FavoriteCategory, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return FavoriteCategory{}, err
	}
	defer done()
	return service.CreateFavoriteCategory(name, source)
}

func (a *App) RenameFavoriteCategory(id int, name string) (FavoriteCategory, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return FavoriteCategory{}, err
	}
	defer done()
	return service.RenameFavoriteCategory(id, name)
}

func (a *App) GetTodos() ([]Todo, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.GetTodosContext(ctx)
}

func (a *App) GetTodayIncompleteTodos() ([]Todo, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.GetTodayIncompleteTodos()
}

func (a *App) GetThisWeekIncompleteTodos() ([]Todo, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.GetThisWeekIncompleteTodos()
}

func (a *App) GetTodosByDueDate(dueDate string) ([]Todo, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.GetTodosByDueDate(dueDate)
}

func (a *App) CreateTodo(request TodoCreateRequest) ([]Todo, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.CreateTodoContext(ctx, request)
}

func (a *App) UpdateTodo(request TodoUpdateRequest) ([]Todo, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.UpdateTodoContext(ctx, request)
}

func (a *App) ToggleTodo(request TodoIDRequest) ([]Todo, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ToggleTodoContext(ctx, request)
}

func (a *App) ToggleTodoSubtask(request TodoSubtaskIDRequest) ([]Todo, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ToggleTodoSubtaskContext(ctx, request)
}

func (a *App) DeleteTodo(request TodoIDRequest) ([]Todo, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.DeleteTodoContext(ctx, request)
}

func (a *App) GetInitialFavoriteUpdates() (FavoriteUpdateScanResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return FavoriteUpdateScanResult{}, err
	}
	defer done()
	return service.GetInitialFavoriteUpdatesContext(ctx)
}

func (a *App) RefreshFavoriteUpdates() (FavoriteUpdateScanResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return FavoriteUpdateScanResult{}, err
	}
	defer done()
	return service.RefreshFavoriteUpdatesContext(ctx)
}

func (a *App) GetFavoriteUpdateState() (FavoriteUpdateState, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return FavoriteUpdateState{}, err
	}
	defer done()
	return service.GetFavoriteUpdateState()
}

func (a *App) GetChannelPosts(channel string) ([]TelegramPost, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.GetChannelPostsContext(ctx, channel)
}

func (a *App) RefreshChannelPosts(channel string) ([]TelegramPost, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.RefreshChannelPostsContext(ctx, channel)
}

func (a *App) AddTelegramFavorite(username string) ([]string, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.AddTelegramFavorite(username)
}

func (a *App) RemoveTelegramFavorite(username string) ([]string, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.RemoveTelegramFavorite(username)
}

func (a *App) ListTelegramFavorites() ([]string, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListTelegramFavorites()
}

func (a *App) AssignTelegramFavoriteCategory(username string, categoryID int) error {
	service, _, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.AssignTelegramFavoriteCategory(username, categoryID)
}

func (a *App) ListTelegramFavoritesWithCategories() ([]FavoriteChannel, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListTelegramFavoritesWithCategories()
}

func (a *App) GetChannelVideos(channel string) ([]YouTubeVideo, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.GetChannelVideosContext(ctx, channel)
}

func (a *App) RefreshChannelVideos(channel string) ([]YouTubeVideo, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.RefreshChannelVideosContext(ctx, channel)
}

func (a *App) AddYouTubeFavorite(channel string) ([]string, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.AddYouTubeFavoriteContext(ctx, channel)
}

func (a *App) RemoveYouTubeFavorite(channel string) ([]string, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.RemoveYouTubeFavoriteContext(ctx, channel)
}

func (a *App) ListYouTubeFavorites() ([]string, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListYouTubeFavorites()
}

func (a *App) AssignYouTubeFavoriteCategory(channel string, categoryID int) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.AssignYouTubeFavoriteCategoryContext(ctx, channel, categoryID)
}

func (a *App) ListYouTubeFavoritesWithCategories() ([]FavoriteChannel, error) {
	service, _, done, err := a.begin()
	if err != nil {
		return nil, err
	}
	defer done()
	return service.ListYouTubeFavoritesWithCategories()
}

func (a *App) GetMCPServerStatus() MCPServerStatus {
	if a == nil || a.mcp == nil {
		return MCPServerStatus{
			State:     "unavailable",
			LastError: "MCP server control is unavailable.",
		}
	}

	state, address, lastError := a.mcp.Snapshot()

	endpoint := ""
	if address != "" {
		endpoint = "http://" + address + "/mcp"
	}

	return MCPServerStatus{
		Running:   state == "on",
		State:     state,
		Endpoint:  endpoint,
		LastError: lastError,
	}
}

func (a *App) SetMCPServerEnabled(
	enabled bool,
) (MCPServerStatus, error) {
	if a == nil || a.mcp == nil {
		return a.GetMCPServerStatus(),
			errors.New("MCP server control is unavailable")
	}

	var err error
	if enabled {
		if a.service == nil || !a.service.GetStartupStatus().Ready {
			return a.GetMCPServerStatus(),
				errors.New("MCP cannot start because the desktop API is unavailable")
		}
		err = a.mcp.Start()
	} else {
		err = a.mcp.Stop()
	}

	return a.GetMCPServerStatus(), err
}
