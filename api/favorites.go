package api

import (
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"currency-wails/backend"
)

var apiYouTubeChannelIDPattern = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)

type favoriteCategoryAssignmentRequest struct {
	CategoryID int `json:"category_id"`
}

type createFavoriteCategoryRequest struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

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

func addTelegramFavoriteHandler(app *backend.App) http.HandlerFunc {
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

func removeTelegramFavoriteHandler(app *backend.App) http.HandlerFunc {
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

func assignTelegramFavoriteCategoryHandler(app *backend.App) http.HandlerFunc {
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
		if !favoriteCategoryExists(w, app, "telegram", request.CategoryID) {
			return
		}
		if !telegramFavoriteExists(w, app, channel) {
			return
		}

		if err := app.AssignTelegramFavoriteCategory(channel, request.CategoryID); err != nil {
			writeError(w, http.StatusInternalServerError, "telegram_favorite_category_assign_failed", "The Telegram favourite category could not be assigned.")
			return
		}

		writeNoContent(w)
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

func addYouTubeFavoriteHandler(app *backend.App) http.HandlerFunc {
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

		favorites, err := app.AddYouTubeFavorite(channel)
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

func removeYouTubeFavoriteHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		channel, isChannelID, ok := youTubeFavoriteChannelFromPath(w, r)
		if !ok {
			return
		}

		if _, err := app.RemoveYouTubeFavorite(channel); err != nil {
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

func assignYouTubeFavoriteCategoryHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		channel, isChannelID, ok := youTubeFavoriteChannelFromPath(w, r)
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
		if !favoriteCategoryExists(w, app, "youtube", request.CategoryID) {
			return
		}
		if isChannelID && !youTubeFavoriteExists(w, app, channel) {
			return
		}

		if err := app.AssignYouTubeFavoriteCategory(channel, request.CategoryID); err != nil {
			writeYouTubeFavoriteMutationError(
				w,
				isChannelID,
				"youtube_favorite_category_assign_failed",
				"The YouTube favourite category could not be assigned.",
			)
			return
		}

		writeNoContent(w)
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

func createFavoriteCategoryHandler(app *backend.App) http.HandlerFunc {
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

		categories, err := app.ListFavoriteCategories(request.Source)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "favorite_categories_failed", "Favourite categories could not be loaded.")
			return
		}

		alreadyExists := false
		for _, category := range categories {
			if strings.EqualFold(strings.TrimSpace(category.Name), request.Name) {
				alreadyExists = true
				break
			}
		}

		category, err := app.CreateFavoriteCategory(request.Name, request.Source)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "favorite_category_create_failed", "Favourite category could not be created.")
			return
		}

		status := http.StatusCreated
		if alreadyExists {
			status = http.StatusOK
		}
		writeJSON(w, status, category)
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

func telegramFavoriteChannelFromPath(w http.ResponseWriter, r *http.Request) (string, bool) {
	channel := strings.ToLower(strings.TrimSpace(r.PathValue("channel")))
	channel = strings.TrimPrefix(channel, "@")

	if len(channel) < 5 || len(channel) > 32 {
		writeError(w, http.StatusBadRequest, "invalid_telegram_channel", "Telegram channel must be a valid username.")
		return "", false
	}
	for _, character := range channel {
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') &&
			character != '_' {
			writeError(w, http.StatusBadRequest, "invalid_telegram_channel", "Telegram channel must be a valid username.")
			return "", false
		}
	}

	return channel, true
}

func youTubeFavoriteChannelFromPath(w http.ResponseWriter, r *http.Request) (string, bool, bool) {
	channel := strings.TrimSpace(r.PathValue("channel"))
	if apiYouTubeChannelIDPattern.MatchString(channel) {
		return channel, true, true
	}

	handle := strings.TrimPrefix(channel, "@")
	if !utf8.ValidString(handle) {
		writeError(w, http.StatusBadRequest, "invalid_youtube_channel", "YouTube channel must be a valid channel ID or handle.")
		return "", false, false
	}

	length := utf8.RuneCountInString(handle)
	if length < 3 || length > 30 {
		writeError(w, http.StatusBadRequest, "invalid_youtube_channel", "YouTube channel must be a valid channel ID or handle.")
		return "", false, false
	}
	for _, character := range handle {
		if !unicode.IsLetter(character) &&
			!unicode.IsNumber(character) &&
			character != '_' &&
			character != '-' &&
			character != '.' {
			writeError(w, http.StatusBadRequest, "invalid_youtube_channel", "YouTube channel must be a valid channel ID or handle.")
			return "", false, false
		}
	}

	return handle, false, true
}

func validFavoriteCategoryID(w http.ResponseWriter, categoryID int) bool {
	if categoryID <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "invalid_category_id", "Category ID must be a positive integer.")
		return false
	}
	return true
}

func favoriteCategoryExists(w http.ResponseWriter, app *backend.App, source string, categoryID int) bool {
	categories, err := app.ListFavoriteCategories(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "favorite_categories_failed", "Favourite categories could not be loaded.")
		return false
	}

	for _, category := range categories {
		if category.ID == categoryID {
			return true
		}
	}

	writeError(w, http.StatusNotFound, "favorite_category_not_found", "The requested favourite category does not exist for this source.")
	return false
}

func telegramFavoriteExists(w http.ResponseWriter, app *backend.App, channel string) bool {
	favorites, err := app.ListTelegramFavorites()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "telegram_favorites_failed", "Telegram favourites could not be loaded.")
		return false
	}

	for _, favorite := range favorites {
		if favorite == channel {
			return true
		}
	}

	writeError(w, http.StatusNotFound, "telegram_favorite_not_found", "The requested Telegram favourite does not exist.")
	return false
}

func youTubeFavoriteExists(w http.ResponseWriter, app *backend.App, channelID string) bool {
	favorites, err := app.ListYouTubeFavorites()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "youtube_favorites_failed", "YouTube favourites could not be loaded.")
		return false
	}

	for _, favorite := range favorites {
		if favorite == channelID {
			return true
		}
	}

	writeError(w, http.StatusNotFound, "youtube_favorite_not_found", "The requested YouTube favourite does not exist.")
	return false
}

func validFavoriteSource(source string) bool {
	return source == "telegram" || source == "youtube"
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
