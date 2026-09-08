package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"something/backend/launcher"
	backend "something/backend/service"
	"something/internal/policy"
)

const maximumPostCount = 20

func newRouter(app *backend.Service) http.Handler {
	mux := http.NewServeMux()
	appLauncher := launcher.New()

	mux.HandleFunc("GET /api/v1/health", healthHandler(app))

	mux.HandleFunc("GET /api/v1/news", newsHandler(app))
	mux.HandleFunc("POST /api/v1/news/initial", initialFavoriteUpdatesHandler(app))
	mux.HandleFunc("POST /api/v1/news/refresh", refreshFavoriteUpdatesHandler(app))
	mux.HandleFunc("POST /api/v1/news/since-last-open", favoriteUpdatesSinceLastOpenHandler(app))
	mux.HandleFunc("GET /api/v1/news/state", favoriteUpdateStateHandler(app))
	mux.HandleFunc("GET /api/v1/news/windows", updateWindowsHandler(app))

	mux.HandleFunc("GET /api/v1/posts/telegram/{channel}", telegramPostsHandler(app))
	mux.HandleFunc("GET /api/v1/posts/youtube/{channel}", youtubePostsHandler(app))
	mux.HandleFunc("POST /api/v1/posts/telegram/{channel}/refresh", refreshTelegramPostsHandler(app))
	mux.HandleFunc("POST /api/v1/posts/youtube/{channel}/refresh", refreshYouTubePostsHandler(app))
	mux.HandleFunc("GET /api/v1/posts/favorites/telegram", favoriteTelegramPostsHandler(app))
	mux.HandleFunc("GET /api/v1/posts/favorites/youtube", favoriteYouTubePostsHandler(app))

	mux.HandleFunc("GET /api/v1/favorites/telegram", telegramFavoritesHandler(app))
	mux.HandleFunc("GET /api/v1/favorites/telegram/categories", telegramFavoritesWithCategoriesHandler(app))
	mux.HandleFunc("PUT /api/v1/favorites/telegram/{channel}", addTelegramFavoriteHandler(app))
	mux.HandleFunc("DELETE /api/v1/favorites/telegram/{channel}", removeTelegramFavoriteHandler(app))
	mux.HandleFunc("PUT /api/v1/favorites/telegram/{channel}/category", assignTelegramFavoriteCategoryHandler(app))
	mux.HandleFunc("GET /api/v1/favorites/youtube", youtubeFavoritesHandler(app))
	mux.HandleFunc("GET /api/v1/favorites/youtube/categories", youtubeFavoritesWithCategoriesHandler(app))
	mux.HandleFunc("PUT /api/v1/favorites/youtube/{channel}", addYouTubeFavoriteHandler(app))
	mux.HandleFunc("DELETE /api/v1/favorites/youtube/{channel}", removeYouTubeFavoriteHandler(app))
	mux.HandleFunc("PUT /api/v1/favorites/youtube/{channel}/category", assignYouTubeFavoriteCategoryHandler(app))
	mux.HandleFunc("GET /api/v1/favorite-categories", favoriteCategoriesHandler(app))
	mux.HandleFunc("POST /api/v1/favorite-categories", createFavoriteCategoryHandler(app))
	mux.HandleFunc("PUT /api/v1/favorite-categories/{id}", renameFavoriteCategoryHandler(app))
	mux.HandleFunc("DELETE /api/v1/favorite-categories/{id}", deleteFavoriteCategoryHandler(app))
	mux.HandleFunc("PUT /api/v1/favorite-categories/reorder", reorderFavoriteCategoriesHandler(app))

	mux.HandleFunc("GET /api/v1/tasks", tasksHandler(app))
	mux.HandleFunc("POST /api/v1/tasks", createTaskHandler(app))

	mux.HandleFunc("GET /api/v1/tasks/today", todayTasksHandler(app))
	mux.HandleFunc("GET /api/v1/tasks/week", thisWeekTasksHandler(app))

	mux.HandleFunc("PUT /api/v1/tasks/{id}", updateTaskHandler(app))
	mux.HandleFunc("POST /api/v1/tasks/{id}/toggle", toggleTaskHandler(app))
	mux.HandleFunc("POST /api/v1/tasks/{id}/subtasks/{subtaskID}/toggle", toggleTaskSubtaskHandler(app))
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", deleteTaskHandler(app))

	mux.HandleFunc("GET /api/v1/notes", notesHandler(app))
	mux.HandleFunc("POST /api/v1/notes", createNoteHandler(app))
	mux.HandleFunc("GET /api/v1/notes/{id}", noteHandler(app))
	mux.HandleFunc("PUT /api/v1/notes/{id}", updateNoteHandler(app))
	mux.HandleFunc("PUT /api/v1/notes/{id}/pinned", setNotePinnedHandler(app))
	mux.HandleFunc("PUT /api/v1/notes/{id}/archived", setNoteArchivedHandler(app))
	mux.HandleFunc("DELETE /api/v1/notes/{id}", deleteNoteHandler(app))
	mux.HandleFunc("GET /api/v1/notes/{id}/tasks", noteTodosHandler(app))
	mux.HandleFunc("POST /api/v1/notes/{id}/tasks", linkNoteTodoHandler(app))
	mux.HandleFunc("DELETE /api/v1/notes/{id}/tasks/{todoID}", unlinkNoteTodoHandler(app))
	mux.HandleFunc("GET /api/v1/tasks/{id}/notes", todoNotesHandler(app))

	mux.HandleFunc("GET /api/v1/note-topics", noteTopicsHandler(app))
	mux.HandleFunc("POST /api/v1/note-topics", createNoteTopicHandler(app))
	mux.HandleFunc("GET /api/v1/note-topics/{id}", noteTopicHandler(app))
	mux.HandleFunc("PUT /api/v1/note-topics/{id}", updateNoteTopicHandler(app))
	mux.HandleFunc("DELETE /api/v1/note-topics/{id}", deleteNoteTopicHandler(app))
	mux.HandleFunc("GET /api/v1/note-topics/{id}/board", noteTopicBoardHandler(app))
	mux.HandleFunc("GET /api/v1/note-topics/{id}/picker", noteTopicPickerHandler(app))
	mux.HandleFunc("POST /api/v1/note-topics/{id}/blocks", addNoteTopicBlockHandler(app))
	mux.HandleFunc("PUT /api/v1/note-topics/{id}/blocks/positions", updateNoteTopicBlockPositionsHandler(app))
	mux.HandleFunc("DELETE /api/v1/note-topic-blocks/{id}", deleteNoteTopicBlockHandler(app))
	mux.HandleFunc("POST /api/v1/note-topics/{id}/connections", createNoteTopicConnectionHandler(app))
	mux.HandleFunc("DELETE /api/v1/note-topic-connections/{id}", deleteNoteTopicConnectionHandler(app))

	mux.HandleFunc("GET /api/v1/bookmarks", bookmarksHandler(app))
	mux.HandleFunc("POST /api/v1/bookmarks", createBookmarkHandler(app))
	mux.HandleFunc("GET /api/v1/bookmarks/tags", bookmarkTagsHandler(app))
	mux.HandleFunc("GET /api/v1/bookmarks/{id}", bookmarkHandler(app))
	mux.HandleFunc("PUT /api/v1/bookmarks/{id}", updateBookmarkHandler(app))
	mux.HandleFunc("PUT /api/v1/bookmarks/{id}/read", setBookmarkReadHandler(app))
	mux.HandleFunc("DELETE /api/v1/bookmarks/{id}", deleteBookmarkHandler(app))

	mux.HandleFunc("GET /api/v1/weather", weatherHandler(app))
	mux.HandleFunc("PUT /api/v1/weather", saveWeatherLocationHandler(app))
	mux.HandleFunc("GET /api/v1/weather/stored", storedWeatherHandler(app))
	mux.HandleFunc("POST /api/v1/weather/stored/refresh", refreshStoredWeatherHandler(app))

	mux.HandleFunc("GET /api/v1/currencies", currenciesHandler(app))
	mux.HandleFunc("GET /api/v1/currencies/rate", currencyRateHandler(app))
	mux.HandleFunc("GET /api/v1/currencies/favorites", currencyFavoritesHandler(app))
	mux.HandleFunc("PUT /api/v1/currencies/favorites/{base}/{target}", addCurrencyFavoriteHandler(app))
	mux.HandleFunc("DELETE /api/v1/currencies/favorites/{base}/{target}", removeCurrencyFavoriteHandler(app))
	mux.HandleFunc("GET /api/v1/currencies/favorites/rates", currencyFavoritesWithRatesHandler(app))

	mux.HandleFunc("GET /api/v1/desktop-apps", desktopAppsHandler(app))
	mux.HandleFunc("PUT /api/v1/desktop-apps/{id}/name", renameDesktopAppHandler(app))
	mux.HandleFunc("POST /api/v1/desktop-apps/{id}/launch", launchDesktopAppHandler(app, appLauncher))

	mux.HandleFunc("GET /api/v1/setups", setupsHandler(app))
	mux.HandleFunc("POST /api/v1/setups", createSetupHandler(app))
	mux.HandleFunc("PUT /api/v1/setups/{id}", updateSetupHandler(app))
	mux.HandleFunc("DELETE /api/v1/setups/{id}", deleteSetupHandler(app))
	mux.HandleFunc("POST /api/v1/setups/{id}/start", startSetupHandler(app, appLauncher))

	mux.HandleFunc("GET /api/v1/steam-games", steamGamesHandler(app))
	mux.HandleFunc("POST /api/v1/steam-games", addSteamGameHandler(app))
	mux.HandleFunc("GET /api/v1/steam-games/settings", steamGameSettingsHandler(app))
	mux.HandleFunc("PUT /api/v1/steam-games/settings", setSteamGameCountryHandler(app))
	mux.HandleFunc("GET /api/v1/steam-games/countries", steamCountriesHandler(app))
	mux.HandleFunc("POST /api/v1/steam-games/refresh", refreshSteamGamesHandler(app))
	mux.HandleFunc("DELETE /api/v1/steam-games/{id}", deleteSteamGameHandler(app))

	mux.HandleFunc("GET /api/v1/wallpapers", wallpaperSettingsHandler(app))
	mux.HandleFunc("PUT /api/v1/wallpapers/selection", selectWallpaperHandler(app))
	mux.HandleFunc("DELETE /api/v1/wallpapers/{id}", deleteWallpaperHandler(app))

	return operationMiddleware(app, mux)
}

func operationMiddleware(app *backend.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}
		requestContext, cancel := context.WithTimeout(r.Context(), policy.APIRequestTimeout)
		defer cancel()
		operationContext, done, err := app.BeginOperation(requestContext)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "backend_not_ready", "The application backend is not ready.")
			return
		}
		defer done()
		next.ServeHTTP(w, r.WithContext(operationContext))
	})
}

func backendReady(w http.ResponseWriter, app *backend.Service) bool {
	if app == nil {
		writeError(w, http.StatusServiceUnavailable, "backend_unavailable", "The application backend is unavailable.")
		return false
	}

	status := app.GetStartupStatus()
	if !status.Ready {
		writeError(w, http.StatusServiceUnavailable, "backend_not_ready", "The application backend is not ready.")
		return false
	}
	return true
}

func validDate(value string) bool {
	_, err := backend.NormalizeDate(value, true)
	return err == nil
}

func hasTodayWeather(daily backend.WeatherDaily) bool {
	return len(daily.Time) > 0 &&
		len(daily.WeatherCode) > 0 &&
		len(daily.Temperature2MMax) > 0 &&
		len(daily.Temperature2MMin) > 0 &&
		len(daily.PrecipitationProbabilityMax) > 0
}

func sortPostsNewestFirst(items []postResponse) {
	sort.SliceStable(items, func(i, j int) bool {
		return timeAfter(items[i].PostedAt, items[j].PostedAt)
	})
}

func limitPosts(items []postResponse) []postResponse {
	if len(items) <= maximumPostCount {
		return items
	}
	return items[:maximumPostCount]
}

func timeAfter(left string, right string) bool {
	leftTime, leftErr := time.Parse(time.RFC3339, left)
	rightTime, rightErr := time.Parse(time.RFC3339, right)
	if leftErr == nil && rightErr == nil {
		return leftTime.After(rightTime)
	}
	return left > right
}

func parseCurrencySymbols(value string) ([]string, error) {
	parts := strings.Split(value, ",")
	symbols := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		symbol := strings.ToUpper(strings.TrimSpace(part))
		if !validCurrencyCode(symbol) {
			return nil, fmt.Errorf("invalid currency code")
		}
		if _, exists := seen[symbol]; exists {
			continue
		}

		seen[symbol] = struct{}{}
		symbols = append(symbols, symbol)
	}

	if len(symbols) == 0 {
		return nil, fmt.Errorf("at least one currency symbol is required")
	}
	return symbols, nil
}

func validCurrencyCode(value string) bool {
	_, err := backend.NormalizeCurrencyCode(value)
	return err == nil
}
