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
		"ActivateRunningApp",
		"AssignTelegramFavoriteCategory",
		"AssignYouTubeFavoriteCategory",
		"CalculateDate",
		"CancelScreenshotOCR",
		"CancelGitWorkspaceJob",
		"CaptureScreenshot",
		"ChooseAndImportLegacyGitWorkspaceConfig",
		"ChooseGitWorkspaceFolder",
		"ChooseFileExplorerFolder",
		"ClearCalculatorHistory",
		"ClearClipboardHistory",
		"ClearWebsiteSearchHistory",
		"ConvertCalculatorUnit",
		"ConvertWorldTime",
		"CreateBookmark",
		"CreateFavoriteCategory",
		"CreateFavoriteCategoryDetailed",
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
		"DeleteFavoriteCategory",
		"DeleteFileExplorerFile",
		"DeleteNote",
		"DeleteNoteTopic",
		"DeleteNoteTopicBlock",
		"DeleteNoteTopicBlockWithRevision",
		"DeleteNoteTopicConnection",
		"DeleteNoteTopicConnectionWithRevision",
		"DeleteNoteTopicWithRevision",
		"DeleteScreenshot",
		"DeleteSetup",
		"DeleteSteamGame",
		"DeleteTodo",
		"DeleteUserWallpaper",
		"DeleteWebsiteSearchTarget",
		"DeleteWorldClock",
		"DetectLegacyGitWorkspaceConfig",
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
		"GetGitRepositoryDetails",
		"GetGitRepositoryHistory",
		"GetGitWorkspaceJob",
		"GetGitWorkspaceSettings",
		"GetMCPServerStatus",
		"GetNote",
		"GetNoteTopicBoard",
		"GetRate",
		"GetScreenshotCapabilities",
		"GetStartupStatus",
		"GetSteamGameSettings",
		"GetStoredLocationWeather",
		"GetThisWeekIncompleteTodos",
		"GetThisWeekTodos",
		"GetTodayIncompleteTodos",
		"GetTodayTodos",
		"GetTodoDatePreferences",
		"GetTodos",
		"GetTodosByDueDate",
		"GetWallpaperSettings",
		"GetWeather",
		"GetWeatherForCity",
		"GetWebsiteSearchRun",
		"GetWebsiteSearchState",
		"ImportDesktopAppIcon",
		"ImportLegacyGitWorkspaceConfig",
		"ImportWallpaper",
		"OpenGitRepositoryFolder",
		"OpenGitRepositoryInEditor",
		"OpenGitRepositoryRemote",
		"LaunchDesktopApp",
		"LinkNoteTodo",
		"LinkNoteTodoWithStatus",
		"ListBookmarks",
		"ListBookmarkTags",
		"ListCalculatorHistory",
		"ListCalculatorUnits",
		"ListClipboardItems",
		"ListDesktopApps",
		"ListFavoriteCategories",
		"ListFavorites",
		"ListFileExplorerDirectory",
		"ListGitRepositories",
		"ListGitWorkspaces",
		"ListNotes",
		"ListNoteTodos",
		"ListNoteTopics",
		"ListNoteTopicsPage",
		"ListRunningApps",
		"ListScreenshots",
		"ListSetups",
		"ListSteamCountries",
		"ListSteamGames",
		"ListTelegramFavorites",
		"ListTelegramFavoritesWithCategories",
		"ListTimeZones",
		"ListTodoNotes",
		"ListTodos",
		"ListWorldClocks",
		"ListYouTubeFavorites",
		"ListYouTubeFavoritesWithCategories",
		"OpenFileExplorerFile",
		"OpenWebsiteSearchResult",
		"PruneMissingGitRepositories",
		"RefreshChannelPosts",
		"RefreshChannelVideos",
		"RefreshFavoriteUpdates",
		"RefreshSteamGames",
		"RefreshSteamGamesOnOpen",
		"RefreshStoredLocationWeather",
		"RelocateDesktopApp",
		"RemoveFavorite",
		"RemoveGitWorkspace",
		"RemoveTelegramFavorite",
		"RemoveYouTubeFavorite",
		"RenameDesktopApp",
		"RenameFavoriteCategory",
		"RenameNoteTopic",
		"RenameScreenshot",
		"ReorderFavoriteCategories",
		"ReorderWorldClocks",
		"RestoreClipboardItem",
		"RunScreenshotOCR",
		"RevertScreenshotEdit",
		"SaveScreenshotEdit",
		"SearchTodos",
		"SearchNoteTopicPicker",
		"SearchWebsites",
		"SelectWallpaper",
		"SetBookmarkRead",
		"SetClipboardItemPinned",
		"SetMCPServerEnabled",
		"SetNoteArchived",
		"SetNotePinned",
		"SetSteamGameCountry",
		"SetTodoDatePreferences",
		"StartSetup",
		"StartGitFetch",
		"StartGitPull",
		"StartGitStatusRefresh",
		"StartGitSync",
		"StartGitWorkspaceRescan",
		"ToggleTodo",
		"ToggleTodoSubtask",
		"UnlinkNoteTodo",
		"UnlinkNoteTodoWithStatus",
		"UpdateBookmark",
		"UpdateClipboardSettings",
		"UpdateFavoriteCategory",
		"UpdateGitWorkspaceSettings",
		"UpdateNote",
		"UpdateNoteTopicBlockPosition",
		"UpdateNoteTopicBlockPositions",
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
