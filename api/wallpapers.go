package api

import (
	"errors"
	"net/http"
	"strings"

	backend "something/backend/service"
)

type wallpaperSelectionRequest struct {
	Selection string `json:"selection"`
}

type userWallpaperResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	MIMEType    string `json:"mime_type"`
	ByteSize    int64  `json:"byte_size"`
	URL         string `json:"url"`
}

type wallpaperSettingsResponse struct {
	Selected          string                  `json:"selected"`
	SelectionSaved    bool                    `json:"selection_saved"`
	BuiltinSelections []string                `json:"builtin_selections"`
	UserWallpapers    []userWallpaperResponse `json:"user_wallpapers"`
}

func wallpaperSettingsResponseFromService(settings *backend.WallpaperSettings) wallpaperSettingsResponse {
	result := wallpaperSettingsResponse{BuiltinSelections: backend.ListBuiltinWallpaperSelections(), UserWallpapers: []userWallpaperResponse{}}
	if settings == nil {
		return result
	}
	result.Selected = settings.Selected
	result.SelectionSaved = settings.SelectionSaved
	for _, item := range settings.UserWallpapers {
		result.UserWallpapers = append(result.UserWallpapers, userWallpaperResponse{
			ID: item.ID, DisplayName: item.DisplayName, MIMEType: item.MIMEType,
			ByteSize: item.ByteSize, URL: item.URL,
		})
	}
	return result
}

func wallpaperSettingsHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		settings, err := app.GetWallpaperSettingsContext(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "wallpaper_settings_failed", "Wallpaper settings could not be loaded.")
			return
		}
		writeJSON(w, http.StatusOK, wallpaperSettingsResponseFromService(settings))
	}
}

func selectWallpaperHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		var request wallpaperSelectionRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		settings, err := app.SelectWallpaperContext(r.Context(), strings.TrimSpace(request.Selection))
		if err != nil {
			writeWallpaperError(w, err, "wallpaper_select_failed", "The wallpaper could not be selected.")
			return
		}
		app.EmitWallpapersChanged()
		writeJSON(w, http.StatusOK, wallpaperSettingsResponseFromService(settings))
	}
}

func deleteWallpaperHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		id := strings.TrimSpace(r.PathValue("id"))
		settings, err := app.DeleteUserWallpaperContext(r.Context(), id)
		if err != nil {
			writeWallpaperError(w, err, "wallpaper_delete_failed", "The wallpaper could not be deleted.")
			return
		}
		app.EmitWallpapersChanged()
		writeJSON(w, http.StatusOK, wallpaperSettingsResponseFromService(settings))
	}
}

func writeWallpaperError(w http.ResponseWriter, err error, fallbackCode string, fallbackMessage string) {
	var validation *backend.ValidationError
	var notFound *backend.NotFoundError
	switch {
	case errors.As(err, &validation):
		writeError(w, http.StatusUnprocessableEntity, "invalid_wallpaper", validation.Message)
	case errors.As(err, &notFound):
		writeError(w, http.StatusNotFound, "wallpaper_not_found", "The requested user wallpaper does not exist.")
	default:
		writeError(w, http.StatusInternalServerError, fallbackCode, fallbackMessage)
	}
}
