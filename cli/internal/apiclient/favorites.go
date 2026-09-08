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
	Name         string `json:"name"`
	Source       string `json:"source"`
	Color        string `json:"color"`
	DisplayOrder *int   `json:"display_order,omitempty"`
}

type FavoriteCategoryUpdateRequest struct {
	Name         string  `json:"name"`
	Color        *string `json:"color,omitempty"`
	DisplayOrder *int    `json:"display_order,omitempty"`
}

// FavoriteCategoryRenameRequest remains as a source-compatible alias for
// clients compiled against the earlier name-only operation.
type FavoriteCategoryRenameRequest = FavoriteCategoryUpdateRequest

type FavoriteCategory struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Source       string `json:"source"`
	Color        string `json:"color"`
	DisplayOrder int    `json:"display_order"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type FavoriteChannel struct {
	Username   string `json:"username,omitempty"`
	ChannelID  string `json:"channel_id,omitempty"`
	CategoryID *int   `json:"category_id,omitempty"`
}

type FavoriteListResult struct {
	Favorites []string `json:"favorites"`
}

type CategorizedFavoriteListResult struct {
	Favorites []FavoriteChannel `json:"favorites"`
}

type FavoriteCategoriesResult struct {
	Categories []FavoriteCategory `json:"categories"`
}
type FavoriteCategoryMutationResult struct {
	Category          FavoriteCategory `json:"category"`
	Changed           bool             `json:"changed"`
	AffectedFavorites int              `json:"affected_favorites"`
}
type FavoriteCategoryDeleteRequest struct {
	Mode             string `json:"mode"`
	TargetCategoryID *int   `json:"target_category_id,omitempty"`
}
type FavoriteCategoryDeleteResult struct {
	DeletedID         int    `json:"deleted_id"`
	Source            string `json:"source"`
	Changed           bool   `json:"changed"`
	AffectedFavorites int    `json:"affected_favorites"`
}
type FavoriteCategoryReorderRequest struct {
	Source      string `json:"source"`
	CategoryIDs []int  `json:"category_ids"`
}

func (c *Client) Favorites(
	ctx context.Context,
	source string,
	categorized bool,
) (any, error) {
	path := fmt.Sprintf("/api/v1/favorites/%s", source)
	if categorized {
		path += "/categories"
		var result CategorizedFavoriteListResult
		err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &result)
		return result, err
	}

	var result FavoriteListResult
	err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &result)
	return result, err
}

func (c *Client) AddFavoriteChannel(
	ctx context.Context,
	source string,
	channel string,
) (FavoriteListResult, error) {
	var result FavoriteListResult
	err := c.doJSON(
		ctx,
		http.MethodPut,
		fmt.Sprintf("/api/v1/favorites/%s/%s", source, channel),
		nil,
		nil, &result,
	)
	return result, err
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
) (FavoriteCategoriesResult, error) {
	query := make(url.Values)
	if source != "" {
		query.Set("source", source)
	}

	var result FavoriteCategoriesResult
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/favorite-categories", query, nil, &result)
	return result, err
}

func (c *Client) CreateFavoriteCategory(
	ctx context.Context,
	request FavoriteCategoryCreateRequest,
) (FavoriteCategoryMutationResult, error) {
	var result FavoriteCategoryMutationResult
	err := c.doJSON(
		ctx,
		http.MethodPost,
		"/api/v1/favorite-categories",
		nil,
		request, &result,
	)
	return result, err
}

func (c *Client) RenameFavoriteCategory(
	ctx context.Context,
	id int,
	request FavoriteCategoryRenameRequest,
) (FavoriteCategoryMutationResult, error) {
	return c.UpdateFavoriteCategory(ctx, id, request)
}

func (c *Client) UpdateFavoriteCategory(
	ctx context.Context,
	id int,
	request FavoriteCategoryUpdateRequest,
) (FavoriteCategoryMutationResult, error) {
	var result FavoriteCategoryMutationResult
	err := c.doJSON(
		ctx,
		http.MethodPut,
		fmt.Sprintf("/api/v1/favorite-categories/%d", id),
		nil,
		request, &result,
	)
	return result, err
}

func (c *Client) DeleteFavoriteCategory(ctx context.Context, id int, request FavoriteCategoryDeleteRequest) (FavoriteCategoryDeleteResult, error) {
	var result FavoriteCategoryDeleteResult
	err := c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/favorite-categories/%d", id), nil, request, &result)
	return result, err
}

func (c *Client) ReorderFavoriteCategories(ctx context.Context, request FavoriteCategoryReorderRequest) (FavoriteCategoriesResult, error) {
	var result FavoriteCategoriesResult
	err := c.doJSON(ctx, http.MethodPut, "/api/v1/favorite-categories/reorder", nil, request, &result)
	return result, err
}
