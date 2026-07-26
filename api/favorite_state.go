package api

import (
	"net/http"

	"currency-wails/backend"
)

func favoriteUpdateStateHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !backendReady(w, app) {
			return
		}

		state, err := app.GetFavoriteUpdateState()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "news_state_failed", "The news update state could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, state)
	}
}

func updateWindowsHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !backendReady(w, app) {
			return
		}

		windows, err := app.GetUpdateWindows()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "news_windows_failed", "The news update windows could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, windows)
	}
}
