package api

import (
	"net/http"

	"currency-wails/backend"
)

func healthHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if app == nil {
			writeError(w, http.StatusServiceUnavailable, "backend_unavailable", "The application backend is unavailable.")
			return
		}

		status := app.GetStartupStatus()
		httpStatus := http.StatusOK
		if !status.Ready {
			httpStatus = http.StatusServiceUnavailable
		}

		writeJSON(w, httpStatus, status)
	}
}
