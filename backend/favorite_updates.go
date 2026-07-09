package backend

import (
	"database/sql"
	"fmt"
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
	scanStartedAt := time.Now().UTC()
	scanStartedAtText := scanStartedAt.Format(time.RFC3339)

	result := FavoriteUpdateScanResult{
		ScanStartedAt: scanStartedAtText,
		Updates:       []FavoriteUpdateItem{},
		Errors:        []FavoriteUpdateError{},
	}

	if err := a.ensureFavoriteUpdateCheckpointTable(); err != nil {
		return result, err
	}
	if err := a.ensureFavoriteUpdateSeenItemsTable(); err != nil {
		return result, err
	}

	defaultCheckedThrough, err := a.defaultFavoriteCheckedThrough(scanType, scanStartedAtText)
	if err != nil {
		return result, err
	}

	a.scanTelegramFavoriteUpdates(&result, defaultCheckedThrough, scanStartedAt)
	a.scanYouTubeFavoriteUpdates(&result, defaultCheckedThrough, scanStartedAt)

	sort.SliceStable(result.Updates, func(i, j int) bool {
		return favoriteUpdateTimeBefore(result.Updates[i].PublishedAt, result.Updates[j].PublishedAt)
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

		posts, err := a.getChannelPosts(source.SourceID, false)
		if err != nil {
			_ = a.recordFavoriteUpdateFailure(source, checkedThrough, scanStartedAt, err)
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    err.Error(),
			})
			continue
		}

		for _, post := range posts {
			publishedAt, ok := parseFavoriteUpdateTime(post.Date)
			if !ok {
				continue
			}

			firstSeenAt, isNewSeenItem, err := a.recordFavoriteUpdateSeenItem(
				source,
				post.PostID,
				publishedAt,
				checkedThrough,
				scanStartedAt,
				sourceHasSeenItems,
			)
			if err != nil {
				result.Errors = append(result.Errors, FavoriteUpdateError{
					Source:   source.Source,
					SourceID: source.SourceID,
					Error:    fmt.Sprintf("failed to record seen Telegram post: %v", err),
				})
				continue
			}
			if !isNewSeenItem || !favoriteUpdateInWindow(firstSeenAt, defaultCheckedThrough, scanStartedAt) {
				continue
			}

			result.Updates = append(result.Updates, FavoriteUpdateItem{
				Source:         favoriteSourceTelegram,
				CheckedThrough: checkedThrough,
				PublishedAt:    publishedAt.Format(time.RFC3339),
				Username:       source.SourceID,
				PostID:         post.PostID,
				Preview:        post.Text,
				Images:         post.Images,
				Views:          post.Views,
				PostURL:        telegramPostURL(source.SourceID, post.PostID),
			})
		}

		_ = a.recordFavoriteUpdateSuccess(source, scanStartedAt)
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

		videos, err := a.getChannelVideos(source.SourceID, false)
		if err != nil {
			_ = a.recordFavoriteUpdateFailure(source, checkedThrough, scanStartedAt, err)
			result.Errors = append(result.Errors, FavoriteUpdateError{
				Source:   source.Source,
				SourceID: source.SourceID,
				Error:    err.Error(),
			})
			continue
		}

		for _, video := range videos {
			publishedAt, ok := parseFavoriteUpdateTime(video.PublishedAt)
			if !ok {
				continue
			}

			firstSeenAt, isNewSeenItem, err := a.recordFavoriteUpdateSeenItem(
				source,
				video.VideoID,
				publishedAt,
				checkedThrough,
				scanStartedAt,
				sourceHasSeenItems,
			)
			if err != nil {
				result.Errors = append(result.Errors, FavoriteUpdateError{
					Source:   source.Source,
					SourceID: source.SourceID,
					Error:    fmt.Sprintf("failed to record seen YouTube video: %v", err),
				})
				continue
			}
			if !isNewSeenItem || !favoriteUpdateInWindow(firstSeenAt, defaultCheckedThrough, scanStartedAt) {
				continue
			}

			result.Updates = append(result.Updates, FavoriteUpdateItem{
				Source:         favoriteSourceYouTube,
				CheckedThrough: checkedThrough,
				PublishedAt:    publishedAt.Format(time.RFC3339),
				ChannelID:      source.SourceID,
				ChannelTitle:   video.ChannelTitle,
				VideoID:        video.VideoID,
				Title:          video.Title,
				Thumbnail:      video.Thumbnail,
				VideoURL:       video.VideoURL,
			})
		}

		_ = a.recordFavoriteUpdateSuccess(source, scanStartedAt)
	}
}

func (a *App) ensureFavoriteUpdateCheckpointTable() error {
	_, err := a.db.Exec(`
		CREATE TABLE IF NOT EXISTS favorite_update_checkpoints (
			source TEXT NOT NULL,
			source_id TEXT NOT NULL,
			checked_through TEXT NOT NULL,
			last_success_at TEXT,
			last_attempted_at TEXT,
			last_error TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (source, source_id)
		)
	`)
	return err
}

func (a *App) ensureFavoriteUpdateSeenItemsTable() error {
	_, err := a.db.Exec(`
		CREATE TABLE IF NOT EXISTS favorite_update_seen_items (
			source TEXT NOT NULL,
			source_id TEXT NOT NULL,
			item_id TEXT NOT NULL,
			published_at TEXT NOT NULL,
			first_seen_at TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (source, source_id, item_id)
		)
	`)
	return err
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

func (a *App) recordFavoriteUpdateSuccess(source favoriteUpdateSource, scanStartedAt time.Time) error {
	value := scanStartedAt.UTC().Format(time.RFC3339)

	_, err := a.db.Exec(`
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
	attemptedAt := scanStartedAt.UTC().Format(time.RFC3339)

	_, err := a.db.Exec(`
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

func (a *App) recordFavoriteUpdateSeenItem(
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
	err := a.db.QueryRow(`
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

	_, err = a.db.Exec(`
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
	postID = strings.TrimSpace(postID)
	if postID == "" {
		return ""
	}

	if strings.Contains(postID, "/") {
		return "https://t.me/" + strings.TrimPrefix(postID, "/")
	}

	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if username == "" {
		return ""
	}

	return fmt.Sprintf("https://t.me/%s/%s", username, postID)
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
