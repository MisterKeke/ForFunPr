package api

import (
	"net/http"
	"strings"

	"currency-wails/backend"
)

type newsListResponse struct {
	ScanStartedAt string                        `json:"scan_started_at"`
	News          []postResponse                `json:"news"`
	Errors        []backend.FavoriteUpdateError `json:"errors"`
	State         backend.FavoriteUpdateState   `json:"state"`
}

// newsHandler returns the result of the most recent favourite update scan
// performed by the UI. Reading this endpoint does not fetch providers or
// advance any refresh checkpoint.
func newsHandler(app *backend.App) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !backendReady(w, app) {
			return
		}

		writeFavoriteUpdateScanResult(w, app.GetLastFavoriteUpdateResult())
	}
}

func initialFavoriteUpdatesHandler(app *backend.App) http.HandlerFunc {
	return favoriteUpdateScanHandler(
		app,
		app.GetInitialFavoriteUpdates,
		"news_initial_scan_failed",
		"The initial favourite update scan could not be completed.",
	)
}

func refreshFavoriteUpdatesHandler(app *backend.App) http.HandlerFunc {
	return favoriteUpdateScanHandler(
		app,
		app.RefreshFavoriteUpdates,
		"news_refresh_failed",
		"Favourite updates could not be refreshed.",
	)
}

func favoriteUpdatesSinceLastOpenHandler(app *backend.App) http.HandlerFunc {
	return favoriteUpdateScanHandler(
		app,
		app.GetFavoriteUpdatesSinceLastOpen,
		"news_since_last_open_failed",
		"Favourite updates since the last open could not be scanned.",
	)
}

func favoriteUpdateScanHandler(
	app *backend.App,
	scan func() (backend.FavoriteUpdateScanResult, error),
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

		result, err := scan()
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

	items := make([]postResponse, 0, len(result.Updates))
	for _, update := range result.Updates {
		if item, ok := newsPostResponse(update); ok {
			items = append(items, item)
		}
	}

	sortPostsNewestFirst(items)

	writeJSON(w, http.StatusOK, newsListResponse{
		ScanStartedAt: result.ScanStartedAt,
		News:          items,
		Errors:        result.Errors,
		State:         result.State,
	})
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
