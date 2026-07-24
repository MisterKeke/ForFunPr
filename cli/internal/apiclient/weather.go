package apiclient

import (
	"context"
	"net/http"
	"net/url"
)

type WeatherLocationRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func (c *Client) Weather(ctx context.Context, city string) (any, error) {
	query := make(url.Values)
	query.Set("city", city)

	return c.doValue(
		ctx,
		http.MethodGet,
		"/api/v1/weather",
		query,
		nil,
	)
}

func (c *Client) SaveWeatherLocation(
	ctx context.Context,
	request WeatherLocationRequest,
) (any, error) {
	return c.doValue(
		ctx,
		http.MethodPut,
		"/api/v1/weather",
		nil,
		request,
	)
}

func (c *Client) StoredWeather(ctx context.Context) (any, error) {
	return c.doValue(
		ctx,
		http.MethodGet,
		"/api/v1/weather/stored",
		nil,
		nil,
	)
}

func (c *Client) RefreshStoredWeather(ctx context.Context) (any, error) {
	return c.doValue(
		ctx,
		http.MethodPost,
		"/api/v1/weather/stored/refresh",
		nil,
		struct{}{},
	)
}
