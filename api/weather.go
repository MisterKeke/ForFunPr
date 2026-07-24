package api

import (
	"math"
	"net/http"
	"strings"
	"unicode/utf8"

	"currency-wails/backend"
)

type saveWeatherLocationRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type weatherResponse struct {
	City                               string  `json:"city"`
	Region                             string  `json:"region,omitempty"`
	Country                            string  `json:"country,omitempty"`
	Date                               string  `json:"date"`
	Timezone                           string  `json:"timezone"`
	TemperatureC                       float64 `json:"temperature_c"`
	FeelsLikeC                         float64 `json:"feels_like_c"`
	WeatherCode                        int     `json:"weather_code"`
	WindSpeedKMH                       float64 `json:"wind_speed_kmh"`
	MinimumC                           float64 `json:"minimum_c"`
	MaximumC                           float64 `json:"maximum_c"`
	PrecipitationProbabilityPercentage float64 `json:"precipitation_probability_percentage"`
}

func weatherHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		city := strings.TrimSpace(r.URL.Query().Get("city"))
		if !utf8.ValidString(city) || utf8.RuneCountInString(city) < 1 || utf8.RuneCountInString(city) > 100 {
			writeError(w, http.StatusBadRequest, "invalid_city", "A city name between 1 and 100 characters is required.")
			return
		}

		result, err := app.GetWeatherForCity(city)
		if err != nil {
			writeError(w, http.StatusBadGateway, "weather_failed", "Weather could not be loaded.")
			return
		}
		if result == nil || !result.Found {
			writeError(w, http.StatusNotFound, "city_not_found", "The requested city was not found.")
			return
		}
		if result.Weather == nil || !hasTodayWeather(result.Weather.Daily) {
			writeError(w, http.StatusBadGateway, "weather_incomplete", "The weather provider returned incomplete data.")
			return
		}

		weather := result.Weather
		writeJSON(w, http.StatusOK, weatherResponse{
			City:                               result.Location.Name,
			Region:                             result.Location.Admin1,
			Country:                            result.Location.Country,
			Date:                               weather.Daily.Time[0],
			Timezone:                           weather.Timezone,
			TemperatureC:                       weather.Current.Temperature2M,
			FeelsLikeC:                         weather.Current.ApparentTemperature,
			WeatherCode:                        weather.Current.WeatherCode,
			WindSpeedKMH:                       weather.Current.WindSpeed10M,
			MinimumC:                           weather.Daily.Temperature2MMin[0],
			MaximumC:                           weather.Daily.Temperature2MMax[0],
			PrecipitationProbabilityPercentage: weather.Daily.PrecipitationProbabilityMax[0],
		})
	}
}

func saveWeatherLocationHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		var request saveWeatherLocationRequest
		if !decodeJSONBody(w, r, &request) {
			return
		}
		if !validWeatherLocation(request.Latitude, request.Longitude) {
			writeError(
				w,
				http.StatusUnprocessableEntity,
				"invalid_coordinates",
				"Latitude must be between -90 and 90 and longitude must be between -180 and 180.",
			)
			return
		}

		result, err := app.GetWeather(request.Latitude, request.Longitude)
		if err != nil {
			writeError(w, http.StatusBadGateway, "weather_save_failed", "The selected location and its forecast could not be saved.")
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func storedWeatherHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !backendReady(w, app) {
			return
		}

		result, err := app.GetStoredLocationWeather()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "stored_weather_failed", "Stored weather could not be loaded.")
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func refreshStoredWeatherHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		// Requiring JSON keeps this state-changing localhost endpoint out of
		// reach of simple cross-origin form requests.
		var request struct{}
		if !decodeJSONBody(w, r, &request) {
			return
		}

		result, err := app.RefreshStoredLocationWeather()
		if err != nil {
			writeError(w, http.StatusBadGateway, "stored_weather_refresh_failed", "Stored weather could not be refreshed.")
			return
		}
		if result == nil || !result.Found {
			writeError(w, http.StatusNotFound, "stored_location_not_found", "No weather location has been saved.")
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func validWeatherLocation(latitude float64, longitude float64) bool {
	return !math.IsNaN(latitude) &&
		!math.IsInf(latitude, 0) &&
		!math.IsNaN(longitude) &&
		!math.IsInf(longitude, 0) &&
		latitude >= -90 &&
		latitude <= 90 &&
		longitude >= -180 &&
		longitude <= 180
}
