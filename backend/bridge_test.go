package backend

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sort"
	"testing"
)

func TestAppFacadeMethodSet(t *testing.T) {
	want := []string{
		"AddFavorite", "AddTelegramFavorite", "AddYouTubeFavorite",
		"AssignTelegramFavoriteCategory", "AssignYouTubeFavoriteCategory",
		"CreateFavoriteCategory", "CreateTodo", "DeleteTodo", "GetAllRates",
		"GetChannelPosts", "GetChannelVideos", "GetFavoriteUpdateState",
		"GetFavoritesWithRates", "GetInitialFavoriteUpdates", "GetMCPServerStatus", "GetRate",
		"GetStoredLocationWeather", "GetThisWeekIncompleteTodos",
		"GetTodayIncompleteTodos", "GetTodos", "GetTodosByDueDate", "GetWeather",
		"GetWeatherForCity", "ListFavoriteCategories", "ListFavorites",
		"ListTelegramFavorites", "ListTelegramFavoritesWithCategories",
		"ListYouTubeFavorites", "ListYouTubeFavoritesWithCategories",
		"RefreshChannelPosts", "RefreshChannelVideos", "RefreshFavoriteUpdates",
		"RefreshStoredLocationWeather", "RemoveFavorite", "RemoveTelegramFavorite",
		"RemoveYouTubeFavorite", "SetMCPServerEnabled", "ToggleTodo", "UpdateTodo",
	}

	typ := reflect.TypeOf(&App{})
	got := make([]string, 0, typ.NumMethod())
	for index := 0; index < typ.NumMethod(); index++ { got = append(got, typ.Method(index).Name) }
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) { t.Fatalf("facade methods = %v, want %v", got, want) }
}

func TestAppFacadeRejectsCallsBeforeStartup(t *testing.T) {
	app := NewApp(NewService(), nil)
	if _, err := app.GetTodos(); !errors.Is(err, ErrBackendNotReady) {
		t.Fatalf("GetTodos error = %v, want ErrBackendNotReady", err)
	}
}

func TestAppFacadeRejectsCallsAfterShutdown(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil { t.Fatal(err) }
	service := NewService()
	ctx, cancel := context.WithCancel(context.Background())
	service.ctx = ctx
	service.cancel = cancel
	service.db = db
	service.ready = true
	service.Shutdown(context.Background())

	if _, err := NewApp(service, nil).GetTodos(); !errors.Is(err, ErrBackendNotReady) {
		t.Fatalf("GetTodos error = %v, want ErrBackendNotReady", err)
	}
}

func TestYouTubePaginationIsExplicitlyUnsupported(t *testing.T) {
	_, err := NewService().GetChannelVideosPaginated("UC0000000000000000000000", 1)
	var unsupported *UnsupportedPaginationError
	if !errors.As(err, &unsupported) { t.Fatalf("error = %v, want UnsupportedPaginationError", err) }
}
