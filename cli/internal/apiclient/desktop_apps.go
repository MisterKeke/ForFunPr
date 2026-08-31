package apiclient

import (
	"context"
	"fmt"
	"net/http"
)

type DesktopApp struct {
	ID          int    `json:"id"`
	DisplayName string `json:"display_name"`
	IconURL     string `json:"icon_url,omitempty"`
	Available   bool   `json:"available"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (c *Client) DesktopApps(ctx context.Context) ([]DesktopApp, error) {
	var result []DesktopApp
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/desktop-apps", nil, nil, &result)
	return result, err
}

func (c *Client) RenameDesktopApp(ctx context.Context, id int, displayName string) (DesktopApp, error) {
	var result DesktopApp
	err := c.doJSON(
		ctx, http.MethodPut, fmt.Sprintf("/api/v1/desktop-apps/%d/name", id), nil,
		map[string]string{"display_name": displayName}, &result,
	)
	return result, err
}
