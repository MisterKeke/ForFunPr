package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"currency-wails/backend"
)

const maximumPostCount = 20

func newRouter(app *backend.App) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", healthHandler(app))

	mux.HandleFunc("GET /api/v1/news", newsHandler(app))
	mux.HandleFunc("POST /api/v1/news/initial", initialFavoriteUpdatesHandler(app))
	mux.HandleFunc("POST /api/v1/news/refresh", refreshFavoriteUpdatesHandler(app))
	mux.HandleFunc("POST /api/v1/news/since-last-open", favoriteUpdatesSinceLastOpenHandler(app))
	mux.HandleFunc("GET /api/v1/news/state", favoriteUpdateStateHandler(app))
	mux.HandleFunc("GET /api/v1/news/windows", updateWindowsHandler(app))

	mux.HandleFunc("GET /api/v1/posts/telegram/{channel}", telegramPostsHandler(app))
	mux.HandleFunc("GET /api/v1/posts/youtube/{channel}", youtubePostsHandler(app))
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

	mux.HandleFunc("GET /api/v1/tasks", tasksHandler(app))
	mux.HandleFunc("POST /api/v1/tasks", createTaskHandler(app))

	mux.HandleFunc("GET /api/v1/tasks/today", todayTasksHandler(app))

	mux.HandleFunc("PUT /api/v1/tasks/{id}", updateTaskHandler(app))
	mux.HandleFunc("POST /api/v1/tasks/{id}/toggle", toggleTaskHandler(app))
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", deleteTaskHandler(app))

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

	return mux
}

func backendReady(w http.ResponseWriter, app *backend.App) bool {
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
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
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
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}
