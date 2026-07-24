package apiclient

import (
	"context"
	"net/http"
)

type NewsItem struct {
	ChannelName string `json:"channel_name"`
	PostedAt    string `json:"posted_at"`
}

type NewsResponse struct {
	News []NewsItem `json:"news"`
}

func (c *Client) News(ctx context.Context) (NewsResponse, error) {
	return c.newsRequest(ctx, http.MethodGet, "/api/v1/news", nil)
}

func (c *Client) InitialNews(ctx context.Context) (NewsResponse, error) {
	return c.newsRequest(
		ctx,
		http.MethodPost,
		"/api/v1/news/initial",
		struct{}{},
	)
}

func (c *Client) RefreshNews(ctx context.Context) (NewsResponse, error) {
	return c.newsRequest(
		ctx,
		http.MethodPost,
		"/api/v1/news/refresh",
		struct{}{},
	)
}

func (c *Client) NewsSinceLastOpen(
	ctx context.Context,
) (NewsResponse, error) {
	return c.newsRequest(
		ctx,
		http.MethodPost,
		"/api/v1/news/since-last-open",
		struct{}{},
	)
}

func (c *Client) NewsState(ctx context.Context) (any, error) {
	return c.doValue(
		ctx,
		http.MethodGet,
		"/api/v1/news/state",
		nil,
		nil,
	)
}

func (c *Client) NewsWindows(ctx context.Context) (any, error) {
	return c.doValue(
		ctx,
		http.MethodGet,
		"/api/v1/news/windows",
		nil,
		nil,
	)
}

func (c *Client) newsRequest(
	ctx context.Context,
	method string,
	path string,
	body any,
) (NewsResponse, error) {
	var result NewsResponse
	if err := c.doJSON(ctx, method, path, nil, body, &result); err != nil {
		return NewsResponse{}, err
	}

	return result, nil
}
