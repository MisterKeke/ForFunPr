package apiclient

import (
	"context"
	"net/http"
)

type NewsError struct {
	Source   string `json:"source"`
	SourceID string `json:"source_id,omitempty"`
	Error    string `json:"error"`
}

type NewsUpdateWindow struct {
	PublishedAfter string `json:"published_after"`
	PublishedUntil string `json:"published_until"`
}

type NewsUpdateWindows struct {
	NewWhileClosed NewsUpdateWindow `json:"new_while_closed"`
	NewWhileOpen   NewsUpdateWindow `json:"new_while_open"`
}

type NewsState struct {
	PreviousOpenedAt  string            `json:"previous_opened_at"`
	CurrentOpenedAt   string            `json:"current_opened_at"`
	PreviousRefreshAt string            `json:"previous_refresh_at"`
	LastRefreshAt     string            `json:"last_refresh_at"`
	UpdateWindows     NewsUpdateWindows `json:"update_windows"`
}

type NewsResponse struct {
	ScanStartedAt string      `json:"scan_started_at"`
	News          []Post      `json:"news"`
	Errors        []NewsError `json:"errors"`
	State         NewsState   `json:"state"`
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
