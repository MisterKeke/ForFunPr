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
		"AddDesktopApp",
		"AddFavorite",
		"AddNoteTopicBlock",
		"AddSteamGame",
		"AddTelegramFavorite",
		"AddWebsiteSearchTarget",
		"AddYouTubeFavorite",
		"AssignTelegramFavoriteCategory",
		"AssignYouTubeFavoriteCategory",
		"CalculateDate",
		"CaptureScreenshot",
		"ChooseFileExplorerFolder",
		"ClearCalculatorHistory",
		"ClearClipboardHistory",
		"ClearWebsiteSearchHistory",
		"ConvertCalculatorUnit",
		"ConvertWorldTime",
		"CreateBookmark",
		"CreateFavoriteCategory",
		"CreateNote",
		"CreateNoteTopic",
		"CreateNoteTopicConnection",
		"CreateSetup",
		"CreateTodo",
		"CreateWorldClock",
		"DeleteBookmark",
		"DeleteCalculatorHistoryItem",
		"DeleteClipboardItem",
		"DeleteDesktopApp",
		"DeleteDesktopAppIcon",
		"DeleteFileExplorerFile",
		"DeleteNote",
		"DeleteNoteTopic",
		"DeleteNoteTopicBlock",
		"DeleteNoteTopicConnection",
		"DeleteScreenshot",
		"DeleteSetup",
		"DeleteSteamGame",
		"DeleteTodo",
		"DeleteUserWallpaper",
		"DeleteWebsiteSearchTarget",
		"DeleteWorldClock",
		"EvaluateCalculatorExpression",
		"ExportScreenshot",
		"GetAllRates",
		"GetBookmark",
		"GetChannelPosts",
		"GetChannelVideos",
		"GetClipboardState",
		"GetFavoritesWithRates",
		"GetFavoriteUpdateState",
		"GetFileExplorerPlaces",
		"GetInitialFavoriteUpdates",
		"GetMCPServerStatus",
		"GetNote",
		"GetNoteTopicBoard",
		"GetRate",
		"GetScreenshotCapabilities",
		"GetSteamGameSettings",
		"GetStoredLocationWeather",
		"GetThisWeekIncompleteTodos",
		"GetTodayIncompleteTodos",
		"GetTodos",
		"GetTodosByDueDate",
		"GetWallpaperSettings",
		"GetWeather",
		"GetWeatherForCity",
		"GetWebsiteSearchRun",
		"GetWebsiteSearchState",
		"ImportDesktopAppIcon",
		"ImportWallpaper",
		"LaunchDesktopApp",
		"LinkNoteTodo",
		"ListBookmarks",
		"ListBookmarkTags",
		"ListCalculatorHistory",
		"ListCalculatorUnits",
		"ListClipboardItems",
		"ListDesktopApps",
		"ListFavoriteCategories",
		"ListFavorites",
		"ListFileExplorerDirectory",
		"ListNotes",
		"ListNoteTodos",
		"ListNoteTopics",
		"ListScreenshots",
		"ListSetups",
		"ListSteamCountries",
		"ListSteamGames",
		"ListTelegramFavorites",
		"ListTelegramFavoritesWithCategories",
		"ListTimeZones",
		"ListTodoNotes",
		"ListWorldClocks",
		"ListYouTubeFavorites",
		"ListYouTubeFavoritesWithCategories",
		"OpenFileExplorerFile",
		"OpenWebsiteSearchResult",
		"RefreshChannelPosts",
		"RefreshChannelVideos",
		"RefreshFavoriteUpdates",
		"RefreshSteamGames",
		"RefreshSteamGamesOnOpen",
		"RefreshStoredLocationWeather",
		"RelocateDesktopApp",
		"RemoveFavorite",
		"RemoveTelegramFavorite",
		"RemoveYouTubeFavorite",
		"RenameDesktopApp",
		"RenameFavoriteCategory",
		"RenameNoteTopic",
		"RenameScreenshot",
		"ReorderWorldClocks",
		"RestoreClipboardItem",
		"RunScreenshotOCR",
		"SaveScreenshotEdit",
		"SearchTodos",
		"SearchWebsites",
		"SelectWallpaper",
		"SetBookmarkRead",
		"SetClipboardItemPinned",
		"SetMCPServerEnabled",
		"SetNoteArchived",
		"SetNotePinned",
		"SetSteamGameCountry",
		"StartSetup",
		"ToggleTodo",
		"ToggleTodoSubtask",
		"UnlinkNoteTodo",
		"UpdateBookmark",
		"UpdateClipboardSettings",
		"UpdateNote",
		"UpdateNoteTopicBlockPosition",
		"UpdateSetup",
		"UpdateTodo",
		"UpdateWorldClock",
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
