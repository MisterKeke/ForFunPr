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
