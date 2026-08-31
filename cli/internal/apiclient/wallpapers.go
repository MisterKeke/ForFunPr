package apiclient

import (
	"context"
	"fmt"
	"net/http"
)

type UserWallpaper struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	MIMEType    string `json:"mime_type"`
	ByteSize    int64  `json:"byte_size"`
	URL         string `json:"url"`
}

type WallpaperSettings struct {
	Selected          string          `json:"selected"`
	SelectionSaved    bool            `json:"selection_saved"`
	BuiltinSelections []string        `json:"builtin_selections"`
	UserWallpapers    []UserWallpaper `json:"user_wallpapers"`
}

func (c *Client) WallpaperSettings(ctx context.Context) (WallpaperSettings, error) {
	var result WallpaperSettings
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/wallpapers", nil, nil, &result)
	return result, err
}

func (c *Client) SelectWallpaper(ctx context.Context, selection string) (WallpaperSettings, error) {
	var result WallpaperSettings
	err := c.doJSON(ctx, http.MethodPut, "/api/v1/wallpapers/selection", nil, map[string]string{"selection": selection}, &result)
	return result, err
}

func (c *Client) DeleteUserWallpaper(ctx context.Context, id string) (WallpaperSettings, error) {
	var result WallpaperSettings
	err := c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/wallpapers/%s", id), nil, nil, &result)
	return result, err
}
