package schemas

import backend "something/backend/service"

type AddSteamGameInput struct {
	StoreURL string `json:"store_url"`
}
type SteamGameIDInput struct {
	ID int `json:"id"`
}
type SteamCountryInput struct {
	CountryCode string `json:"country_code"`
}

var AddSteamGameInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"store_url": map[string]any{"type": "string", "minLength": 1, "maxLength": 4096, "description": "Steam Store /app/ URL."},
	}, "required": []string{"store_url"}, "additionalProperties": false,
}

var SteamGameIDInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"id": map[string]any{"type": "integer", "minimum": 1, "description": "Tracked Steam game ID, not Steam App ID."},
	}, "required": []string{"id"}, "additionalProperties": false,
}

var SteamCountryInputSchema = map[string]any{
	"type": "object", "properties": map[string]any{
		"country_code": map[string]any{
			"type": "string", "description": "Supported Steam Store country code.",
			"enum": supportedSteamCountryCodeSchemaValues(),
		},
	}, "required": []string{"country_code"}, "additionalProperties": false,
}

func supportedSteamCountryCodeSchemaValues() []string {
	countries := backend.ListSupportedSteamCountries()
	values := make([]string, 0, len(countries))
	for _, country := range countries {
		values = append(values, country.Code)
	}
	return values
}

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

type SteamGamesOutput struct {
	Games []SteamGame `json:"games"`
}

type SteamGameSettings struct {
	CountryCode string `json:"country_code"`
	UpdatedAt   string `json:"updated_at"`
}

type SteamCountry struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
type SteamCountriesOutput struct {
	Countries []SteamCountry `json:"countries"`
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
