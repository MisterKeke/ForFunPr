package api

import (
	"net/http"
	"strings"

	"currency-wails/backend"
)

func telegramFavoritesHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !backendReady(w, app) {
			return
		}

		favorites, err := app.ListTelegramFavorites()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "telegram_favorites_failed", "Telegram favourites could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"favorites": favorites})
	}
}

func telegramFavoritesWithCategoriesHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !backendReady(w, app) {
			return
		}

		favorites, err := app.ListTelegramFavoritesWithCategories()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "telegram_favorite_categories_failed", "Categorized Telegram favourites could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"favorites": favorites})
	}
}

func youtubeFavoritesHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !backendReady(w, app) {
			return
		}

		favorites, err := app.ListYouTubeFavorites()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "youtube_favorites_failed", "YouTube favourites could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"favorites": favorites})
	}
}

func youtubeFavoritesWithCategoriesHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !backendReady(w, app) {
			return
		}

		favorites, err := app.ListYouTubeFavoritesWithCategories()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "youtube_favorite_categories_failed", "Categorized YouTube favourites could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"favorites": favorites})
	}
}

func favoriteCategoriesHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		source := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
		if source == "" {
			source = "telegram"
		}
		if source != "telegram" && source != "youtube" {
			writeError(w, http.StatusBadRequest, "invalid_source", "Source must be telegram or youtube.")
			return
		}

		categories, err := app.ListFavoriteCategories(source)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "favorite_categories_failed", "Favourite categories could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"categories": categories})
	}
}
