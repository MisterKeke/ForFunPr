package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

const weatherForecastDays = 6

// WeatherCurrent contains the current conditions returned to the frontend.
type WeatherCurrent struct {
	Temperature2M      float64 `json:"temperature_2m"`
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
func (a *App) GetWeather(latitude float64, longitude float64) (*WeatherResult, error) {
	if !validWeatherCoordinates(latitude, longitude) {
		return nil, fmt.Errorf("latitude must be between -90 and 90 and longitude must be between -180 and 180")
	}

	return a.getWeather(a.requestContext(), latitude, longitude)
}

// GetWeatherForCity looks up a city through Open-Meteo before retrieving its forecast.
func (a *App) GetWeatherForCity(city string) (*CityWeatherResult, error) {
	city = strings.TrimSpace(city)
	if !utf8.ValidString(city) || utf8.RuneCountInString(city) < 1 || utf8.RuneCountInString(city) > 100 {
		return nil, fmt.Errorf("city name must contain between 1 and 100 characters")
	}

	ctx := a.requestContext()
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

func (a *App) getWeather(ctx context.Context, latitude float64, longitude float64) (*WeatherResult, error) {
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
