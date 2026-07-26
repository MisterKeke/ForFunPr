package backend

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	favoriteUpdateScanInitial = "initial"
	favoriteUpdateScanRefresh = "refresh"
)

type FavoriteUpdateScanResult struct {
	ScanStartedAt string                `json:"scan_started_at"`
	Updates       []FavoriteUpdateItem  `json:"updates"`
	Errors        []FavoriteUpdateError `json:"errors"`
	State         FavoriteUpdateState   `json:"state"`
}

type FavoriteUpdateItem struct {
	Source         string `json:"source"`
	CheckedThrough string `json:"checked_through"`
	PublishedAt    string `json:"publishedAt"`

	Username string   `json:"username,omitempty"`
	PostID   string   `json:"postId,omitempty"`
	Preview  string   `json:"preview,omitempty"`
	Images   []string `json:"images,omitempty"`
	Views    string   `json:"views,omitempty"`
	PostURL  string   `json:"postUrl,omitempty"`

	ChannelID    string `json:"channelId,omitempty"`
	ChannelTitle string `json:"channelTitle,omitempty"`
	VideoID      string `json:"videoId,omitempty"`
	Title        string `json:"title,omitempty"`
	Thumbnail    string `json:"thumbnail,omitempty"`
	VideoURL     string `json:"videoUrl,omitempty"`
	Description  string `json:"description,omitempty"`
	Duration     string `json:"duration,omitempty"`
}

type FavoriteUpdateError struct {
	Source   string `json:"source"`
	SourceID string `json:"source_id,omitempty"`
	Error    string `json:"error"`
}

type favoriteUpdateSource struct {
	Source   string
	SourceID string
	AddedAt  string
}

type telegramFavoriteFetch struct {
	source             favoriteUpdateSource
	checkedThrough     string
	sourceHasSeenItems bool
	posts              []TelegramPost
	err                error
}

type youTubeFavoriteFetch struct {
	source             favoriteUpdateSource
	checkedThrough     string
	sourceHasSeenItems bool
	videos             []YouTubeVideo
	err                error
}

type favoriteUpdateStore interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

func (a *Service) GetInitialFavoriteUpdates() (FavoriteUpdateScanResult, error) {
	return a.GetInitialFavoriteUpdatesContext(a.requestContext())
}

func (a *Service) GetInitialFavoriteUpdatesContext(ctx context.Context) (FavoriteUpdateScanResult, error) {
	return a.scanFavoriteUpdates(ctx, favoriteUpdateScanInitial)
}

func (a *Service) RefreshFavoriteUpdates() (FavoriteUpdateScanResult, error) {
	return a.RefreshFavoriteUpdatesContext(a.requestContext())
}

func (a *Service) RefreshFavoriteUpdatesContext(ctx context.Context) (FavoriteUpdateScanResult, error) {
	return a.scanFavoriteUpdates(ctx, favoriteUpdateScanRefresh)
}

func (a *Service) GetFavoriteUpdatesSinceLastOpen() (FavoriteUpdateScanResult, error) {
	return a.GetFavoriteUpdatesSinceLastOpenContext(a.requestContext())
}

func (a *Service) GetFavoriteUpdatesSinceLastOpenContext(ctx context.Context) (FavoriteUpdateScanResult, error) {
	return a.GetInitialFavoriteUpdatesContext(ctx)
}

// GetLastFavoriteUpdateResult returns a copy of the most recent completed
// favourite update scan without fetching providers or advancing checkpoints.
func (a *Service) GetLastFavoriteUpdateResult() FavoriteUpdateScanResult {
	a.lastFavoriteUpdateMu.RLock()
	defer a.lastFavoriteUpdateMu.RUnlock()

	return cloneFavoriteUpdateScanResult(a.lastFavoriteUpdateResult)
}

func (a *Service) scanFavoriteUpdates(ctx context.Context, scanType string) (FavoriteUpdateScanResult, error) {
	a.favoriteUpdateMu.Lock()
	defer a.favoriteUpdateMu.Unlock()
	if err := ctx.Err(); err != nil {
		return FavoriteUpdateScanResult{}, err
	}

	scanStartedAt := time.Now().UTC()
	scanStartedAtText := scanStartedAt.Format(time.RFC3339)

	result := FavoriteUpdateScanResult{
		ScanStartedAt: scanStartedAtText,
		Updates:       []FavoriteUpdateItem{},
		Errors:        []FavoriteUpdateError{},
	}

	defaultCheckedThrough, err := a.defaultFavoriteCheckedThrough(scanType, scanStartedAtText)
	if err != nil {
		return result, err
	}

	a.scanTelegramFavoriteUpdates(ctx, &result, defaultCheckedThrough, scanStartedAt)
	if err := ctx.Err(); err != nil { return result, err }
	a.scanYouTubeFavoriteUpdates(ctx, &result, defaultCheckedThrough, scanStartedAt)
	if err := ctx.Err(); err != nil { return result, err }

	sort.SliceStable(result.Updates, func(i, j int) bool {
		return favoriteUpdateTimeBefore(result.Updates[j].PublishedAt, result.Updates[i].PublishedAt)
	})

	if scanType == favoriteUpdateScanRefresh {
		if err := a.recordFavoriteUpdateRefresh(ctx, scanStartedAtText); err != nil {
			return result, err
		}
	}

	state, err := a.GetFavoriteUpdateState()
	if err != nil {
		return result, err
	}
	result.State = state
	a.storeLastFavoriteUpdateResult(result)

	return result, nil
}

func (a *Service) storeLastFavoriteUpdateResult(result FavoriteUpdateScanResult) {
	a.lastFavoriteUpdateMu.Lock()
	defer a.lastFavoriteUpdateMu.Unlock()

	a.lastFavoriteUpdateResult = cloneFavoriteUpdateScanResult(result)
}

func cloneFavoriteUpdateScanResult(result FavoriteUpdateScanResult) FavoriteUpdateScanResult {
	clone := result
	clone.Updates = append([]FavoriteUpdateItem(nil), result.Updates...)
	for index := range clone.Updates {
		clone.Updates[index].Images = append([]string(nil), result.Updates[index].Images...)
	}
	clone.Errors = append([]FavoriteUpdateError(nil), result.Errors...)

	if clone.Updates == nil {
		clone.Updates = []FavoriteUpdateItem{}
	}
	if clone.Errors == nil {
		clone.Errors = []FavoriteUpdateError{}
	}

	return clone
}

func (a *Service) scanTelegramFavoriteUpdates(ctx context.Context, result *FavoriteUpdateScanResult, defaultCheckedThrough string, scanStartedAt time.Time) {
	sources, err := a.listTelegramFavoriteUpdateSources()
	if err != nil {
		result.Errors = append(result.Errors, FavoriteUpdateError{
			Source: favoriteSourceTelegram,
			Error:  safeFavoriteInternalError("Telegram favorites could not be read", err),
		})
		return
	}

	fetches := make([]telegramFavoriteFetch, 0, len(sources))
	for _, source := range sources {
		checkedThrough, err := a.favoriteCheckedThrough(source, defaultCheckedThrough)
		if err != nil {
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    safeFavoriteInternalError("The Telegram refresh checkpoint could not be read", err),
			})
			continue
		}

		sourceHasSeenItems, err := a.favoriteUpdateSourceHasSeenItems(source)
		if err != nil {
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    safeFavoriteInternalError("Seen Telegram posts could not be read", err),
			})
			continue
		}

		fetches = append(fetches, telegramFavoriteFetch{
			source:             source,
			checkedThrough:     checkedThrough,
			sourceHasSeenItems: sourceHasSeenItems,
		})
	}

	runBounded(ctx, len(fetches), favoriteRefreshWorkerLimit, func(ctx context.Context, index int) {
		posts, fetchErr := a.getChannelPosts(ctx, fetches[index].source.SourceID, false)
		fetches[index].posts = posts
		fetches[index].err = fetchErr
	})

	for _, fetch := range fetches {
		if ctx.Err() != nil { return }
		source := fetch.source
		if fetch.err != nil {
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    safeFavoriteProviderError(providerTelegram, fetch.err),
			})
			if err := a.recordFavoriteUpdateFailure(source, fetch.checkedThrough, scanStartedAt, fetch.err); err != nil {
				result.Errors = append(result.Errors, FavoriteUpdateError{
					Source:   source.Source,
					SourceID: source.SourceID,
					Error:    safeFavoriteInternalError("The Telegram failure checkpoint could not be saved", err),
				})
			}
			continue
		}

		updates, err := a.persistTelegramFavoriteUpdateSource(fetch, defaultCheckedThrough, scanStartedAt)
		if err != nil {
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    safeFavoriteInternalError("Telegram updates could not be saved", err),
			})
			continue
		}

		result.Updates = append(result.Updates, updates...)
	}
}

func (a *Service) scanYouTubeFavoriteUpdates(ctx context.Context, result *FavoriteUpdateScanResult, defaultCheckedThrough string, scanStartedAt time.Time) {
	sources, err := a.listYouTubeFavoriteUpdateSources()
	if err != nil {
		result.Errors = append(result.Errors, FavoriteUpdateError{
			Source: favoriteSourceYouTube,
			Error:  safeFavoriteInternalError("YouTube favorites could not be read", err),
		})
		return
	}

	fetches := make([]youTubeFavoriteFetch, 0, len(sources))
	for _, source := range sources {
		checkedThrough, err := a.favoriteCheckedThrough(source, defaultCheckedThrough)
		if err != nil {
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    safeFavoriteInternalError("The YouTube refresh checkpoint could not be read", err),
			})
			continue
		}

		sourceHasSeenItems, err := a.favoriteUpdateSourceHasSeenItems(source)
		if err != nil {
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    safeFavoriteInternalError("Seen YouTube videos could not be read", err),
			})
			continue
		}

		fetches = append(fetches, youTubeFavoriteFetch{
			source:             source,
			checkedThrough:     checkedThrough,
			sourceHasSeenItems: sourceHasSeenItems,
		})
	}

	runBounded(ctx, len(fetches), favoriteRefreshWorkerLimit, func(ctx context.Context, index int) {
		videos, fetchErr := a.getChannelVideos(ctx, fetches[index].source.SourceID, false)
		fetches[index].videos = videos
		fetches[index].err = fetchErr
	})

	for _, fetch := range fetches {
		if ctx.Err() != nil { return }
		source := fetch.source
		if fetch.err != nil {
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    safeFavoriteProviderError(providerYouTube, fetch.err),
			})
			if err := a.recordFavoriteUpdateFailure(source, fetch.checkedThrough, scanStartedAt, fetch.err); err != nil {
				result.Errors = append(result.Errors, FavoriteUpdateError{
					Source:   source.Source,
					SourceID: source.SourceID,
					Error:    safeFavoriteInternalError("The YouTube failure checkpoint could not be saved", err),
				})
			}
			continue
		}

		updates, err := a.persistYouTubeFavoriteUpdateSource(fetch, defaultCheckedThrough, scanStartedAt)
		if err != nil {
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    safeFavoriteInternalError("YouTube updates could not be saved", err),
			})
			continue
		}

		result.Updates = append(result.Updates, updates...)
	}
}

func (a *Service) persistTelegramFavoriteUpdateSource(
	fetch telegramFavoriteFetch,
	defaultCheckedThrough string,
	scanStartedAt time.Time,
) ([]FavoriteUpdateItem, error) {
	tx, err := a.db.BeginTx(a.requestContext(), nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	updates := make([]FavoriteUpdateItem, 0, len(fetch.posts))
	for _, post := range fetch.posts {
		publishedAt, ok := parseFavoriteUpdateTime(post.Date)
		if !ok {
			continue
		}

		firstSeenAt, isNewSeenItem, err := recordFavoriteUpdateSeenItem(
			tx,
			fetch.source,
			post.PostID,
			publishedAt,
			fetch.checkedThrough,
			scanStartedAt,
			fetch.sourceHasSeenItems,
		)
		if err != nil {
			return nil, fmt.Errorf("record seen post: %w", err)
		}
		if !isNewSeenItem || !favoriteUpdateInWindow(firstSeenAt, defaultCheckedThrough, scanStartedAt) {
			continue
		}

		updates = append(updates, FavoriteUpdateItem{
			Source:         favoriteSourceTelegram,
			CheckedThrough: fetch.checkedThrough,
			PublishedAt:    publishedAt.Format(time.RFC3339),
			Username:       fetch.source.SourceID,
			PostID:         post.PostID,
			Preview:        post.Text,
			Images:         post.Images,
			Views:          post.Views,
			PostURL:        TelegramPostURL(fetch.source.SourceID, post.PostID),
		})
	}

	if err := recordFavoriteUpdateSuccess(tx, fetch.source, scanStartedAt); err != nil {
		return nil, fmt.Errorf("advance checkpoint: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return updates, nil
}

func (a *Service) persistYouTubeFavoriteUpdateSource(
	fetch youTubeFavoriteFetch,
	defaultCheckedThrough string,
	scanStartedAt time.Time,
) ([]FavoriteUpdateItem, error) {
	tx, err := a.db.BeginTx(a.requestContext(), nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	updates := make([]FavoriteUpdateItem, 0, len(fetch.videos))
	for _, video := range fetch.videos {
		publishedAt, ok := parseFavoriteUpdateTime(video.PublishedAt)
		if !ok {
			continue
		}

		firstSeenAt, isNewSeenItem, err := recordFavoriteUpdateSeenItem(
			tx,
			fetch.source,
			video.VideoID,
			publishedAt,
			fetch.checkedThrough,
			scanStartedAt,
			fetch.sourceHasSeenItems,
		)
		if err != nil {
			return nil, fmt.Errorf("record seen video: %w", err)
		}
		if !isNewSeenItem || !favoriteUpdateInWindow(firstSeenAt, defaultCheckedThrough, scanStartedAt) {
			continue
		}

		updates = append(updates, FavoriteUpdateItem{
			Source:         favoriteSourceYouTube,
			CheckedThrough: fetch.checkedThrough,
			PublishedAt:    publishedAt.Format(time.RFC3339),
			ChannelID:      fetch.source.SourceID,
			ChannelTitle:   video.ChannelTitle,
			VideoID:        video.VideoID,
			Title:          video.Title,
			Thumbnail:      video.Thumbnail,
			VideoURL:       video.VideoURL,
			Description:    video.Description,
			Duration:       video.Duration,
			Views:          video.Views,
		})
	}

	if err := recordFavoriteUpdateSuccess(tx, fetch.source, scanStartedAt); err != nil {
		return nil, fmt.Errorf("advance checkpoint: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return updates, nil
}

func (a *Service) listTelegramFavoriteUpdateSources() ([]favoriteUpdateSource, error) {
	rows, err := a.db.Query(`
		SELECT username, added_at
		FROM telegram_favorites
		ORDER BY added_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFavoriteUpdateSources(rows, favoriteSourceTelegram)
}

func (a *Service) listYouTubeFavoriteUpdateSources() ([]favoriteUpdateSource, error) {
	rows, err := a.db.Query(`
		SELECT channel_id, added_at
		FROM youtube_favorites
		ORDER BY added_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFavoriteUpdateSources(rows, favoriteSourceYouTube)
}

func scanFavoriteUpdateSources(rows *sql.Rows, source string) ([]favoriteUpdateSource, error) {
	sources := []favoriteUpdateSource{}

	for rows.Next() {
		var sourceID string
		var addedAt string

		if err := rows.Scan(&sourceID, &addedAt); err != nil {
			return nil, err
		}

		sourceID = strings.TrimSpace(sourceID)
		if sourceID == "" {
			continue
		}

		sources = append(sources, favoriteUpdateSource{
			Source:   source,
			SourceID: sourceID,
			AddedAt:  addedAt,
		})
	}

	return sources, rows.Err()
}

func (a *Service) favoriteCheckedThrough(source favoriteUpdateSource, fallback string) (string, error) {
	var checkedThrough string

	err := a.db.QueryRow(`
		SELECT checked_through
		FROM favorite_update_checkpoints
		WHERE source = ? AND source_id = ?
	`, source.Source, source.SourceID).Scan(&checkedThrough)

	if err == nil {
		return checkedThrough, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	return maxFavoriteUpdateTimestamp(fallback, source.AddedAt), nil
}

func recordFavoriteUpdateSuccess(store favoriteUpdateStore, source favoriteUpdateSource, scanStartedAt time.Time) error {
	value := scanStartedAt.UTC().Format(time.RFC3339)

	_, err := store.Exec(`
		INSERT INTO favorite_update_checkpoints (
			source,
			source_id,
			checked_through,
			last_success_at,
			last_attempted_at,
			last_error,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, NULL, CURRENT_TIMESTAMP)
		ON CONFLICT(source, source_id) DO UPDATE SET
			checked_through = excluded.checked_through,
			last_success_at = excluded.last_success_at,
			last_attempted_at = excluded.last_attempted_at,
			last_error = NULL,
			updated_at = CURRENT_TIMESTAMP
	`, source.Source, source.SourceID, value, value, value)

	return err
}

func (a *Service) recordFavoriteUpdateFailure(source favoriteUpdateSource, checkedThrough string, scanStartedAt time.Time, fetchErr error) error {
	return recordFavoriteUpdateFailure(a.db, source, checkedThrough, scanStartedAt, fetchErr)
}

func recordFavoriteUpdateFailure(store favoriteUpdateStore, source favoriteUpdateSource, checkedThrough string, scanStartedAt time.Time, fetchErr error) error {
	attemptedAt := scanStartedAt.UTC().Format(time.RFC3339)

	_, err := store.Exec(`
		INSERT INTO favorite_update_checkpoints (
			source,
			source_id,
			checked_through,
			last_attempted_at,
			last_error,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(source, source_id) DO UPDATE SET
			last_attempted_at = excluded.last_attempted_at,
			last_error = excluded.last_error,
			updated_at = CURRENT_TIMESTAMP
	`, source.Source, source.SourceID, checkedThrough, attemptedAt, fetchErr.Error())

	return err
}

func recordFavoriteUpdateSeenItem(
	store favoriteUpdateStore,
	source favoriteUpdateSource,
	itemID string,
	publishedAt time.Time,
	checkedThrough string,
	scanStartedAt time.Time,
	sourceHasSeenItems bool,
) (time.Time, bool, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return time.Time{}, false, nil
	}

	var firstSeenAtText string
	err := store.QueryRow(`
		SELECT first_seen_at
		FROM favorite_update_seen_items
		WHERE source = ? AND source_id = ? AND item_id = ?
	`, source.Source, source.SourceID, itemID).Scan(&firstSeenAtText)

	if err == nil {
		firstSeenAt, ok := parseFavoriteUpdateTime(firstSeenAtText)
		if !ok {
			firstSeenAt = scanStartedAt
		}
		return firstSeenAt, false, nil
	}
	if err != sql.ErrNoRows {
		return time.Time{}, false, err
	}

	firstSeenAt := scanStartedAt
	checked, checkedOK := parseFavoriteUpdateTime(checkedThrough)
	if !sourceHasSeenItems && checkedOK && !publishedAt.After(checked) {
		firstSeenAt = checked
	}

	firstSeenAtText = firstSeenAt.UTC().Format(time.RFC3339)
	publishedAtText := publishedAt.UTC().Format(time.RFC3339)

	_, err = store.Exec(`
		INSERT INTO favorite_update_seen_items (
			source,
			source_id,
			item_id,
			published_at,
			first_seen_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(source, source_id, item_id) DO UPDATE SET
			published_at = excluded.published_at,
			updated_at = CURRENT_TIMESTAMP
	`, source.Source, source.SourceID, itemID, publishedAtText, firstSeenAtText)
	if err != nil {
		return time.Time{}, false, err
	}

	return firstSeenAt, true, nil
}

func (a *Service) favoriteUpdateSourceHasSeenItems(source favoriteUpdateSource) (bool, error) {
	var existing int
	err := a.db.QueryRow(`
		SELECT 1
		FROM favorite_update_seen_items
		WHERE source = ? AND source_id = ?
		LIMIT 1
	`, source.Source, source.SourceID).Scan(&existing)

	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, err
}

func (a *Service) defaultFavoriteCheckedThrough(scanType string, scanStartedAt string) (string, error) {
	state, err := a.getAppState()
	if err != nil {
		return "", err
	}

	switch scanType {
	case favoriteUpdateScanInitial:
		return firstNonEmpty(state.PreviousOpenedAt, state.CurrentOpenedAt, scanStartedAt), nil
	case favoriteUpdateScanRefresh:
		return firstNonEmpty(state.LastRefreshAt, state.CurrentOpenedAt, scanStartedAt), nil
	default:
		return scanStartedAt, nil
	}
}

func (a *Service) recordFavoriteUpdateRefresh(ctx context.Context, scanStartedAt string) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil { return fmt.Errorf("begin favorite refresh transaction: %w", err) }
	defer tx.Rollback()

	state, err := getAppState(ctx, tx)
	if err != nil {
		return err
	}

	previousRefreshAt := firstNonEmpty(state.LastRefreshAt, state.CurrentOpenedAt, scanStartedAt)

	if err := upsertAppStateValue(ctx, tx, "previous_refresh_at", previousRefreshAt); err != nil {
		return err
	}

	if err := upsertAppStateValue(ctx, tx, "last_refresh_at", scanStartedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func safeFavoriteProviderError(provider string, err error) string {
	slog.Error("Favorite provider refresh failed", "provider", provider, "error", err)
	switch {
	case errors.Is(err, context.Canceled):
		return provider + " request was canceled."
	case errors.Is(err, context.DeadlineExceeded):
		return provider + " request timed out."
	default:
		return provider + " data could not be loaded."
	}
}

func safeFavoriteInternalError(message string, err error) string {
	slog.Error("Favorite refresh storage operation failed", "operation", message, "error", err)
	return message + "."
}

func favoriteUpdateInWindow(publishedAt time.Time, checkedThrough string, scanStartedAt time.Time) bool {
	checked, ok := parseFavoriteUpdateTime(checkedThrough)
	if !ok {
		checked = time.Time{}
	}

	return publishedAt.After(checked) && !publishedAt.After(scanStartedAt)
}

func favoriteUpdateTimeBefore(left string, right string) bool {
	leftTime, leftOK := parseFavoriteUpdateTime(left)
	rightTime, rightOK := parseFavoriteUpdateTime(right)

	if leftOK && rightOK {
		return leftTime.Before(rightTime)
	}

	return left < right
}

func maxFavoriteUpdateTimestamp(left string, right string) string {
	leftTime, leftOK := parseFavoriteUpdateTime(left)
	rightTime, rightOK := parseFavoriteUpdateTime(right)

	if rightOK && (!leftOK || rightTime.After(leftTime)) {
		return rightTime.UTC().Format(time.RFC3339)
	}

	if leftOK {
		return leftTime.UTC().Format(time.RFC3339)
	}

	return firstNonEmpty(left, right)
}

func parseFavoriteUpdateTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed.UTC(), true
	}

	parsed, err = time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed.UTC(), true
	}

	return time.Time{}, false
}

// TelegramPostURL returns the canonical public URL for a Telegram post.
func TelegramPostURL(username string, postID string) string {
	username = normalizeTelegramUsername(username)
	if username == "" {
		return ""
	}

	postID = strings.TrimSpace(postID)
	if postID == "" {
		return ""
	}

	if strings.Contains(postID, "/") {
		parts := strings.Split(strings.Trim(postID, "/"), "/")
		if len(parts) != 2 {
			return ""
		}
		username = normalizeTelegramUsername(parts[0])
		postID = parts[1]
	}

	if username == "" || !telegramPostIDPattern.MatchString(postID) {
		return ""
	}

	return (&url.URL{
		Scheme: "https",
		Host:   "t.me",
	}).JoinPath(username, postID).String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
