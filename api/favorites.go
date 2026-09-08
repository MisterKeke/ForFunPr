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
	Name         string `json:"name"`
	Source       string `json:"source"`
	Color        string `json:"color"`
	DisplayOrder *int   `json:"display_order,omitempty"`
}

type renameFavoriteCategoryRequest struct {
	Name         string  `json:"name"`
	Color        *string `json:"color,omitempty"`
	DisplayOrder *int    `json:"display_order,omitempty"`
}

type deleteFavoriteCategoryRequest struct {
	Mode             string `json:"mode"`
	TargetCategoryID *int   `json:"target_category_id,omitempty"`
}

type reorderFavoriteCategoriesRequest struct {
	Source      string `json:"source"`
	CategoryIDs []int  `json:"category_ids"`
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

		result, err := app.CreateFavoriteCategoryContext(r.Context(), backend.FavoriteCategoryWriteRequest{
			Name: request.Name, Source: request.Source, Color: request.Color, DisplayOrder: request.DisplayOrder,
		})
		if err != nil {
			writeFavoriteCategoryMutationError(w, err, "favorite_category_create_failed", "Favourite category could not be created.")
			return
		}

		status := http.StatusCreated
		if !result.Changed {
			status = http.StatusOK
		} else {
			app.EmitFavoriteCategoriesChanged(result.Category.Source)
		}
		writeJSON(w, status, result)
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

		categories, err := app.ListFavoriteCategoriesContext(r.Context(), source)
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

		current, err := app.GetFavoriteCategoryContext(r.Context(), id)
		if err != nil {
			writeFavoriteCategoryMutationError(w, err, "favorite_category_update_failed", "Favourite category could not be updated.")
			return
		}
		color := current.Color
		if request.Color != nil {
			color = *request.Color
		}
		result, err := app.UpdateFavoriteCategoryContext(r.Context(), id, backend.FavoriteCategoryWriteRequest{
			Name: request.Name, Color: color, DisplayOrder: request.DisplayOrder,
		})
		if err != nil {
			writeFavoriteCategoryMutationError(w, err, "favorite_category_update_failed", "Favourite category could not be updated.")
			return
		}
		if result.Changed {
			app.EmitFavoriteCategoriesChanged(result.Category.Source)
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func deleteFavoriteCategoryHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		id, err := strconv.Atoi(strings.TrimSpace(r.PathValue("id")))
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_category_id", "Category ID must be a positive integer.")
			return
		}
		var request deleteFavoriteCategoryRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		result, err := app.DeleteFavoriteCategoryContext(r.Context(), backend.FavoriteCategoryDeleteRequest{
			ID: id, Mode: request.Mode, TargetCategoryID: request.TargetCategoryID,
		})
		if err != nil {
			writeFavoriteCategoryMutationError(w, err, "favorite_category_delete_failed", "Favourite category could not be deleted.")
			return
		}
		app.EmitFavoriteCategoriesChanged(result.Source)
		writeJSON(w, http.StatusOK, result)
	}
}

func reorderFavoriteCategoriesHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		var request reorderFavoriteCategoriesRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		items, err := app.ReorderFavoriteCategoriesContext(r.Context(), backend.FavoriteCategoryReorderRequest{
			Source: request.Source, CategoryIDs: request.CategoryIDs,
		})
		if err != nil {
			writeFavoriteCategoryMutationError(w, err, "favorite_category_reorder_failed", "Favourite categories could not be reordered.")
			return
		}
		app.EmitFavoriteCategoriesChanged(request.Source)
		writeJSON(w, http.StatusOK, map[string]any{"categories": items})
	}
}

func writeFavoriteCategoryMutationError(w http.ResponseWriter, err error, code, message string) {
	var notFound *backend.NotFoundError
	var conflict *backend.ConflictError
	var validation *backend.ValidationError
	switch {
	case errors.As(err, &notFound):
		writeError(w, http.StatusNotFound, "favorite_category_not_found", "The requested favourite category does not exist.")
	case errors.As(err, &conflict):
		writeError(w, http.StatusConflict, "favorite_category_conflict", conflict.Error())
	case errors.As(err, &validation):
		writeError(w, http.StatusUnprocessableEntity, "invalid_favorite_category", validation.Message)
	default:
		writeError(w, http.StatusInternalServerError, code, message)
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
