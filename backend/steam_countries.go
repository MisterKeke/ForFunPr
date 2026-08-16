package backend

import "strings"

// SteamCountry is one store country that the price tracker allows users to
// select. Steam determines the actual currency and regional price returned for
// each country code.
type SteamCountry struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

var supportedSteamCountries = []SteamCountry{
	{Code: "AU", Name: "Australia"},
	{Code: "BR", Name: "Brazil"},
	{Code: "CA", Name: "Canada"},
	{Code: "CN", Name: "China"},
	{Code: "FR", Name: "France"},
	{Code: "DE", Name: "Germany"},
	{Code: "IN", Name: "India"},
	{Code: "JP", Name: "Japan"},
	{Code: "MX", Name: "Mexico"},
	{Code: "NL", Name: "Netherlands"},
	{Code: "PL", Name: "Poland"},
	{Code: "KR", Name: "South Korea"},
	{Code: "SE", Name: "Sweden"},
	{Code: "TR", Name: "Türkiye"},
	{Code: "UA", Name: "Ukraine"},
	{Code: "GB", Name: "United Kingdom"},
	{Code: "US", Name: "United States"},
}

var supportedSteamCountryCodes = func() map[string]struct{} {
	codes := make(map[string]struct{}, len(supportedSteamCountries))
	for _, country := range supportedSteamCountries {
		codes[country.Code] = struct{}{}
	}
	return codes
}()

func ListSupportedSteamCountries() []SteamCountry {
	countries := make([]SteamCountry, len(supportedSteamCountries))
	copy(countries, supportedSteamCountries)
	return countries
}

func normalizeSteamCountryCode(value string) (string, error) {
	code := strings.ToUpper(strings.TrimSpace(value))
	if _, supported := supportedSteamCountryCodes[code]; !supported {
		return "", &ValidationError{
			Field:   "country_code",
			Message: "Choose a supported Steam store country.",
		}
	}
	return code, nil
}
