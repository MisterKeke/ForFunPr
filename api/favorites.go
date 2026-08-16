package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	backend "something/backend/service"
)

type favoriteCategoryAssignmentRequest struct {
	CategoryID int `json:"category_id"`
}

type createFavoriteCategoryRequest struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

type renameFavoriteCategoryRequest struct {
	Name string `json:"name"`
}

func telegramFavoritesHandler(app *backend.Service) http.HandlerFunc {
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

func addTelegramFavoriteHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		channel, ok := telegramFavoriteChannelFromPath(w, r)
		if !ok {
			return
		}

		before, err := app.ListTelegramFavorites()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "telegram_favorites_failed", "Telegram favourites could not be loaded.")
			return
		}

		favorites, err := app.AddTelegramFavorite(channel)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "telegram_favorite_add_failed", "Telegram favourite could not be added.")
			return
		}

		status := http.StatusOK
		if len(favorites) > len(before) {
			status = http.StatusCreated
		}
		writeJSON(w, status, map[string]any{"favorites": favorites})
	}
}

func removeTelegramFavoriteHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		channel, ok := telegramFavoriteChannelFromPath(w, r)
		if !ok {
			return
		}

		if _, err := app.RemoveTelegramFavorite(channel); err != nil {
			writeError(w, http.StatusInternalServerError, "telegram_favorite_remove_failed", "Telegram favourite could not be removed.")
			return
		}

		writeNoContent(w)
	}
}

func assignTelegramFavoriteCategoryHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		channel, ok := telegramFavoriteChannelFromPath(w, r)
		if !ok {
			return
		}

		var request favoriteCategoryAssignmentRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		if !validFavoriteCategoryID(w, request.CategoryID) {
			return
		}
		if err := app.AssignTelegramFavoriteCategory(channel, request.CategoryID); err != nil {
			writeFavoriteAssignmentError(w, err, "telegram")
			return
		}

		writeNoContent(w)
	}
}

func telegramFavoritesWithCategoriesHandler(app *backend.Service) http.HandlerFunc {
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

func youtubeFavoritesHandler(app *backend.Service) http.HandlerFunc {
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

func addYouTubeFavoriteHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		channel, isChannelID, ok := youTubeFavoriteChannelFromPath(w, r)
		if !ok {
			return
		}

		before, err := app.ListYouTubeFavorites()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "youtube_favorites_failed", "YouTube favourites could not be loaded.")
			return
		}

		favorites, err := app.AddYouTubeFavoriteContext(r.Context(), channel)
		if err != nil {
			writeYouTubeFavoriteMutationError(
				w,
				isChannelID,
				"youtube_favorite_add_failed",
				"YouTube favourite could not be added.",
			)
			return
		}

		status := http.StatusOK
		if len(favorites) > len(before) {
			status = http.StatusCreated
		}
		writeJSON(w, status, map[string]any{"favorites": favorites})
	}
}

func removeYouTubeFavoriteHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		channel, isChannelID, ok := youTubeFavoriteChannelFromPath(w, r)
		if !ok {
			return
		}

		if _, err := app.RemoveYouTubeFavoriteContext(r.Context(), channel); err != nil {
			writeYouTubeFavoriteMutationError(
				w,
				isChannelID,
				"youtube_favorite_remove_failed",
				"YouTube favourite could not be removed.",
			)
			return
		}

		writeNoContent(w)
	}
}

func assignYouTubeFavoriteCategoryHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		channel, _, ok := youTubeFavoriteChannelFromPath(w, r)
		if !ok {
			return
		}

		var request favoriteCategoryAssignmentRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		if !validFavoriteCategoryID(w, request.CategoryID) {
			return
		}
		if err := app.AssignYouTubeFavoriteCategoryContext(r.Context(), channel, request.CategoryID); err != nil {
			writeFavoriteAssignmentError(w, err, "youtube")
			return
		}

		writeNoContent(w)
	}
}

func youtubeFavoritesWithCategoriesHandler(app *backend.Service) http.HandlerFunc {
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

func createFavoriteCategoryHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		var request createFavoriteCategoryRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}

		request.Name = strings.TrimSpace(request.Name)
		request.Source = strings.ToLower(strings.TrimSpace(request.Source))
		if request.Name == "" {
			writeError(w, http.StatusUnprocessableEntity, "invalid_category_name", "Category name cannot be empty.")
			return
		}
		if !validFavoriteSource(request.Source) {
			writeError(w, http.StatusUnprocessableEntity, "invalid_source", "Source must be telegram or youtube.")
			return
		}

		category, created, err := app.CreateFavoriteCategoryWithStatus(request.Name, request.Source)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "favorite_category_create_failed", "Favourite category could not be created.")
			return
		}

		status := http.StatusCreated
		if !created {
			status = http.StatusOK
		} else {
			app.EmitFavoriteCategoriesChanged(category.Source)
		}
		writeJSON(w, status, category)
	}
}

func favoriteCategoriesHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		source := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
		if source == "" {
			source = "telegram"
		}
		if !validFavoriteSource(source) {
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

func renameFavoriteCategoryHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		id, err := strconv.Atoi(strings.TrimSpace(r.PathValue("id")))
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_category_id", "Category ID must be a positive integer.")
			return
		}

		var request renameFavoriteCategoryRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		request.Name = strings.TrimSpace(request.Name)
		if request.Name == "" {
			writeError(w, http.StatusUnprocessableEntity, "invalid_category_name", "Category name cannot be empty.")
			return
		}

		category, err := app.RenameFavoriteCategory(id, request.Name)
		if err != nil {
			var notFound *backend.NotFoundError
			var conflict *backend.ConflictError
			var validation *backend.ValidationError
			switch {
			case errors.As(err, &notFound):
				writeError(w, http.StatusNotFound, "favorite_category_not_found", "The requested favourite category does not exist.")
			case errors.As(err, &conflict):
				writeError(w, http.StatusConflict, "favorite_category_name_conflict", "A favourite category with that name already exists for this source.")
			case errors.As(err, &validation):
				writeError(w, http.StatusUnprocessableEntity, "invalid_category_name", validation.Message)
			default:
				writeError(w, http.StatusInternalServerError, "favorite_category_rename_failed", "Favourite category could not be renamed.")
			}
			return
		}
		app.EmitFavoriteCategoriesChanged(category.Source)
		writeJSON(w, http.StatusOK, category)
	}
}

func telegramFavoriteChannelFromPath(w http.ResponseWriter, r *http.Request) (string, bool) {
	channel, err := backend.NormalizeTelegramUsername(r.PathValue("channel"))
	if err == nil {
		return channel, true
	}
	writeError(w, http.StatusBadRequest, "invalid_telegram_channel", "Telegram channel must be a valid username.")
	return "", false
}

func youTubeFavoriteChannelFromPath(w http.ResponseWriter, r *http.Request) (string, bool, bool) {
	channel, isChannelID, err := backend.NormalizeYouTubeReference(r.PathValue("channel"))
	if err == nil {
		return channel, isChannelID, true
	}
	writeError(w, http.StatusBadRequest, "invalid_youtube_channel", "YouTube channel must be a valid channel ID or handle.")
	return "", false, false
}

func validFavoriteCategoryID(w http.ResponseWriter, categoryID int) bool {
	if categoryID <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "invalid_category_id", "Category ID must be a positive integer.")
		return false
	}
	return true
}

func validFavoriteSource(source string) bool {
	_, err := backend.NormalizeFavoriteSource(source)
	return err == nil
}

func writeYouTubeFavoriteMutationError(
	w http.ResponseWriter,
	isChannelID bool,
	code string,
	message string,
) {
	status := http.StatusBadGateway
	if isChannelID {
		status = http.StatusInternalServerError
	}
	writeError(w, status, code, message)
}

func writeFavoriteAssignmentError(w http.ResponseWriter, err error, source string) {
	var notFound *backend.NotFoundError
	if errors.As(err, &notFound) {
		code := source + "_favorite_not_found"
		message := "The requested favourite does not exist."
		if strings.Contains(notFound.Resource, "category") {
			code = "favorite_category_not_found"
			message = "The requested favourite category does not exist for this source."
		}
		writeError(w, http.StatusNotFound, code, message)
		return
	}
	var validation *backend.ValidationError
	if errors.As(err, &validation) {
		writeError(w, http.StatusUnprocessableEntity, "invalid_favorite_assignment", validation.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, source+"_favorite_category_assign_failed", "The favourite category could not be assigned.")
}
