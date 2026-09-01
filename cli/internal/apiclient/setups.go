package apiclient

import (
	"context"
	"fmt"
	"net/http"
)

type Setup struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AppIDs      []int  `json:"app_ids"`
	IconURL     string `json:"icon_url,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type SetupWriteRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AppIDs      []int  `json:"app_ids"`
	RemoveIcon  bool   `json:"remove_icon,omitempty"`
}

type SetupLaunchFailure struct {
	AppID   int    `json:"app_id"`
	AppName string `json:"app_name"`
	Error   string `json:"error"`
}

type SetupStartResult struct {
	Attempted int                  `json:"attempted"`
	Launched  int                  `json:"launched"`
	Failures  []SetupLaunchFailure `json:"failures"`
}

func (c *Client) Setups(ctx context.Context) ([]Setup, error) {
	var result []Setup
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/setups", nil, nil, &result)
	return result, err
}

func (c *Client) CreateSetup(ctx context.Context, request SetupWriteRequest) (Setup, error) {
	var result Setup
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/setups", nil, request, &result)
	return result, err
}

func (c *Client) UpdateSetup(ctx context.Context, id int, request SetupWriteRequest) (Setup, error) {
	var result Setup
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/v1/setups/%d", id), nil, request, &result)
	return result, err
}

func (c *Client) DeleteSetup(ctx context.Context, id int) error {
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/setups/%d", id), nil, nil, nil)
}

func (c *Client) StartSetup(ctx context.Context, id int, confirm bool) (SetupStartResult, error) {
	var result SetupStartResult
	err := c.doJSON(
		ctx,
		http.MethodPost,
		fmt.Sprintf("/api/v1/setups/%d/start", id),
		nil,
		map[string]bool{"confirm": confirm},
		&result,
	)
	return result, err
}
