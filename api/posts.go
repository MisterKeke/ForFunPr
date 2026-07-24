package api

import (
	"net/http"
	"strconv"
	"strings"

	"currency-wails/backend"
)

type postResponse struct {
	PostedAt string `json:"posted_at"`
	Text     string `json:"text,omitempty"`
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

		var posts []backend.TelegramPost
		if paginated {
			posts, err = app.GetChannelPostsPaginated(channel, before)
		} else {
			posts, err = app.GetChannelPosts(channel)
		}
		if err != nil {
			writeError(w, http.StatusBadGateway, "telegram_failed", "Telegram posts could not be loaded.")
			return
		}

		items := make([]postResponse, 0, len(posts))
		for _, post := range posts {
			if strings.TrimSpace(post.Date) == "" {
				continue
			}
			items = append(items, postResponse{
				PostedAt: post.Date,
				Text:     post.Text,
			})
		}

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

		var videos []backend.YouTubeVideo
		if paginated {
			videos, err = app.GetChannelVideosPaginated(channel, before)
		} else {
			videos, err = app.GetChannelVideos(channel)
		}
		if err != nil {
			writeError(w, http.StatusBadGateway, "youtube_failed", "YouTube posts could not be loaded.")
			return
		}

		items := make([]postResponse, 0, len(videos))
		for _, video := range videos {
			if strings.TrimSpace(video.PublishedAt) == "" {
				continue
			}
			items = append(items, postResponse{
				PostedAt: video.PublishedAt,
				Text:     video.Title,
			})
		}

		sortPostsNewestFirst(items)
		writeJSON(w, http.StatusOK, limitPosts(items))
	}
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
