package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type FavoriteCategoryAssignmentRequest struct {
	CategoryID int `json:"category_id"`
}

type FavoriteCategoryCreateRequest struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

type FavoriteCategoryRenameRequest struct {
	Name string `json:"name"`
}

func (c *Client) Favorites(
	ctx context.Context,
	source string,
	categorized bool,
) (any, error) {
	path := fmt.Sprintf("/api/v1/favorites/%s", source)
	if categorized {
		path += "/categories"
	}

	return c.doValue(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) AddFavoriteChannel(
	ctx context.Context,
	source string,
	channel string,
) (any, error) {
	return c.doValue(
		ctx,
		http.MethodPut,
		fmt.Sprintf("/api/v1/favorites/%s/%s", source, channel),
		nil,
		nil,
	)
}

func (c *Client) RemoveFavoriteChannel(
	ctx context.Context,
	source string,
	channel string,
) error {
	return c.doJSON(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("/api/v1/favorites/%s/%s", source, channel),
		nil,
		nil,
		nil,
	)
}

func (c *Client) AssignFavoriteCategory(
	ctx context.Context,
	source string,
	channel string,
	categoryID int,
) error {
	return c.doJSON(
		ctx,
		http.MethodPut,
		fmt.Sprintf(
			"/api/v1/favorites/%s/%s/category",
			source,
			channel,
		),
		nil,
		FavoriteCategoryAssignmentRequest{CategoryID: categoryID},
		nil,
	)
}

func (c *Client) FavoriteCategories(
	ctx context.Context,
	source string,
) (any, error) {
	query := make(url.Values)
	if source != "" {
		query.Set("source", source)
	}

	return c.doValue(
		ctx,
		http.MethodGet,
		"/api/v1/favorite-categories",
		query,
		nil,
	)
}

func (c *Client) CreateFavoriteCategory(
	ctx context.Context,
	request FavoriteCategoryCreateRequest,
) (any, error) {
	return c.doValue(
		ctx,
		http.MethodPost,
		"/api/v1/favorite-categories",
		nil,
		request,
	)
}

func (c *Client) RenameFavoriteCategory(
	ctx context.Context,
	id int,
	request FavoriteCategoryRenameRequest,
) (any, error) {
	return c.doValue(
		ctx,
		http.MethodPut,
		fmt.Sprintf("/api/v1/favorite-categories/%d", id),
		nil,
		request,
	)
}
