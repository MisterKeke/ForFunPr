package api

import (
	"fmt"
	"net/http"
	"strings"

	"currency-wails/backend"
)

type currenciesResponse struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

func currenciesHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		base := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("base")))
		if !validCurrencyCode(base) {
			writeError(w, http.StatusBadRequest, "invalid_base_currency", "Base must be a three-letter currency code.")
			return
		}

		allRates, err := app.GetAllRates(base)
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

func currencyRateHandler(app *backend.App) http.HandlerFunc {
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

		result, err := app.GetRate(base, target)
		if err != nil {
			writeError(w, http.StatusBadGateway, "currency_rate_failed", "The currency rate could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func currencyFavoritesHandler(app *backend.App) http.HandlerFunc {
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

func currencyFavoritesWithRatesHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !backendReady(w, app) {
			return
		}

		result, err := app.GetFavoritesWithRates()
		if err != nil {
			writeError(w, http.StatusBadGateway, "currency_favorite_rates_failed", "Favourite currency rates could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}
