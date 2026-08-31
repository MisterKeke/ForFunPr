package api

import (
	"errors"
	"net/http"
	"strings"

	backend "something/backend/service"
)

type desktopAppResponse struct {
	ID          int    `json:"id"`
	DisplayName string `json:"display_name"`
	IconURL     string `json:"icon_url,omitempty"`
	Available   bool   `json:"available"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type renameDesktopAppRequest struct {
	DisplayName string `json:"display_name"`
}

func desktopAppResponseFromService(app backend.DesktopApp) desktopAppResponse {
	return desktopAppResponse{
		ID: app.ID, DisplayName: app.DisplayName, IconURL: app.IconURL,
		Available: app.Available, CreatedAt: app.CreatedAt, UpdatedAt: app.UpdatedAt,
	}
}

func desktopAppsHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		items, err := app.ListDesktopAppsContext(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "desktop_apps_failed", "Desktop applications could not be loaded.")
			return
		}
		result := make([]desktopAppResponse, 0, len(items))
		for _, item := range items {
			result = append(result, desktopAppResponseFromService(item))
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func renameDesktopAppHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		id, ok := parsePositivePathID(w, r, "desktop_app")
		if !ok {
			return
		}
		var request renameDesktopAppRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		request.DisplayName = strings.TrimSpace(request.DisplayName)
		item, err := app.RenameDesktopAppContext(r.Context(), id, request.DisplayName)
		if err != nil {
			var validation *backend.ValidationError
			var notFound *backend.NotFoundError
			switch {
			case errors.As(err, &validation):
				writeError(w, http.StatusUnprocessableEntity, "invalid_desktop_app_name", validation.Message)
			case errors.As(err, &notFound):
				writeError(w, http.StatusNotFound, "desktop_app_not_found", "The requested desktop application does not exist.")
			default:
				writeError(w, http.StatusInternalServerError, "desktop_app_rename_failed", "The desktop application could not be renamed.")
			}
			return
		}
		app.EmitDesktopAppsChanged()
		writeJSON(w, http.StatusOK, desktopAppResponseFromService(item))
	}
}
