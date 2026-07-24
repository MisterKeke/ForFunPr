package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func (c *Client) Currencies(
	ctx context.Context,
	base string,
	symbols string,
) (any, error) {
	query := make(url.Values)
	query.Set("base", base)
	if symbols != "" {
		query.Set("symbols", symbols)
	}

	return c.doValue(
		ctx,
		http.MethodGet,
		"/api/v1/currencies",
		query,
		nil,
	)
}

func (c *Client) CurrencyRate(
	ctx context.Context,
	base string,
	target string,
) (any, error) {
	query := make(url.Values)
	query.Set("base", base)
	query.Set("target", target)

	return c.doValue(
		ctx,
		http.MethodGet,
		"/api/v1/currencies/rate",
		query,
		nil,
	)
}

func (c *Client) CurrencyFavorites(ctx context.Context) (any, error) {
	return c.doValue(
		ctx,
		http.MethodGet,
		"/api/v1/currencies/favorites",
		nil,
		nil,
	)
}

func (c *Client) CurrencyFavoriteRates(ctx context.Context) (any, error) {
	return c.doValue(
		ctx,
		http.MethodGet,
		"/api/v1/currencies/favorites/rates",
		nil,
		nil,
	)
}

func (c *Client) AddCurrencyFavorite(
	ctx context.Context,
	base string,
	target string,
) (any, error) {
	return c.doValue(
		ctx,
		http.MethodPut,
		fmt.Sprintf(
			"/api/v1/currencies/favorites/%s/%s",
			base,
			target,
		),
		nil,
		nil,
	)
}

func (c *Client) RemoveCurrencyFavorite(
	ctx context.Context,
	base string,
	target string,
) error {
	return c.doJSON(
		ctx,
		http.MethodDelete,
		fmt.Sprintf(
			"/api/v1/currencies/favorites/%s/%s",
			base,
			target,
		),
		nil,
		nil,
		nil,
	)
}
