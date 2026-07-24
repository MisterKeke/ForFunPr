package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// FavoriteRate is one favorite currency pair and its latest rate, when found.
type FavoriteRate struct {
	Code  string  `json:"code"`
	Base  string  `json:"base"`
	To    string  `json:"to"`
	Rate  float64 `json:"rate"`
	Found bool    `json:"found"`
}

// FavoritesWithRatesResult is the typed response from GetFavoritesWithRates.
// Wails serializes this struct directly for the JavaScript bridge.
type FavoritesWithRatesResult struct {
	Base      string         `json:"base"`
	Favorites []FavoriteRate `json:"favorites"`
}

type frankfurterRateResponse struct {
	Date  string  `json:"date"`
	Base  string  `json:"base"`
	Quote string  `json:"quote"`
	Rate  float64 `json:"rate"`
}

type RateResult struct {
	Base  string  `json:"base"`
	Date  string  `json:"date"`
	To    string  `json:"to"`
	Rate  float64 `json:"rate"`
	Found bool    `json:"found"`
}

type AllRatesResult struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
	Codes []string           `json:"codes"`
}

type AddFavoriteResult struct {
	Pair   string `json:"pair"`
	Added  bool   `json:"added"`
	Exists bool   `json:"exists"`
	Error  string `json:"error,omitempty"`
}

func (a *App) GetRate(base string, target string) (*RateResult, error) {
	base = normalizeCurrency(base)
	target = normalizeCurrency(target)
	if base == "" || target == "" {
		return nil, fmt.Errorf("base and target currency codes must be three ASCII letters")
	}

	return a.getRate(a.requestContext(), base, target)
}

func (a *App) getRate(ctx context.Context, base string, target string) (*RateResult, error) {
	if base == target {
		return &RateResult{Base: base, Date: "", To: target, Rate: 1.0, Found: true}, nil
	}

	body, status, err := a.httpClient.get(
		ctx,
		providerFrankfurter,
		frankfurterRateURL(base, target),
		nil,
		http.StatusOK,
		http.StatusNotFound,
	)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return &RateResult{Base: base, To: target, Found: false}, nil
	}

	var data frankfurterRateResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("Frankfurter response contained invalid JSON: %w", err)
	}
	if data.Base != base || data.Quote != target {
		return nil, fmt.Errorf("Frankfurter response did not match the requested currency pair")
	}

	return &RateResult{
		Base:  data.Base,
		Date:  data.Date,
		To:    data.Quote,
		Rate:  data.Rate,
		Found: true,
	}, nil
}

func (a *App) GetAllRates(base string) (*AllRatesResult, error) {
	base = normalizeCurrency(base)
	if base == "" {
		return nil, fmt.Errorf("base currency code must be three ASCII letters")
	}

	body, _, err := a.httpClient.get(
		a.requestContext(),
		providerFrankfurter,
		frankfurterRatesURL(base),
		nil,
		http.StatusOK,
	)
	if err != nil {
		return nil, err
	}

	var list []frankfurterRateResponse
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("Frankfurter response contained invalid JSON: %w", err)
	}

	if len(list) == 0 {
		return &AllRatesResult{Base: base, Rates: make(map[string]float64), Codes: []string{}}, nil
	}

	ratesMap := make(map[string]float64)
	codes := make([]string, 0, len(list))
	date := list[0].Date

	for _, item := range list {
		ratesMap[item.Quote] = item.Rate
		codes = append(codes, item.Quote)
	}
	sort.Strings(codes)

	return &AllRatesResult{
		Base:  base,
		Date:  date,
		Rates: ratesMap,
		Codes: codes,
	}, nil
}

func frankfurterRateURL(base string, target string) *url.URL {
	return (&url.URL{
		Scheme: "https",
		Host:   "api.frankfurter.dev",
	}).JoinPath("v2", "rate", base, target)
}

func frankfurterRatesURL(base string) *url.URL {
	endpoint := (&url.URL{
		Scheme: "https",
		Host:   "api.frankfurter.dev",
	}).JoinPath("v2", "rates")
	query := endpoint.Query()
	query.Set("base", base)
	endpoint.RawQuery = query.Encode()
	return endpoint
}

func normalizeFavoritePair(value string) (string, string, bool) {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(value)), ":")
	if len(parts) != 2 {
		return "", "", false
	}

	base := normalizeCurrency(parts[0])
	quote := normalizeCurrency(parts[1])
	if base == "" || quote == "" {
		return "", "", false
	}

	return base, quote, true
}

func (a *App) AddFavorite(name string) (AddFavoriteResult, error) {
	base, quote, ok := normalizeFavoritePair(name)
	if !ok {
		return AddFavoriteResult{}, fmt.Errorf("invalid currency pair format")
	}
	if base == quote {
		return AddFavoriteResult{}, fmt.Errorf("favorite currencies must be different")
	}

	result, err := a.db.Exec(
		`INSERT OR IGNORE INTO favorite_rates (base, quote) VALUES (?, ?)`,
		base, quote,
	)
	if err != nil {
		return AddFavoriteResult{}, fmt.Errorf("add favorite rate %s:%s: %w", base, quote, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return AddFavoriteResult{}, fmt.Errorf("check added favorite rate %s:%s: %w", base, quote, err)
	}
	if affected < 0 || affected > 1 {
		return AddFavoriteResult{}, fmt.Errorf(
			"add favorite rate %s:%s affected %d rows",
			base,
			quote,
			affected,
		)
	}

	return AddFavoriteResult{
		Pair:   base + ":" + quote,
		Added:  affected == 1,
		Exists: affected == 0,
	}, nil
}

func (a *App) RemoveFavorite(name string) (string, error) {
	base, quote, ok := normalizeFavoritePair(name)
	if !ok {
		return "", fmt.Errorf("invalid currency pair format")
	}

	result, err := a.db.Exec(
		`DELETE FROM favorite_rates WHERE base = ? AND quote = ?`,
		base,
		quote,
	)
	if err != nil {
		return "", fmt.Errorf("remove favorite rate %s:%s: %w", base, quote, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("check removed favorite rate %s:%s: %w", base, quote, err)
	}
	if affected < 0 || affected > 1 {
		return "", fmt.Errorf(
			"remove favorite rate %s:%s affected %d rows",
			base,
			quote,
			affected,
		)
	}

	return base + ":" + quote, nil
}

func (a *App) ListFavorites() ([]string, error) {
	rows, err := a.db.Query(`SELECT base, quote FROM favorite_rates ORDER BY base, quote`)
	if err != nil {
		return []string{}, fmt.Errorf("query favorite rates: %w", err)
	}
	defer rows.Close()

	favorites := []string{}
	for rows.Next() {
		var base string
		var quote string
		if err := rows.Scan(&base, &quote); err != nil {
			return []string{}, fmt.Errorf("scan favorite rate: %w", err)
		}
		favorites = append(favorites, base+":"+quote)
	}

	if err := rows.Err(); err != nil {
		return []string{}, fmt.Errorf("iterate favorite rates: %w", err)
	}

	return favorites, nil
}

func (a *App) GetFavoritesWithRates() (FavoritesWithRatesResult, error) {
	payload := FavoritesWithRatesResult{
		Base:      "",
		Favorites: make([]FavoriteRate, 0),
	}

	favorites, err := a.ListFavorites()
	if err != nil {
		return FavoritesWithRatesResult{}, err
	}

	for _, key := range favorites {
		base, quote, ok := normalizeFavoritePair(key)
		if !ok {
			continue
		}

		payload.Favorites = append(payload.Favorites, FavoriteRate{
			Code:  key,
			Base:  base,
			To:    quote,
			Found: false,
		})
	}

	ctx := a.requestContext()
	runBounded(ctx, len(payload.Favorites), favoriteRefreshWorkerLimit, func(ctx context.Context, index int) {
		favorite := &payload.Favorites[index]
		res, err := a.getRate(ctx, favorite.Base, favorite.To)
		if err != nil || !res.Found {
			return
		}
		favorite.Rate = res.Rate
		favorite.Found = true
	})

	return payload, nil
}
