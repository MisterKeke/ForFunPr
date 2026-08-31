package apiclient

import (
	"context"
	"fmt"
	"net/http"
)

type SteamGame struct {
	ID                int    `json:"id"`
	SteamAppID        uint32 `json:"steam_app_id"`
	StoreURL          string `json:"store_url"`
	Name              string `json:"name"`
	ImageURL          string `json:"image_url"`
	ImageSourceURL    string `json:"image_source_url,omitempty"`
	PriceStatus       string `json:"price_status"`
	Currency          string `json:"currency,omitempty"`
	RegularPriceMinor *int64 `json:"regular_price_minor"`
	CurrentPriceMinor *int64 `json:"current_price_minor"`
	DiscountPercent   int    `json:"discount_percent"`
	PriceCountryCode  string `json:"price_country_code"`
	LastCheckedAt     string `json:"last_checked_at,omitempty"`
	LastAttemptedAt   string `json:"last_attempted_at,omitempty"`
	LastRefreshError  string `json:"last_refresh_error,omitempty"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type SteamGameSettings struct {
	CountryCode string `json:"country_code"`
	UpdatedAt   string `json:"updated_at"`
}

type SteamCountry struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type SteamGameRefreshError struct {
	GameID     int    `json:"game_id"`
	SteamAppID uint32 `json:"steam_app_id"`
	Name       string `json:"name"`
	Error      string `json:"error"`
}

type SteamGameRefreshResult struct {
	Games       []SteamGame             `json:"games"`
	Attempted   int                     `json:"attempted"`
	Updated     int                     `json:"updated"`
	Failed      int                     `json:"failed"`
	StartedAt   string                  `json:"started_at"`
	CompletedAt string                  `json:"completed_at"`
	Errors      []SteamGameRefreshError `json:"errors"`
}

func (c *Client) SteamGames(ctx context.Context) ([]SteamGame, error) {
	var result []SteamGame
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/steam-games", nil, nil, &result)
	return result, err
}

func (c *Client) AddSteamGame(ctx context.Context, storeURL string) (SteamGame, error) {
	var result SteamGame
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/steam-games", nil, map[string]string{"store_url": storeURL}, &result)
	return result, err
}

func (c *Client) DeleteSteamGame(ctx context.Context, id int) error {
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/steam-games/%d", id), nil, nil, nil)
}

func (c *Client) SteamGameSettings(ctx context.Context) (SteamGameSettings, error) {
	var result SteamGameSettings
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/steam-games/settings", nil, nil, &result)
	return result, err
}

func (c *Client) SetSteamGameCountry(ctx context.Context, countryCode string) (SteamGameSettings, error) {
	var result SteamGameSettings
	err := c.doJSON(ctx, http.MethodPut, "/api/v1/steam-games/settings", nil, map[string]string{"country_code": countryCode}, &result)
	return result, err
}

func (c *Client) SteamCountries(ctx context.Context) ([]SteamCountry, error) {
	var result []SteamCountry
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/steam-games/countries", nil, nil, &result)
	return result, err
}

func (c *Client) RefreshSteamGames(ctx context.Context) (SteamGameRefreshResult, error) {
	var result SteamGameRefreshResult
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/steam-games/refresh", nil, struct{}{}, &result)
	return result, err
}
