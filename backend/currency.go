package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

type favoriteRate struct {
	Code  string  `json:"code"`
	Base  string  `json:"base"`
	To    string  `json:"to"`
	Rate  float64 `json:"rate"`
	Found bool    `json:"found"`
}

type favoritesPayload struct {
	Base      string         `json:"base"`
	Favorites []favoriteRate `json:"favorites"`
}

type V2SingleRateResponse struct {
	Date  string  `json:"date"`
	Base  string  `json:"base"`
	Quote string  `json:"quote"`
	Rate  float64 `json:"rate"`
}

type V2RatesResponse struct {
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
	base = strings.ToUpper(strings.TrimSpace(base))
	target = strings.ToUpper(strings.TrimSpace(target))

	if base == "" || target == "" {
		return nil, fmt.Errorf("base and target currency codes are required")
	}

	if base == target {
		return &RateResult{Base: base, Date: "", To: target, Rate: 1.0, Found: true}, nil
	}

	url := "https://api.frankfurter.dev/v2/rate/" + base + "/" + target

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &RateResult{Base: base, To: target, Found: false}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var data V2SingleRateResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
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
	base = strings.ToUpper(strings.TrimSpace(base))
	if base == "" {
		return nil, fmt.Errorf("base currency code is required")
	}

	url := "https://api.frankfurter.dev/v2/rates?base=" + base

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	var list []V2RatesResponse
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
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

func normalizeCurrency(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 3 {
		return ""
	}
	return code
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

func (a *App) AddFavorite(name string) AddFavoriteResult {
	base, quote, ok := normalizeFavoritePair(name)
	if !ok {
		return AddFavoriteResult{
			Error: "invalid currency pair format",
		}
	}

	var exists bool
	err := a.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM favorite_rates WHERE base = ? AND quote = ?)`,
		base, quote,
	).Scan(&exists)
	if err != nil {
		return AddFavoriteResult{
			Error: err.Error(),
		}
	}

	if exists {
		return AddFavoriteResult{
			Pair:   base + ":" + quote,
			Exists: true,
			Added:  false,
		}
	}

	_, err = a.db.Exec(
		`INSERT INTO favorite_rates (base, quote) VALUES (?, ?)`,
		base, quote,
	)
	if err != nil {
		return AddFavoriteResult{
			Error: err.Error(),
		}
	}

	return AddFavoriteResult{
		Pair:   base + ":" + quote,
		Added:  true,
		Exists: false,
	}
}

func (a *App) RemoveFavorite(name string) string {
	base, quote, ok := normalizeFavoritePair(name)
	if !ok {
		return ""
	}

	_, err := a.db.Exec(
		`DELETE FROM favorite_rates WHERE base = ? AND quote = ?`,
		base,
		quote,
	)
	if err != nil {
		return ""
	}

	return base + ":" + quote
}

func (a *App) ListFavorites() []string {
	rows, err := a.db.Query(`SELECT base, quote FROM favorite_rates ORDER BY base, quote`)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	favorites := []string{}
	for rows.Next() {
		var base string
		var quote string
		if err := rows.Scan(&base, &quote); err == nil {
			favorites = append(favorites, base+":"+quote)
		}
	}

	return favorites
}

func (a *App) GetFavoriteswithRates() string {
	payload := favoritesPayload{
		Base:      "",
		Favorites: make([]favoriteRate, 0),
	}

	for _, key := range a.ListFavorites() {
		base, quote, ok := normalizeFavoritePair(key)
		if !ok {
			continue
		}

		res, err := a.GetRate(base, quote)
		if err != nil || !res.Found {
			payload.Favorites = append(payload.Favorites, favoriteRate{
				Code:  key,
				Base:  base,
				To:    quote,
				Found: false,
			})
			continue
		}
		payload.Favorites = append(payload.Favorites, favoriteRate{
			Code:  key,
			Base:  base,
			To:    quote,
			Rate:  res.Rate,
			Found: true,
		})
	}

	data, _ := json.Marshal(payload)
	return string(data)
}
