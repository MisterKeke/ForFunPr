package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maximumSteamStoreURLBytes = 4096
	maximumSteamGameNameRunes = 300
)

type steamAppDetailsEnvelope struct {
	Success bool                `json:"success"`
	Data    steamAppDetailsData `json:"data"`
}

type steamAppDetailsData struct {
	Type          string              `json:"type"`
	Name          string              `json:"name"`
	SteamAppID    uint32              `json:"steam_appid"`
	IsFree        bool                `json:"is_free"`
	HeaderImage   string              `json:"header_image"`
	CapsuleImage  string              `json:"capsule_image"`
	PriceOverview *steamPriceOverview `json:"price_overview"`
}

type steamPriceOverview struct {
	Currency        string `json:"currency"`
	Initial         int64  `json:"initial"`
	Final           int64  `json:"final"`
	DiscountPercent int    `json:"discount_percent"`
}

type fetchedSteamGame struct {
	SteamAppID        uint32
	Name              string
	ImageSourceURL    string
	PriceStatus       string
	Currency          string
	RegularPriceMinor *int64
	CurrentPriceMinor *int64
	DiscountPercent   int
}

func parseSteamGameURL(raw string) (uint32, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, "", &ValidationError{
			Field:   "url",
			Message: "Steam game URL cannot be empty.",
		}
	}
	if len(raw) > maximumSteamStoreURLBytes {
		return 0, "", &ValidationError{
			Field:   "url",
			Message: "Steam game URL is too long.",
		}
	}

	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return 0, "", &ValidationError{
			Field:   "url",
			Message: "Enter a valid Steam Store URL.",
		}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return 0, "", &ValidationError{
			Field:   "url",
			Message: "Steam game URL must use HTTP or HTTPS.",
		}
	}
	if parsed.User != nil {
		return 0, "", &ValidationError{
			Field:   "url",
			Message: "Steam game URL cannot contain credentials.",
		}
	}
	if !strings.EqualFold(parsed.Hostname(), "store.steampowered.com") {
		return 0, "", &ValidationError{
			Field:   "url",
			Message: "Enter a URL from store.steampowered.com.",
		}
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || !strings.EqualFold(parts[0], "app") {
		return 0, "", &ValidationError{
			Field:   "url",
			Message: "The URL must point to a Steam /app/ page.",
		}
	}
	parsedID, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil || parsedID == 0 {
		return 0, "", &ValidationError{
			Field:   "url",
			Message: "The Steam URL contains an invalid App ID.",
		}
	}

	appID := uint32(parsedID)
	canonicalURL := fmt.Sprintf("https://store.steampowered.com/app/%d/", appID)
	return appID, canonicalURL, nil
}

func steamAppDetailsURL(appID uint32, countryCode string) *url.URL {
	requestURL := &url.URL{
		Scheme: "https",
		Host:   "store.steampowered.com",
		Path:   "/api/appdetails",
	}
	query := requestURL.Query()
	query.Set("appids", strconv.FormatUint(uint64(appID), 10))
	query.Set("cc", countryCode)
	query.Set("l", "english")
	requestURL.RawQuery = query.Encode()
	return requestURL
}

func (a *Service) fetchSteamGameContext(
	ctx context.Context,
	appID uint32,
	countryCode string,
) (fetchedSteamGame, error) {
	countryCode, err := normalizeSteamCountryCode(countryCode)
	if err != nil {
		return fetchedSteamGame{}, err
	}
	if appID == 0 {
		return fetchedSteamGame{}, &ValidationError{
			Field:   "steam_app_id",
			Message: "Choose a valid Steam game.",
		}
	}

	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("User-Agent", "Something/1.0")
	body, _, err := a.httpClient.get(
		ctx,
		providerSteamStore,
		steamAppDetailsURL(appID, countryCode),
		headers,
		http.StatusOK,
	)
	if err != nil {
		return fetchedSteamGame{}, err
	}

	response := map[string]steamAppDetailsEnvelope{}
	if err := json.Unmarshal(body, &response); err != nil {
		return fetchedSteamGame{}, fmt.Errorf("decode Steam Store response: %w", err)
	}
	key := strconv.FormatUint(uint64(appID), 10)
	entry, found := response[key]
	if !found || !entry.Success {
		return fetchedSteamGame{}, fmt.Errorf("Steam did not return details for App ID %d", appID)
	}
	if entry.Data.SteamAppID != appID {
		return fetchedSteamGame{}, fmt.Errorf("Steam returned details for a different App ID")
	}
	if !strings.EqualFold(strings.TrimSpace(entry.Data.Type), "game") {
		return fetchedSteamGame{}, &ValidationError{
			Field:   "url",
			Message: "This Steam page is not a game.",
		}
	}

	name := strings.TrimSpace(entry.Data.Name)
	if name == "" || !utf8.ValidString(name) {
		return fetchedSteamGame{}, fmt.Errorf("Steam response did not contain a valid game name")
	}
	if utf8.RuneCountInString(name) > maximumSteamGameNameRunes {
		return fetchedSteamGame{}, fmt.Errorf("Steam returned a game name longer than %d characters", maximumSteamGameNameRunes)
	}

	imageURL := strings.TrimSpace(entry.Data.HeaderImage)
	if imageURL == "" {
		imageURL = strings.TrimSpace(entry.Data.CapsuleImage)
	}
	imageURL = normalizeSteamArtworkURL(imageURL)

	status, currency, regular, current, discount, err := normalizeSteamPrice(entry.Data)
	if err != nil {
		return fetchedSteamGame{}, err
	}

	return fetchedSteamGame{
		SteamAppID:        appID,
		Name:              name,
		ImageSourceURL:    imageURL,
		PriceStatus:       status,
		Currency:          currency,
		RegularPriceMinor: regular,
		CurrentPriceMinor: current,
		DiscountPercent:   discount,
	}, nil
}

func normalizeSteamPrice(
	data steamAppDetailsData,
) (string, string, *int64, *int64, int, error) {
	if data.IsFree {
		regular := int64(0)
		current := int64(0)
		return "free", "", &regular, &current, 0, nil
	}
	if data.PriceOverview == nil {
		return "unavailable", "", nil, nil, 0, nil
	}

	price := data.PriceOverview
	if price.Initial < 0 || price.Final < 0 {
		return "", "", nil, nil, 0, fmt.Errorf("Steam returned a negative price")
	}
	currency := strings.ToUpper(strings.TrimSpace(price.Currency))
	if !currencyCodePattern.MatchString(currency) {
		return "", "", nil, nil, 0, fmt.Errorf("Steam returned an invalid currency")
	}
	if price.DiscountPercent < 0 || price.DiscountPercent > 100 {
		return "", "", nil, nil, 0, fmt.Errorf("Steam returned an invalid discount")
	}

	regular := price.Initial
	current := price.Final
	return "priced", currency, &regular, &current, price.DiscountPercent, nil
}
