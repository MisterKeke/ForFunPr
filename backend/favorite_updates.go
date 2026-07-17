package backend

import (
	"context"
	"database/sql"
	"fmt"
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

func (a *App) GetInitialFavoriteUpdates() (FavoriteUpdateScanResult, error) {
	return a.scanFavoriteUpdates(favoriteUpdateScanInitial)
}

func (a *App) RefreshFavoriteUpdates() (FavoriteUpdateScanResult, error) {
	return a.scanFavoriteUpdates(favoriteUpdateScanRefresh)
}

func (a *App) GetFavoriteUpdatesSinceLastOpen() (FavoriteUpdateScanResult, error) {
	return a.GetInitialFavoriteUpdates()
}

func (a *App) scanFavoriteUpdates(scanType string) (FavoriteUpdateScanResult, error) {
	a.favoriteUpdateMu.Lock()
	defer a.favoriteUpdateMu.Unlock()

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

	a.scanTelegramFavoriteUpdates(&result, defaultCheckedThrough, scanStartedAt)
	a.scanYouTubeFavoriteUpdates(&result, defaultCheckedThrough, scanStartedAt)

	sort.SliceStable(result.Updates, func(i, j int) bool {
		return favoriteUpdateTimeBefore(result.Updates[j].PublishedAt, result.Updates[i].PublishedAt)
	})

	if scanType == favoriteUpdateScanRefresh {
		if err := a.recordFavoriteUpdateRefresh(scanStartedAtText); err != nil {
			return result, err
		}
	}

	state, err := a.GetFavoriteUpdateState()
	if err != nil {
		return result, err
	}
	result.State = state

	return result, nil
}

func (a *App) scanTelegramFavoriteUpdates(result *FavoriteUpdateScanResult, defaultCheckedThrough string, scanStartedAt time.Time) {
	sources, err := a.listTelegramFavoriteUpdateSources()
	if err != nil {
		result.Errors = append(result.Errors, FavoriteUpdateError{
			Source: favoriteSourceTelegram,
			Error:  fmt.Sprintf("failed to list Telegram favorites: %v", err),
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
				Error:    fmt.Sprintf("failed to read checkpoint: %v", err),
			})
			continue
		}

		sourceHasSeenItems, err := a.favoriteUpdateSourceHasSeenItems(source)
		if err != nil {
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    fmt.Sprintf("failed to read seen Telegram posts: %v", err),
			})
			continue
		}

		fetches = append(fetches, telegramFavoriteFetch{
			source:             source,
			checkedThrough:     checkedThrough,
			sourceHasSeenItems: sourceHasSeenItems,
		})
	}

	runBounded(a.requestContext(), len(fetches), favoriteRefreshWorkerLimit, func(ctx context.Context, index int) {
		posts, fetchErr := a.getChannelPosts(ctx, fetches[index].source.SourceID, false)
		fetches[index].posts = posts
		fetches[index].err = fetchErr
	})

	for _, fetch := range fetches {
		source := fetch.source
		if fetch.err != nil {
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    fetch.err.Error(),
			})
			if err := a.recordFavoriteUpdateFailure(source, fetch.checkedThrough, scanStartedAt, fetch.err); err != nil {
				result.Errors = append(result.Errors, FavoriteUpdateError{
					Source:   source.Source,
					SourceID: source.SourceID,
					Error:    fmt.Sprintf("failed to record failed Telegram refresh checkpoint: %v", err),
				})
			}
			continue
		}

		updates, err := a.persistTelegramFavoriteUpdateSource(fetch, defaultCheckedThrough, scanStartedAt)
		if err != nil {
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    fmt.Sprintf("failed to persist Telegram updates and checkpoint: %v", err),
			})
			continue
		}

		result.Updates = append(result.Updates, updates...)
	}
}

func (a *App) scanYouTubeFavoriteUpdates(result *FavoriteUpdateScanResult, defaultCheckedThrough string, scanStartedAt time.Time) {
	sources, err := a.listYouTubeFavoriteUpdateSources()
	if err != nil {
		result.Errors = append(result.Errors, FavoriteUpdateError{
			Source: favoriteSourceYouTube,
			Error:  fmt.Sprintf("failed to list YouTube favorites: %v", err),
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
				Error:    fmt.Sprintf("failed to read checkpoint: %v", err),
			})
			continue
		}

		sourceHasSeenItems, err := a.favoriteUpdateSourceHasSeenItems(source)
		if err != nil {
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    fmt.Sprintf("failed to read seen YouTube videos: %v", err),
			})
			continue
		}

		fetches = append(fetches, youTubeFavoriteFetch{
			source:             source,
			checkedThrough:     checkedThrough,
			sourceHasSeenItems: sourceHasSeenItems,
		})
	}

	runBounded(a.requestContext(), len(fetches), favoriteRefreshWorkerLimit, func(ctx context.Context, index int) {
		videos, fetchErr := a.getChannelVideos(ctx, fetches[index].source.SourceID, false)
		fetches[index].videos = videos
		fetches[index].err = fetchErr
	})

	for _, fetch := range fetches {
		source := fetch.source
		if fetch.err != nil {
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    fetch.err.Error(),
			})
			if err := a.recordFavoriteUpdateFailure(source, fetch.checkedThrough, scanStartedAt, fetch.err); err != nil {
				result.Errors = append(result.Errors, FavoriteUpdateError{
					Source:   source.Source,
					SourceID: source.SourceID,
					Error:    fmt.Sprintf("failed to record failed YouTube refresh checkpoint: %v", err),
				})
			}
			continue
		}

		updates, err := a.persistYouTubeFavoriteUpdateSource(fetch, defaultCheckedThrough, scanStartedAt)
		if err != nil {
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    fmt.Sprintf("failed to persist YouTube updates and checkpoint: %v", err),
			})
			continue
		}

		result.Updates = append(result.Updates, updates...)
	}
}

func (a *App) persistTelegramFavoriteUpdateSource(
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
			PostURL:        telegramPostURL(fetch.source.SourceID, post.PostID),
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

func (a *App) persistYouTubeFavoriteUpdateSource(
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

func (a *App) listTelegramFavoriteUpdateSources() ([]favoriteUpdateSource, error) {
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

func (a *App) listYouTubeFavoriteUpdateSources() ([]favoriteUpdateSource, error) {
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

func (a *App) favoriteCheckedThrough(source favoriteUpdateSource, fallback string) (string, error) {
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

func (a *App) recordFavoriteUpdateFailure(source favoriteUpdateSource, checkedThrough string, scanStartedAt time.Time, fetchErr error) error {
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

func (a *App) favoriteUpdateSourceHasSeenItems(source favoriteUpdateSource) (bool, error) {
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

func (a *App) defaultFavoriteCheckedThrough(scanType string, scanStartedAt string) (string, error) {
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

func (a *App) recordFavoriteUpdateRefresh(scanStartedAt string) error {
	state, err := a.getAppState()
	if err != nil {
		return err
	}

	previousRefreshAt := firstNonEmpty(state.LastRefreshAt, state.CurrentOpenedAt, scanStartedAt)

	if err := a.upsertAppStateValue("previous_refresh_at", previousRefreshAt); err != nil {
		return err
	}

	return a.upsertAppStateValue("last_refresh_at", scanStartedAt)
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

func telegramPostURL(username string, postID string) string {
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
