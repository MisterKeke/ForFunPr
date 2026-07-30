package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type Bookmark struct {
	ID          int      `json:"id"`
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Read        bool     `json:"read"`
	ReadAt      string   `json:"read_at"`
	Tags        []string `json:"tags"`
	Revision    int      `json:"revision"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type BookmarkListResult struct {
	Bookmarks []Bookmark `json:"bookmarks"`
	Total     int        `json:"total"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
}

type BookmarkFilter struct {
	Query  string
	Status string
	Tags   []string
	Limit  int
	Offset int
}

type BookmarkWriteRequest struct {
	URL              string   `json:"url"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Tags             []string `json:"tags"`
	ExpectedRevision int      `json:"expected_revision,omitempty"`
}

type BookmarkReadRequest struct {
	Read             bool `json:"read"`
	ExpectedRevision int  `json:"expected_revision"`
}

func (c *Client) ListBookmarks(ctx context.Context, filter BookmarkFilter) (BookmarkListResult, error) {
	query := make(url.Values)
	if filter.Query != "" {
		query.Set("q", filter.Query)
	}
	if filter.Status != "" {
		query.Set("status", filter.Status)
	}
	for _, tag := range filter.Tags {
		query.Add("tag", tag)
	}
	if filter.Limit > 0 {
		query.Set("limit", strconv.Itoa(filter.Limit))
	}
	if filter.Offset > 0 {
		query.Set("offset", strconv.Itoa(filter.Offset))
	}
	var result BookmarkListResult
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/bookmarks", query, nil, &result)
	return result, err
}

func (c *Client) GetBookmark(ctx context.Context, id int) (Bookmark, error) {
	var bookmark Bookmark
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/bookmarks/%d", id), nil, nil, &bookmark)
	return bookmark, err
}

func (c *Client) CreateBookmark(ctx context.Context, request BookmarkWriteRequest) (Bookmark, error) {
	var bookmark Bookmark
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/bookmarks", nil, request, &bookmark)
	return bookmark, err
}

func (c *Client) UpdateBookmark(ctx context.Context, id int, request BookmarkWriteRequest) (Bookmark, error) {
	var bookmark Bookmark
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/v1/bookmarks/%d", id), nil, request, &bookmark)
	return bookmark, err
}

func (c *Client) SetBookmarkRead(ctx context.Context, id int, request BookmarkReadRequest) (Bookmark, error) {
	var bookmark Bookmark
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/v1/bookmarks/%d/read", id), nil, request, &bookmark)
	return bookmark, err
}

func (c *Client) DeleteBookmark(ctx context.Context, id int) error {
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/bookmarks/%d", id), nil, nil, nil)
}

func (c *Client) ListBookmarkTags(ctx context.Context) ([]string, error) {
	var tags []string
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/bookmarks/tags", nil, nil, &tags)
	return tags, err
}
