package api

import (
	"context"
	"net/http"
	"strings"

	backend "something/backend/service"
)

type newsListResponse struct {
	ScanStartedAt string                        `json:"scan_started_at"`
	News          []postResponse                `json:"news"`
	NewNews       []postResponse                `json:"new_news"`
	Errors        []backend.FavoriteUpdateError `json:"errors"`
	State         backend.FavoriteUpdateState   `json:"state"`
}

// newsHandler returns the durable result of the latest news scan. Reading this
// endpoint does not fetch providers or advance any refresh checkpoint.
func newsHandler(app *backend.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		result, err := app.GetCurrentFavoriteUpdatesContext(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "news_list_failed", "Current news could not be loaded.")
			return
		}
		writeFavoriteUpdateScanResult(w, result)
	}
}

func initialFavoriteUpdatesHandler(app *backend.Service) http.HandlerFunc {
	return favoriteUpdateScanHandler(
		app,
		app.GetInitialFavoriteUpdatesContext,
		"news_initial_scan_failed",
		"The initial favourite update scan could not be completed.",
	)
}

func refreshFavoriteUpdatesHandler(app *backend.Service) http.HandlerFunc {
	return favoriteUpdateScanHandler(
		app,
		app.RefreshFavoriteUpdatesContext,
		"news_refresh_failed",
		"Favourite updates could not be refreshed.",
	)
}

func favoriteUpdatesSinceLastOpenHandler(app *backend.Service) http.HandlerFunc {
	return favoriteUpdateScanHandler(
		app,
		app.GetFavoriteUpdatesSinceLastOpenContext,
		"news_since_last_open_failed",
		"Favourite updates since the last open could not be scanned.",
	)
}

func favoriteUpdateScanHandler(
	app *backend.Service,
	scan func(context.Context) (backend.FavoriteUpdateScanResult, error),
	errorCode string,
	errorMessage string,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !backendReady(w, app) {
			return
		}

		// Requiring JSON keeps these state-changing localhost endpoints out of
		// reach of simple cross-origin form requests.
		var request struct{}
		if !decodeJSONBody(w, r, &request) {
			return
		}

		result, err := scan(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, errorCode, errorMessage)
			return
		}

		writeFavoriteUpdateScanResult(w, result)
	}
}

func writeFavoriteUpdateScanResult(w http.ResponseWriter, result backend.FavoriteUpdateScanResult) {
	// Provider failures are part of a completed scan result. Preserve both the
	// successful updates and the source-specific errors so API clients can
	// distinguish a partial scan from an internal scan failure.
	if len(result.Errors) > 0 {
		w.Header().Set("X-Partial-Result", "true")
	}

	items := newsPostResponses(result.Updates)
	newItems := newsPostResponses(result.NewUpdates)

	sortPostsNewestFirst(items)
	sortPostsNewestFirst(newItems)

	writeJSON(w, http.StatusOK, newsListResponse{
		ScanStartedAt: result.ScanStartedAt,
		News:          items,
		NewNews:       newItems,
		Errors:        result.Errors,
		State:         result.State,
	})
}

func newsPostResponses(updates []backend.FavoriteUpdateItem) []postResponse {
	items := make([]postResponse, 0, len(updates))
	for _, update := range updates {
		if item, ok := newsPostResponse(update); ok {
			items = append(items, item)
		}
	}
	return items
}

func newsPostResponse(update backend.FavoriteUpdateItem) (postResponse, bool) {
	source := strings.ToLower(strings.TrimSpace(update.Source))
	channelName := newsChannelName(update)
	postedAt := strings.TrimSpace(update.PublishedAt)
	if channelName == "" || postedAt == "" {
		return postResponse{}, false
	}

	item := postResponse{
		Source:      source,
		ChannelName: channelName,
		PostedAt:    postedAt,
		Views:       update.Views,
	}

	switch source {
	case "telegram":
		item.Text = strings.TrimSpace(update.Preview)
		item.Images = update.Images
		item.PostID = update.PostID
		item.PostURL = update.PostURL
	case "youtube":
		item.Text = strings.TrimSpace(update.Title)
		item.VideoID = update.VideoID
		item.Title = update.Title
		item.Description = update.Description
		item.Thumbnail = update.Thumbnail
		item.ChannelID = update.ChannelID
		item.ChannelTitle = update.ChannelTitle
		item.VideoURL = update.VideoURL
		item.Duration = update.Duration
	default:
		return postResponse{}, false
	}

	return item, true
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
