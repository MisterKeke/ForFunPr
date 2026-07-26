package backend

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const weatherForecastDays = 6

// WeatherCurrent contains the current conditions returned to the frontend.
type WeatherCurrent struct {
	Temperature2M       float64 `json:"temperature_2m"`
	ApparentTemperature float64 `json:"apparent_temperature"`
	WeatherCode         int     `json:"weather_code"`
	WindSpeed10M        float64 `json:"wind_speed_10m"`
}

// WeatherDaily contains each day in the requested forecast period.
type WeatherDaily struct {
	Time                        []string  `json:"time"`
	WeatherCode                 []int     `json:"weather_code"`
	Temperature2MMax            []float64 `json:"temperature_2m_max"`
	Temperature2MMin            []float64 `json:"temperature_2m_min"`
	PrecipitationProbabilityMax []float64 `json:"precipitation_probability_max"`
}

// WeatherResult is the parsed Open-Meteo response exposed through Wails.
type WeatherResult struct {
	Current  WeatherCurrent `json:"current"`
	Daily    WeatherDaily   `json:"daily"`
	Timezone string         `json:"timezone"`
}

// WeatherLocation identifies a city returned by the Open-Meteo geocoding API.
type WeatherLocation struct {
	Name    string `json:"name"`
	Admin1  string `json:"admin1"`
	Country string `json:"country"`
}

// CityWeatherResult combines an optional city match with its forecast.
type CityWeatherResult struct {
	Found    bool            `json:"found"`
	Location WeatherLocation `json:"location"`
	Weather  *WeatherResult  `json:"weather,omitempty"`
}

// StoredLocationWeatherResult reports whether a local weather location has
// been saved and provides its cached forecast, if one is available. UpdatedAt
// is the time at which Open-Meteo last returned that cached forecast.
type StoredLocationWeatherResult struct {
	Found     bool           `json:"found"`
	Weather   *WeatherResult `json:"weather,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
}

type openMeteoForecastResponse struct {
	Current  WeatherCurrent `json:"current"`
	Daily    WeatherDaily   `json:"daily"`
	Timezone string         `json:"timezone"`
}

type openMeteoSearchResponse struct {
	Results []openMeteoSearchResult `json:"results"`
}

type openMeteoSearchResult struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Admin1    string  `json:"admin1"`
	Country   string  `json:"country"`
}

// GetWeather retrieves the local forecast for browser-provided coordinates.
func (a *Service) GetWeather(latitude float64, longitude float64) (*WeatherResult, error) {
	return a.GetWeatherContext(a.requestContext(), latitude, longitude)
}

func (a *Service) GetWeatherContext(ctx context.Context, latitude float64, longitude float64) (*WeatherResult, error) {
	if !validWeatherCoordinates(latitude, longitude) {
		return nil, fmt.Errorf("latitude must be between -90 and 90 and longitude must be between -180 and 180")
	}

	weather, err := a.getWeather(ctx, latitude, longitude)
	if err != nil {
		return nil, err
	}
	if err := a.saveWeatherLocationAndForecastContext(ctx, latitude, longitude, weather); err != nil {
		return nil, fmt.Errorf("save local weather forecast: %w", err)
	}

	return weather, nil
}

// GetStoredLocationWeather retrieves only the locally cached weather for the
// location saved by GetWeather. It intentionally never contacts Open-Meteo,
// allowing the frontend to render a previous successful forecast immediately.
// A missing saved location is expected on the first application run and is
// represented by Found=false rather than an error.
func (a *Service) GetStoredLocationWeather() (*StoredLocationWeatherResult, error) {
	latitude, longitude, forecastJSON, updatedAt, found, err := a.loadStoredWeatherLocation()
	if err != nil {
		return nil, fmt.Errorf("read saved weather location: %w", err)
	}
	if !found {
		return &StoredLocationWeatherResult{Found: false}, nil
	}
	if !validWeatherCoordinates(latitude, longitude) {
		return nil, fmt.Errorf("saved weather location contains invalid coordinates")
	}

	result := &StoredLocationWeatherResult{
		Found:     true,
		UpdatedAt: updatedAt,
	}
	if weather, ok := decodeStoredWeather(forecastJSON); ok {
		result.Weather = weather
	}

	return result, nil
}

// RefreshStoredLocationWeather retrieves a newer forecast for the saved
// location, replaces the cached response only after a successful request, and
// returns the fresh result. The frontend calls this after rendering the cache.
func (a *Service) RefreshStoredLocationWeather() (*StoredLocationWeatherResult, error) {
	return a.RefreshStoredLocationWeatherContext(a.requestContext())
}

func (a *Service) RefreshStoredLocationWeatherContext(ctx context.Context) (*StoredLocationWeatherResult, error) {
	latitude, longitude, found, err := a.loadWeatherLocationContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("read saved weather location: %w", err)
	}
	if !found {
		return &StoredLocationWeatherResult{Found: false}, nil
	}
	if !validWeatherCoordinates(latitude, longitude) {
		return nil, fmt.Errorf("saved weather location contains invalid coordinates")
	}

	weather, err := a.getWeather(ctx, latitude, longitude)
	if err != nil {
		return nil, err
	}
	updatedAt, err := a.saveWeatherForecastContext(ctx, weather)
	if err != nil {
		return nil, fmt.Errorf("cache refreshed weather forecast: %w", err)
	}

	return &StoredLocationWeatherResult{
		Found:     true,
		Weather:   weather,
		UpdatedAt: updatedAt,
	}, nil
}

// GetWeatherForCity looks up a city through Open-Meteo before retrieving its forecast.
func (a *Service) GetWeatherForCity(city string) (*CityWeatherResult, error) {
	return a.GetWeatherForCityContext(a.requestContext(), city)
}

func (a *Service) GetWeatherForCityContext(ctx context.Context, city string) (*CityWeatherResult, error) {
	city = strings.TrimSpace(city)
	if !utf8.ValidString(city) || utf8.RuneCountInString(city) < 1 || utf8.RuneCountInString(city) > 100 {
		return nil, fmt.Errorf("city name must contain between 1 and 100 characters")
	}

	body, _, err := a.httpClient.get(
		ctx,
		providerOpenMeteo,
		openMeteoSearchURL(city),
		nil,
		http.StatusOK,
	)
	if err != nil {
		return nil, err
	}

	var search openMeteoSearchResponse
	if err := json.Unmarshal(body, &search); err != nil {
		return nil, fmt.Errorf("Open-Meteo geocoding response contained invalid JSON: %w", err)
	}
	if len(search.Results) == 0 {
		return &CityWeatherResult{Found: false}, nil
	}

	match := search.Results[0]
	if !validWeatherCoordinates(match.Latitude, match.Longitude) {
		return nil, fmt.Errorf("Open-Meteo returned invalid coordinates for the selected city")
	}

	weather, err := a.getWeather(ctx, match.Latitude, match.Longitude)
	if err != nil {
		return nil, err
	}

	return &CityWeatherResult{
		Found: true,
		Location: WeatherLocation{
			Name:    match.Name,
			Admin1:  match.Admin1,
			Country: match.Country,
		},
		Weather: weather,
	}, nil
}

func (a *Service) getWeather(ctx context.Context, latitude float64, longitude float64) (*WeatherResult, error) {
	body, _, err := a.httpClient.get(
		ctx,
		providerOpenMeteo,
		openMeteoForecastURL(latitude, longitude),
		nil,
		http.StatusOK,
	)
	if err != nil {
		return nil, err
	}

	var response openMeteoForecastResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("Open-Meteo forecast response contained invalid JSON: %w", err)
	}
	if !validDailyWeather(response.Daily) {
		return nil, fmt.Errorf("Open-Meteo forecast response was missing daily weather data")
	}

	return &WeatherResult{
		Current:  response.Current,
		Daily:    response.Daily,
		Timezone: response.Timezone,
	}, nil
}

func (a *Service) loadWeatherLocationContext(ctx context.Context) (latitude float64, longitude float64, found bool, err error) {
	err = a.db.QueryRowContext(ctx, `
		SELECT latitude, longitude
		FROM location
		WHERE id = 1
	`).Scan(&latitude, &longitude)
	if err == nil {
		return latitude, longitude, true, nil
	}
	if err == sql.ErrNoRows {
		return 0, 0, false, nil
	}
	return 0, 0, false, err
}

func (a *Service) loadStoredWeatherLocation() (
	latitude float64,
	longitude float64,
	forecastJSON string,
	updatedAt string,
	found bool,
	err error,
) {
	var storedForecast sql.NullString
	var storedUpdatedAt sql.NullString
	err = a.db.QueryRowContext(a.requestContext(), `
		SELECT latitude, longitude, forecast_json, forecast_updated_at
		FROM location
		WHERE id = 1
	`).Scan(&latitude, &longitude, &storedForecast, &storedUpdatedAt)
	if err == nil {
		return latitude, longitude, storedForecast.String, storedUpdatedAt.String, true, nil
	}
	if err == sql.ErrNoRows {
		return 0, 0, "", "", false, nil
	}
	return 0, 0, "", "", false, err
}

func (a *Service) saveWeatherLocationAndForecastContext(
	ctx context.Context,
	latitude float64,
	longitude float64,
	weather *WeatherResult,
) error {
	forecastJSON, err := marshalWeather(weather)
	if err != nil {
		return err
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339)

	_, err = a.db.ExecContext(ctx, `
		INSERT INTO location (id, latitude, longitude, forecast_json, forecast_updated_at)
		VALUES (1, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			latitude = excluded.latitude,
			longitude = excluded.longitude,
			forecast_json = excluded.forecast_json,
			forecast_updated_at = excluded.forecast_updated_at,
			updated_at = CURRENT_TIMESTAMP
	`, latitude, longitude, forecastJSON, updatedAt)
	return err
}

func (a *Service) saveWeatherForecastContext(ctx context.Context, weather *WeatherResult) (string, error) {
	forecastJSON, err := marshalWeather(weather)
	if err != nil {
		return "", err
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339)

	result, err := a.db.ExecContext(ctx, `
		UPDATE location
		SET forecast_json = ?, forecast_updated_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, forecastJSON, updatedAt)
	if err != nil {
		return "", err
	}
	if err := requireSingleMutation(result, "update saved weather", "weather location", false); err != nil {
		return "", err
	}
	return updatedAt, nil
}

func marshalWeather(weather *WeatherResult) (string, error) {
	if weather == nil || !validDailyWeather(weather.Daily) {
		return "", fmt.Errorf("weather forecast is incomplete")
	}

	payload, err := json.Marshal(weather)
	if err != nil {
		return "", fmt.Errorf("encode weather forecast: %w", err)
	}
	return string(payload), nil
}

func decodeStoredWeather(payload string) (*WeatherResult, bool) {
	if strings.TrimSpace(payload) == "" {
		return nil, false
	}

	var weather WeatherResult
	if err := json.Unmarshal([]byte(payload), &weather); err != nil || !validDailyWeather(weather.Daily) {
		return nil, false
	}
	return &weather, true
}

func openMeteoForecastURL(latitude float64, longitude float64) *url.URL {
	endpoint := (&url.URL{
		Scheme: "https",
		Host:   "api.open-meteo.com",
	}).JoinPath("v1", "forecast")
	query := endpoint.Query()
	query.Set("latitude", fmt.Sprintf("%.6f", latitude))
	query.Set("longitude", fmt.Sprintf("%.6f", longitude))
	query.Set("current", "temperature_2m,apparent_temperature,weather_code,wind_speed_10m")
	query.Set("daily", "weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max")
	query.Set("timezone", "auto")
	query.Set("forecast_days", fmt.Sprintf("%d", weatherForecastDays))
	endpoint.RawQuery = query.Encode()
	return endpoint
}

func openMeteoSearchURL(city string) *url.URL {
	endpoint := (&url.URL{
		Scheme: "https",
		Host:   "geocoding-api.open-meteo.com",
	}).JoinPath("v1", "search")
	query := endpoint.Query()
	query.Set("name", city)
	query.Set("count", "1")
	query.Set("language", "en")
	query.Set("format", "json")
	endpoint.RawQuery = query.Encode()
	return endpoint
}

func validWeatherCoordinates(latitude float64, longitude float64) bool {
	return !math.IsNaN(latitude) && !math.IsInf(latitude, 0) &&
		!math.IsNaN(longitude) && !math.IsInf(longitude, 0) &&
		latitude >= -90 && latitude <= 90 && longitude >= -180 && longitude <= 180
}

func validDailyWeather(daily WeatherDaily) bool {
	length := len(daily.Time)
	return length > 0 &&
		len(daily.WeatherCode) == length &&
		len(daily.Temperature2MMax) == length &&
		len(daily.Temperature2MMin) == length &&
		len(daily.PrecipitationProbabilityMax) == length
}
