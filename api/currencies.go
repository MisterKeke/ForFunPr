package api

import (
	"fmt"
	"net/http"
	"strings"

	backend "something/backend/service"
)

type currenciesResponse struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

func currencyPairFromPath(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	base := strings.ToUpper(strings.TrimSpace(r.PathValue("base")))
	target := strings.ToUpper(strings.TrimSpace(r.PathValue("target")))
	if !validCurrencyCode(base) || !validCurrencyCode(target) {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid_currency_pair",
			"Base and target must be three-letter currency codes.",
		)
		return "", "", false
	}

	return base, target, true
}

func currenciesHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		base := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("base")))
		if !validCurrencyCode(base) {
			writeError(w, http.StatusBadRequest, "invalid_base_currency", "Base must be a three-letter currency code.")
			return
		}

		allRates, err := app.GetAllRatesContext(r.Context(), base)
		if err != nil {
			writeError(w, http.StatusBadGateway, "currencies_failed", "Currency rates could not be loaded.")
			return
		}

		symbolsValue := strings.TrimSpace(r.URL.Query().Get("symbols"))
		if symbolsValue == "" {
			writeJSON(w, http.StatusOK, allRates)
			return
		}

		symbols, err := parseCurrencySymbols(symbolsValue)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_currencies", "Symbols must be comma-separated three-letter currency codes.")
			return
		}

		rates := make(map[string]float64, len(symbols))
		missing := make([]string, 0)
		for _, symbol := range symbols {
			if symbol == base {
				rates[symbol] = 1
				continue
			}

			rate, found := allRates.Rates[symbol]
			if !found {
				missing = append(missing, symbol)
				continue
			}
			rates[symbol] = rate
		}

		if len(missing) > 0 {
			writeError(
				w,
				http.StatusUnprocessableEntity,
				"unsupported_currencies",
				fmt.Sprintf("No rate is available for: %s.", strings.Join(missing, ", ")),
			)
			return
		}

		writeJSON(w, http.StatusOK, currenciesResponse{
			Base:  allRates.Base,
			Date:  allRates.Date,
			Rates: rates,
		})
	}
}

func currencyRateHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		base := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("base")))
		target := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("target")))
		if !validCurrencyCode(base) || !validCurrencyCode(target) {
			writeError(w, http.StatusBadRequest, "invalid_currency_pair", "Base and target must be three-letter currency codes.")
			return
		}

		result, err := app.GetRateContext(r.Context(), base, target)
		if err != nil {
			writeError(w, http.StatusBadGateway, "currency_rate_failed", "The currency rate could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func currencyFavoritesHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !backendReady(w, app) {
			return
		}

		favorites, err := app.ListFavorites()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "currency_favorites_failed", "Currency favourites could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"favorites": favorites})
	}
}

func currencyFavoritesWithRatesHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		result, err := app.GetFavoritesWithRatesContext(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "currency_favorite_rates_failed", "Favourite currency rates could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func addCurrencyFavoriteHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		base, target, ok := currencyPairFromPath(w, r)
		if !ok {
			return
		}
		if base == target {
			writeError(
				w,
				http.StatusUnprocessableEntity,
				"identical_currency_pair",
				"Base and target currencies must be different.",
			)
			return
		}

		result, err := app.AddFavorite(base + ":" + target)
		if err != nil {
			writeError(
				w,
				http.StatusInternalServerError,
				"currency_favorite_add_failed",
				"Currency favourite could not be added.",
			)
			return
		}

		status := http.StatusCreated
		if result.Exists {
			status = http.StatusOK
		}
		writeJSON(w, status, result)
	}
}

func removeCurrencyFavoriteHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		base, target, ok := currencyPairFromPath(w, r)
		if !ok {
			return
		}

		if _, err := app.RemoveFavorite(base + ":" + target); err != nil {
			writeError(
				w,
				http.StatusInternalServerError,
				"currency_favorite_remove_failed",
				"Currency favourite could not be removed.",
			)
			return
		}

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusNoContent)
	}
}
