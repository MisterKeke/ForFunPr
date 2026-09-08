package backend

import (
	"something/backend/fileexplorer"
	"something/backend/screencapture"
	"something/backend/service"
)

// These aliases keep the Wails-facing API in the backend namespace while the
// implementation and platform-specific models live in focused packages.
type Service = service.Service
type StartupStatus = service.StartupStatus
type CapabilityState = service.CapabilityState
type ScreenshotCaptureRequest = screencapture.Request
type ScreenshotCaptureRectangle = screencapture.Rectangle
type ScreenshotOCRQueueResult = service.ScreenshotOCRQueueResult
type AddFavoriteResult = service.AddFavoriteResult
type AllRatesResult = service.AllRatesResult
type Bookmark = service.Bookmark
type BookmarkCreateRequest = service.BookmarkCreateRequest
type BookmarkFilter = service.BookmarkFilter
type BookmarkListResult = service.BookmarkListResult
type BookmarkReadRequest = service.BookmarkReadRequest
type BookmarkUpdateRequest = service.BookmarkUpdateRequest
type CityWeatherResult = service.CityWeatherResult
type ClipboardItem = service.ClipboardItem
type ClipboardListFilter = service.ClipboardListFilter
type ClipboardListResult = service.ClipboardListResult
type ClipboardSettings = service.ClipboardSettings
type CalculatorExpressionRequest = service.CalculatorExpressionRequest
type CalculatorHistoryItem = service.CalculatorHistoryItem
type CalculatorResult = service.CalculatorResult
type DateCalculationRequest = service.DateCalculationRequest
type DateCalculationResult = service.DateCalculationResult
type DesktopApp = service.DesktopApp
type FavoriteCategory = service.FavoriteCategory
type FavoriteCategoryWriteRequest = service.FavoriteCategoryWriteRequest
type FavoriteCategoryMutationResult = service.FavoriteCategoryMutationResult
type FavoriteCategoryDeleteRequest = service.FavoriteCategoryDeleteRequest
type FavoriteCategoryDeleteResult = service.FavoriteCategoryDeleteResult
type FavoriteCategoryReorderRequest = service.FavoriteCategoryReorderRequest
type FavoriteChannel = service.FavoriteChannel
type FavoritesWithRatesResult = service.FavoritesWithRatesResult
type FavoriteUpdateScanResult = service.FavoriteUpdateScanResult
type FavoriteUpdateState = service.FavoriteUpdateState
type Note = service.Note
type NoteCreateRequest = service.NoteCreateRequest
type NoteListFilter = service.NoteListFilter
type NoteListResult = service.NoteListResult
type NoteSummary = service.NoteSummary
type NoteStateRequest = service.NoteStateRequest
type NoteTodoConnectionRequest = service.NoteTodoConnectionRequest
type NoteTodoMutationResult = service.NoteTodoMutationResult
type NoteTopic = service.NoteTopic
type NoteTopicBlock = service.NoteTopicBlock
type NoteTopicBlockCreateRequest = service.NoteTopicBlockCreateRequest
type NoteTopicBlockPositionRequest = service.NoteTopicBlockPositionRequest
type NoteTopicBlockPositionsRequest = service.NoteTopicBlockPositionsRequest
type NoteTopicBoard = service.NoteTopicBoard
type NoteTopicConnection = service.NoteTopicConnection
type NoteTopicConnectionCreateRequest = service.NoteTopicConnectionCreateRequest
type NoteTopicIDRequest = service.NoteTopicIDRequest
type NoteTopicListFilter = service.NoteTopicListFilter
type NoteTopicListResult = service.NoteTopicListResult
type NoteTopicMutationResult = service.NoteTopicMutationResult
type NoteTopicPickerFilter = service.NoteTopicPickerFilter
type NoteTopicPickerResult = service.NoteTopicPickerResult
type NoteTopicWriteRequest = service.NoteTopicWriteRequest
type NoteUpdateRequest = service.NoteUpdateRequest
type RateResult = service.RateResult
type Screenshot = service.Screenshot
type ScreenshotEditRequest = service.ScreenshotEditRequest
type ScreenshotListFilter = service.ScreenshotListFilter
type ScreenshotListResult = service.ScreenshotListResult
type Setup = service.Setup
type SetupCreateRequest = service.SetupCreateRequest
type SetupLaunchFailure = service.SetupLaunchFailure
type SetupStartResult = service.SetupStartResult
type SetupUpdateRequest = service.SetupUpdateRequest
type SteamCountry = service.SteamCountry
type SteamGame = service.SteamGame
type SteamGameRefreshResult = service.SteamGameRefreshResult
type SteamGameSettings = service.SteamGameSettings
type StoredLocationWeatherResult = service.StoredLocationWeatherResult
type TelegramPost = service.TelegramPost
type Todo = service.Todo
type TodoCreateRequest = service.TodoCreateRequest
type TodoFilter = service.TodoFilter
type TodoListResult = service.TodoListResult
type TodoTodayQuery = service.TodoTodayQuery
type TodoTodayResult = service.TodoTodayResult
type TodoWeekQuery = service.TodoWeekQuery
type TodoWeekResult = service.TodoWeekResult
type TodoDatePreferences = service.TodoDatePreferences
type TodoDeletionReceipt = service.TodoDeletionReceipt
type TodoIDRequest = service.TodoIDRequest
type TodoSubtaskIDRequest = service.TodoSubtaskIDRequest
type TodoUpdateRequest = service.TodoUpdateRequest
type UserWallpaper = service.UserWallpaper
type UnitConversionRequest = service.UnitConversionRequest
type UnitOption = service.UnitOption
type ValidationError = service.ValidationError
type WallpaperSettings = service.WallpaperSettings
type WeatherResult = service.WeatherResult
type WebsiteSearchHistoryItem = service.WebsiteSearchHistoryItem
type WebsiteSearchPageResult = service.WebsiteSearchPageResult
type WebsiteSearchRequest = service.WebsiteSearchRequest
type WebsiteSearchResponse = service.WebsiteSearchResponse
type WebsiteSearchState = service.WebsiteSearchState
type WebsiteSearchTarget = service.WebsiteSearchTarget
type YouTubeVideo = service.YouTubeVideo
type WorldClock = service.WorldClock
type WorldClockOrderRequest = service.WorldClockOrderRequest
type WorldClockWriteRequest = service.WorldClockWriteRequest
type WorldTimeConversion = service.WorldTimeConversion
type WorldTimeConversionRequest = service.WorldTimeConversionRequest
type TimeZoneOption = service.TimeZoneOption

type FileExplorerDirectoryRequest = fileexplorer.FileExplorerDirectoryRequest
type FileExplorerFileRequest = fileexplorer.FileExplorerFileRequest
type FileExplorerListing = fileexplorer.FileExplorerListing
type FileExplorerPlace = fileexplorer.FileExplorerPlace

var ErrBackendNotReady = service.ErrBackendNotReady

const fileExplorerDefaultPageSize = fileexplorer.DefaultPageSize
