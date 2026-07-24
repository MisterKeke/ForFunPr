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
	// A scan can be partial because each favourite source is checked
	// independently. Preserve updates returned by the successful sources.
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
