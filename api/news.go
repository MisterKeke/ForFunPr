package api

import (
	"net/http"
	"sort"
	"strings"

	"currency-wails/backend"
)

// newsResponse intentionally exposes only the favourite channel name and the
// publication time, regardless of whether the item came from Telegram or
// YouTube.
type newsResponse struct {
	ChannelName string `json:"channel_name"`
	PostedAt    string `json:"posted_at"`
	Text        string `json:"text,omitempty"`
}

type newsListResponse struct {
	News []newsResponse `json:"news"`
}

// newsHandler returns the result of the most recent favourite update scan
// performed by the UI. Reading this endpoint does not fetch providers or
// advance any refresh checkpoint.
func newsHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !backendReady(w, app) {
			return
		}

		result := app.GetLastFavoriteUpdateResult()

		// The UI can receive a partial scan because each favourite source is
		// checked independently. Preserve the successful part of that scan.
		if len(result.Errors) > 0 {
			if len(result.Updates) == 0 {
				writeError(
					w,
					http.StatusBadGateway,
					"news_provider_failed",
					"One or more news sources could not be refreshed.",
				)
				return
			}
			w.Header().Set("X-Partial-Result", "true")
		}

		items := make([]newsResponse, 0, len(result.Updates))
		for _, update := range result.Updates {
			channelName := newsChannelName(update)
			postedAt := strings.TrimSpace(update.PublishedAt)
			if channelName == "" || postedAt == "" {
				continue
			}

			items = append(items, newsResponse{
				ChannelName: channelName,
				PostedAt:    postedAt,
				Text:        newsText(update),
			})
		}

		sort.SliceStable(items, func(i, j int) bool {
			return timeAfter(items[i].PostedAt, items[j].PostedAt)
		})

		writeJSON(w, http.StatusOK, newsListResponse{News: items})
	}
}

func newsChannelName(update backend.FavoriteUpdateItem) string {
	if strings.EqualFold(update.Source, "youtube") {
		if channelTitle := strings.TrimSpace(update.ChannelTitle); channelTitle != "" {
			return channelTitle
		}
		return strings.TrimSpace(update.ChannelID)
	}

	return strings.TrimSpace(update.Username)
}

func newsText(update backend.FavoriteUpdateItem) string {
	if strings.EqualFold(update.Source, "youtube") {
		return strings.TrimSpace(update.Title)
	}

	return strings.TrimSpace(update.Preview)
}
