package api

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"currency-wails/backend"
)

const favoritePostFetchConcurrency = 4

type postResponse struct {
	Source       string   `json:"source"`
	ChannelName  string   `json:"channel_name"`
	PostedAt     string   `json:"posted_at"`
	Text         string   `json:"text,omitempty"`
	Images       []string `json:"images,omitempty"`
	Views        string   `json:"views,omitempty"`
	PostID       string   `json:"post_id,omitempty"`
	PostURL      string   `json:"post_url,omitempty"`
	VideoID      string   `json:"video_id,omitempty"`
	Title        string   `json:"title,omitempty"`
	Description  string   `json:"description,omitempty"`
	Thumbnail    string   `json:"thumbnail,omitempty"`
	ChannelID    string   `json:"channel_id,omitempty"`
	ChannelTitle string   `json:"channel_title,omitempty"`
	VideoURL     string   `json:"video_url,omitempty"`
	Duration     string   `json:"duration,omitempty"`
}

type favoritePostFetchResult struct {
	items []postResponse
	err   error
}

func telegramPostsHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		channel := strings.TrimSpace(r.PathValue("channel"))
		if channel == "" {
			writeError(w, http.StatusBadRequest, "invalid_channel", "A Telegram channel username is required.")
			return
		}

		before, paginated, err := paginationCursor(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_before", "Before must be a non-negative integer.")
			return
		}

		posts, err := loadTelegramPosts(app, channel, before, paginated)
		if err != nil {
			writeError(w, http.StatusBadGateway, "telegram_failed", "Telegram posts could not be loaded.")
			return
		}

		items := telegramPostResponses(channel, posts)
		sortPostsNewestFirst(items)
		writeJSON(w, http.StatusOK, limitPosts(items))
	}
}

func youtubePostsHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		channel := strings.TrimSpace(r.PathValue("channel"))
		if channel == "" {
			writeError(w, http.StatusBadRequest, "invalid_channel", "A YouTube channel handle or channel ID is required.")
			return
		}

		before, paginated, err := paginationCursor(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_before", "Before must be a non-negative integer.")
			return
		}

		videos, err := loadYouTubePosts(app, channel, before, paginated)
		if err != nil {
			writeError(w, http.StatusBadGateway, "youtube_failed", "YouTube posts could not be loaded.")
			return
		}

		items := youtubePostResponses(channel, videos)
		sortPostsNewestFirst(items)
		writeJSON(w, http.StatusOK, limitPosts(items))
	}
}

func favoriteTelegramPostsHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		before, paginated, err := paginationCursor(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_before", "Before must be a non-negative integer.")
			return
		}

		channels, err := app.ListTelegramFavorites()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "telegram_favorites_failed", "Telegram favourites could not be loaded.")
			return
		}

		items, failures := fetchFavoritePosts(
			channels,
			func(channel string) ([]postResponse, error) {
				posts, err := loadTelegramPosts(
					app,
					channel,
					before,
					paginated,
				)
				if err != nil {
					return nil, err
				}
				return limitPosts(
					telegramPostResponses(channel, posts),
				), nil
			},
		)
		if failures == len(channels) && len(channels) > 0 {
			writeError(w, http.StatusBadGateway, "telegram_favorite_posts_failed", "Posts from Telegram favourites could not be loaded.")
			return
		}
		if failures > 0 {
			w.Header().Set("X-Partial-Result", "true")
		}

		sortPostsNewestFirst(items)
		writeJSON(w, http.StatusOK, items)
	}
}

func favoriteYouTubePostsHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		before, paginated, err := paginationCursor(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_before", "Before must be a non-negative integer.")
			return
		}

		channels, err := app.ListYouTubeFavorites()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "youtube_favorites_failed", "YouTube favourites could not be loaded.")
			return
		}

		items, failures := fetchFavoritePosts(
			channels,
			func(channel string) ([]postResponse, error) {
				videos, err := loadYouTubePosts(
					app,
					channel,
					before,
					paginated,
				)
				if err != nil {
					return nil, err
				}
				return limitPosts(
					youtubePostResponses(channel, videos),
				), nil
			},
		)
		if failures == len(channels) && len(channels) > 0 {
			writeError(w, http.StatusBadGateway, "youtube_favorite_posts_failed", "Posts from YouTube favourites could not be loaded.")
			return
		}
		if failures > 0 {
			w.Header().Set("X-Partial-Result", "true")
		}

		sortPostsNewestFirst(items)
		writeJSON(w, http.StatusOK, items)
	}
}

func loadTelegramPosts(
	app *backend.App,
	channel string,
	before int,
	paginated bool,
) ([]backend.TelegramPost, error) {
	if paginated {
		return app.GetChannelPostsPaginated(channel, before)
	}
	return app.GetChannelPosts(channel)
}

func loadYouTubePosts(
	app *backend.App,
	channel string,
	before int,
	paginated bool,
) ([]backend.YouTubeVideo, error) {
	if paginated {
		return app.GetChannelVideosPaginated(channel, before)
	}
	return app.GetChannelVideos(channel)
}

func telegramPostResponses(
	channel string,
	posts []backend.TelegramPost,
) []postResponse {
	items := make([]postResponse, 0, len(posts))
	for _, post := range posts {
		if strings.TrimSpace(post.Date) == "" {
			continue
		}
		items = append(items, postResponse{
			Source:      "telegram",
			ChannelName: channel,
			PostedAt:    post.Date,
			Text:        post.Text,
			Images:      post.Images,
			Views:       post.Views,
			PostID:      post.PostID,
			PostURL:     backend.TelegramPostURL(channel, post.PostID),
		})
	}
	return items
}

func youtubePostResponses(
	requestedChannel string,
	videos []backend.YouTubeVideo,
) []postResponse {
	items := make([]postResponse, 0, len(videos))
	for _, video := range videos {
		if strings.TrimSpace(video.PublishedAt) == "" {
			continue
		}

		channelName := strings.TrimSpace(video.ChannelTitle)
		if channelName == "" {
			channelName = requestedChannel
		}

		items = append(items, postResponse{
			Source:       "youtube",
			ChannelName:  channelName,
			PostedAt:     video.PublishedAt,
			Text:         video.Title,
			Views:        video.Views,
			VideoID:      video.VideoID,
			Title:        video.Title,
			Description:  video.Description,
			Thumbnail:    video.Thumbnail,
			ChannelID:    video.ChannelID,
			ChannelTitle: video.ChannelTitle,
			VideoURL:     video.VideoURL,
			Duration:     video.Duration,
		})
	}
	return items
}

func fetchFavoritePosts(
	channels []string,
	fetch func(string) ([]postResponse, error),
) ([]postResponse, int) {
	if len(channels) == 0 {
		return []postResponse{}, 0
	}

	results := make(chan favoritePostFetchResult, len(channels))
	semaphore := make(chan struct{}, favoritePostFetchConcurrency)
	var waitGroup sync.WaitGroup

	for _, channel := range channels {
		channel := channel
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			items, err := fetch(channel)
			results <- favoritePostFetchResult{items: items, err: err}
		}()
	}

	waitGroup.Wait()
	close(results)

	items := make([]postResponse, 0)
	failures := 0
	for result := range results {
		if result.err != nil {
			failures++
			continue
		}
		items = append(items, result.items...)
	}

	return items, failures
}

func paginationCursor(r *http.Request) (int, bool, error) {
	value, present := r.URL.Query()["before"]
	if !present {
		return 0, false, nil
	}

	cursorText := ""
	if len(value) > 0 {
		cursorText = strings.TrimSpace(value[0])
	}
	cursor, err := strconv.Atoi(cursorText)
	if err != nil || cursor < 0 {
		return 0, true, strconv.ErrSyntax
	}

	return cursor, true, nil
}
