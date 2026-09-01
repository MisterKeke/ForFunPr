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

type DesktopAppLaunchResult struct {
	AppID    int    `json:"app_id"`
	AppName  string `json:"app_name"`
	Launched bool   `json:"launched"`
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

func (c *Client) LaunchDesktopApp(
	ctx context.Context,
	id int,
	confirm bool,
) (DesktopAppLaunchResult, error) {
	var result DesktopAppLaunchResult
	err := c.doJSON(
		ctx,
		http.MethodPost,
		fmt.Sprintf("/api/v1/desktop-apps/%d/launch", id),
		nil,
		map[string]bool{"confirm": confirm},
		&result,
	)
	return result, err
}
