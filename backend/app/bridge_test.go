package backend

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	backendservice "something/backend/service"
)

func TestAppFacadeMethodSet(t *testing.T) {
	want := []string{
		"AddFavorite", "AddTelegramFavorite", "AddYouTubeFavorite",
		"AssignTelegramFavoriteCategory", "AssignYouTubeFavoriteCategory",
		"CreateFavoriteCategory", "CreateTodo", "DeleteTodo", "DeleteUserWallpaper", "GetAllRates",
		"GetChannelPosts", "GetChannelVideos", "GetFavoriteUpdateState",
		"GetFavoritesWithRates", "GetInitialFavoriteUpdates", "GetMCPServerStatus", "GetRate",
		"GetStoredLocationWeather", "GetThisWeekIncompleteTodos",
		"GetTodayIncompleteTodos", "GetTodos", "GetTodosByDueDate", "GetWeather",
		"GetWallpaperSettings", "GetWeatherForCity", "ImportWallpaper", "ListFavoriteCategories", "ListFavorites",
		"ListTelegramFavorites", "ListTelegramFavoritesWithCategories",
		"ListYouTubeFavorites", "ListYouTubeFavoritesWithCategories",
		"RefreshChannelPosts", "RefreshChannelVideos", "RefreshFavoriteUpdates",
		"RefreshStoredLocationWeather", "RemoveFavorite", "RemoveTelegramFavorite",
		"RemoveYouTubeFavorite", "RenameFavoriteCategory", "SelectWallpaper", "SetMCPServerEnabled",
		"SearchTodos", "ToggleTodo", "ToggleTodoSubtask", "UpdateTodo",
	}

	typ := reflect.TypeOf(&App{})
	got := make([]string, 0, typ.NumMethod())
	for index := 0; index < typ.NumMethod(); index++ {
		got = append(got, typ.Method(index).Name)
	}
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("facade methods = %v, want %v", got, want)
	}
}

func TestAppFacadeRejectsCallsBeforeStartup(t *testing.T) {
	app := NewApp(backendservice.NewService(), nil)
	if _, err := app.GetTodos(); !errors.Is(err, backendservice.ErrBackendNotReady) {
		t.Fatalf("GetTodos error = %v, want ErrBackendNotReady", err)
	}
}

func TestAppFacadeRejectsCallsAfterShutdown(t *testing.T) {
	service := backendservice.NewService()
	service.Shutdown(context.Background())

	if _, err := NewApp(service, nil).GetTodos(); !errors.Is(err, backendservice.ErrBackendNotReady) {
		t.Fatalf("GetTodos error = %v, want ErrBackendNotReady", err)
	}
}

func TestYouTubePaginationIsExplicitlyUnsupported(t *testing.T) {
	_, err := backendservice.NewService().GetChannelVideosPaginated("UC0000000000000000000000", 1)
	var unsupported *backendservice.UnsupportedPaginationError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error = %v, want UnsupportedPaginationError", err)
	}
}
