package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type Post struct {
	Source       string   `json:"source"`
	ChannelName  string   `json:"channel_name"`
	PostedAt     string   `json:"posted_at"`
	Text         string   `json:"text"`
	Images       []string `json:"images"`
	Views        string   `json:"views"`
	PostID       string   `json:"post_id"`
	VideoID      string   `json:"video_id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Thumbnail    string   `json:"thumbnail"`
	ChannelID    string   `json:"channel_id"`
	ChannelTitle string   `json:"channel_title"`
	VideoURL     string   `json:"video_url"`
	Duration     string   `json:"duration"`
}

func (c *Client) TelegramPosts(
	ctx context.Context,
	channel string,
	before *int,
) ([]Post, error) {
	return c.posts(ctx, "telegram", channel, before)
}

func (c *Client) YouTubePosts(
	ctx context.Context,
	channel string,
	before *int,
) ([]Post, error) {
	return c.posts(ctx, "youtube", channel, before)
}

func (c *Client) FavoriteTelegramPosts(
	ctx context.Context,
	before *int,
) ([]Post, error) {
	return c.favoritePosts(ctx, "telegram", before)
}

func (c *Client) FavoriteYouTubePosts(
	ctx context.Context,
	before *int,
) ([]Post, error) {
	return c.favoritePosts(ctx, "youtube", before)
}

func (c *Client) posts(
	ctx context.Context,
	source string,
	channel string,
	before *int,
) ([]Post, error) {
	query := make(url.Values)
	if before != nil {
		query.Set("before", strconv.Itoa(*before))
	}

	var result []Post
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		fmt.Sprintf("/api/v1/posts/%s/%s", source, channel),
		query,
		nil,
		&result,
	); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) favoritePosts(
	ctx context.Context,
	source string,
	before *int,
) ([]Post, error) {
	query := make(url.Values)
	if before != nil {
		query.Set("before", strconv.Itoa(*before))
	}

	var result []Post
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		fmt.Sprintf("/api/v1/posts/favorites/%s", source),
		query,
		nil,
		&result,
	); err != nil {
		return nil, err
	}

	return result, nil
}
