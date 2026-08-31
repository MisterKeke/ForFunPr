package api

import (
	"errors"
	"net/http"

	backend "something/backend/service"
)

type addSteamGameRequest struct {
	StoreURL string `json:"store_url"`
}

type steamCountryRequest struct {
	CountryCode string `json:"country_code"`
}

func steamGamesHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		items, err := app.ListSteamGamesContext(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "steam_games_failed", "Steam games could not be loaded.")
			return
		}
		writeJSON(w, http.StatusOK, items)
	}
}

func addSteamGameHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		var request addSteamGameRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		item, err := app.AddSteamGameContext(r.Context(), request.StoreURL)
		if err != nil {
			var validation *backend.ValidationError
			var conflict *backend.ConflictError
			switch {
			case errors.As(err, &validation):
				writeError(w, http.StatusUnprocessableEntity, "invalid_steam_game", validation.Message)
			case errors.As(err, &conflict):
				writeError(w, http.StatusConflict, "steam_game_conflict", conflict.Message)
			default:
				writeError(w, http.StatusBadGateway, "steam_game_fetch_failed", "The Steam game could not be loaded from Steam.")
			}
			return
		}
		app.EmitSteamGamesChanged()
		writeJSON(w, http.StatusCreated, item)
	}
}

func deleteSteamGameHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		id, ok := parsePositivePathID(w, r, "steam_game")
		if !ok {
			return
		}
		if err := app.DeleteSteamGameContext(r.Context(), id); err != nil {
			writeSteamGameError(w, err, "steam_game_delete_failed", "The Steam game could not be deleted.")
			return
		}
		app.EmitSteamGamesChanged()
		writeNoContent(w)
	}
}

func steamGameSettingsHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		settings, err := app.GetSteamGameSettingsContext(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "steam_settings_failed", "Steam settings could not be loaded.")
			return
		}
		writeJSON(w, http.StatusOK, settings)
	}
}

func setSteamGameCountryHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		var request steamCountryRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		settings, err := app.SetSteamGameCountryContext(r.Context(), request.CountryCode)
		if err != nil {
			writeSteamGameError(w, err, "steam_country_failed", "The Steam country could not be saved.")
			return
		}
		app.EmitSteamGamesChanged()
		writeJSON(w, http.StatusOK, settings)
	}
}

func steamCountriesHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !backendReady(w, app) {
			return
		}
		writeJSON(w, http.StatusOK, backend.ListSupportedSteamCountries())
	}
}

func refreshSteamGamesHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}
		var request struct{}
		if !decodeJSONBody(w, r, &request) {
			return
		}
		result, err := app.RefreshSteamGamesContext(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "steam_refresh_failed", "Steam game prices could not be refreshed.")
			return
		}
		if len(result.Errors) > 0 {
			w.Header().Set("X-Partial-Result", "true")
		}
		app.EmitSteamGamesChanged()
		writeJSON(w, http.StatusOK, result)
	}
}

func writeSteamGameError(w http.ResponseWriter, err error, fallbackCode string, fallbackMessage string) {
	var validation *backend.ValidationError
	var notFound *backend.NotFoundError
	switch {
	case errors.As(err, &validation):
		writeError(w, http.StatusUnprocessableEntity, "invalid_steam_game", validation.Message)
	case errors.As(err, &notFound):
		writeError(w, http.StatusNotFound, "steam_game_not_found", "The requested Steam game does not exist.")
	default:
		writeError(w, http.StatusInternalServerError, fallbackCode, fallbackMessage)
	}
}
