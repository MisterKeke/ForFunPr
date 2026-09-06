package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	defaultClipboardLimit = 100
	maximumClipboardLimit = 500
	clipboardChangedEvent = "clipboard:changed"
)

type ClipboardItem struct {
	ID            int    `json:"id"`
	Kind          string `json:"kind"`
	Content       string `json:"content"`
	ByteSize      int    `json:"byte_size"`
	FirstCopiedAt string `json:"first_copied_at"`
	LastCopiedAt  string `json:"last_copied_at"`
	CopyCount     int    `json:"copy_count"`
	Pinned        bool   `json:"pinned"`
}

type ClipboardSettings struct {
	CollectionEnabled bool `json:"collection_enabled"`
	RetentionDays     int  `json:"retention_days"`
	MaximumItems      int  `json:"maximum_items"`
	MaximumTextBytes  int  `json:"maximum_text_bytes"`
}

type ClipboardListFilter struct {
	Query      string `json:"query"`
	Kind       string `json:"kind"`
	PinnedOnly bool   `json:"pinned_only"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
}

type ClipboardListResult struct {
	Items  []ClipboardItem `json:"items"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

func (a *Service) GetClipboardSettingsContext(ctx context.Context) (ClipboardSettings, error) {
	var settings ClipboardSettings
	var enabled int
	err := a.db.QueryRowContext(ctx, `
		SELECT collection_enabled, retention_days, maximum_items, maximum_text_bytes
		FROM clipboard_settings WHERE id = 1
	`).Scan(&enabled, &settings.RetentionDays, &settings.MaximumItems, &settings.MaximumTextBytes)
	if err != nil {
		return ClipboardSettings{}, fmt.Errorf("read clipboard settings: %w", err)
	}
	settings.CollectionEnabled = enabled != 0
	return settings, nil
}

func (a *Service) UpdateClipboardSettingsContext(ctx context.Context, settings ClipboardSettings) (ClipboardSettings, error) {
	if settings.RetentionDays < 1 || settings.RetentionDays > 3650 {
		return ClipboardSettings{}, &ValidationError{Field: "retention_days", Message: "retention must be between 1 and 3650 days"}
	}
	if settings.MaximumItems < 10 || settings.MaximumItems > 10000 {
		return ClipboardSettings{}, &ValidationError{Field: "maximum_items", Message: "maximum items must be between 10 and 10000"}
	}
	if settings.MaximumTextBytes < 1024 || settings.MaximumTextBytes > 1024*1024 {
		return ClipboardSettings{}, &ValidationError{Field: "maximum_text_bytes", Message: "maximum text size must be between 1 KiB and 1 MiB"}
	}
	_, err := a.db.ExecContext(ctx, `
		UPDATE clipboard_settings
		SET collection_enabled = ?, retention_days = ?, maximum_items = ?, maximum_text_bytes = ?
		WHERE id = 1
	`, boolDatabaseValue(settings.CollectionEnabled), settings.RetentionDays, settings.MaximumItems, settings.MaximumTextBytes)
	if err != nil {
		return ClipboardSettings{}, fmt.Errorf("update clipboard settings: %w", err)
	}
	if err := a.PruneClipboardHistoryContext(ctx, settings); err != nil {
		return ClipboardSettings{}, err
	}
	a.emitClipboardChanged()
	return settings, nil
}

func (a *Service) RecordClipboardText(value string) error {
	ctx, done, err := a.BeginOperation(a.OperationContext())
	if err != nil {
		return err
	}
	defer done()
	return a.RecordClipboardTextContext(ctx, value)
}

func (a *Service) RecordClipboardTextContext(ctx context.Context, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	settings, err := a.GetClipboardSettingsContext(ctx)
	if err != nil {
		return err
	}
	if !settings.CollectionEnabled || len([]byte(value)) > settings.MaximumTextBytes {
		return nil
	}

	digest := sha256.Sum256([]byte(value))
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = a.db.ExecContext(ctx, `
		INSERT INTO clipboard_items (
			kind, content, content_hash, byte_size, first_copied_at, last_copied_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(content_hash) DO UPDATE SET
			kind = excluded.kind,
			content = excluded.content,
			byte_size = excluded.byte_size,
			last_copied_at = excluded.last_copied_at,
			copy_count = clipboard_items.copy_count + 1
	`, clipboardKind(value), value, hex.EncodeToString(digest[:]), len([]byte(value)), now, now)
	if err != nil {
		return fmt.Errorf("record clipboard item: %w", err)
	}
	if err := a.PruneClipboardHistoryContext(ctx, settings); err != nil {
		return err
	}
	a.emitClipboardChanged()
	return nil
}

func (a *Service) ListClipboardItemsContext(ctx context.Context, filter ClipboardListFilter) (ClipboardListResult, error) {
	limit, offset, err := normalizeCollectionPage(filter.Limit, filter.Offset, defaultClipboardLimit, maximumClipboardLimit)
	if err != nil {
		return ClipboardListResult{}, err
	}
	filter.Query = strings.TrimSpace(filter.Query)
	if len([]rune(filter.Query)) > 256 {
		return ClipboardListResult{}, &ValidationError{Field: "query", Message: "clipboard search cannot exceed 256 characters"}
	}
	if filter.Kind != "" && filter.Kind != "text" && filter.Kind != "url" {
		return ClipboardListResult{}, &ValidationError{Field: "kind", Message: "clipboard kind must be text or url"}
	}

	where := []string{"1 = 1"}
	args := []any{}
	if filter.Query != "" {
		where = append(where, "LOWER(content) LIKE ? ESCAPE '\\'")
		args = append(args, collectionLikePattern(filter.Query))
	}
	if filter.Kind != "" {
		where = append(where, "kind = ?")
		args = append(args, filter.Kind)
	}
	if filter.PinnedOnly {
		where = append(where, "is_pinned = 1")
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := a.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM clipboard_items WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return ClipboardListResult{}, fmt.Errorf("count clipboard items: %w", err)
	}
	queryArgs := append([]any(nil), args...)
	queryArgs = append(queryArgs, limit, offset)
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, kind, content, byte_size, first_copied_at, last_copied_at, copy_count, is_pinned
		FROM clipboard_items WHERE `+whereSQL+`
		ORDER BY is_pinned DESC, last_copied_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return ClipboardListResult{}, fmt.Errorf("list clipboard items: %w", err)
	}
	defer rows.Close()
	items := make([]ClipboardItem, 0)
	for rows.Next() {
		var item ClipboardItem
		var pinned int
		if err := rows.Scan(&item.ID, &item.Kind, &item.Content, &item.ByteSize, &item.FirstCopiedAt, &item.LastCopiedAt, &item.CopyCount, &pinned); err != nil {
			return ClipboardListResult{}, fmt.Errorf("scan clipboard item: %w", err)
		}
		item.Pinned = pinned != 0
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ClipboardListResult{}, fmt.Errorf("iterate clipboard items: %w", err)
	}
	return ClipboardListResult{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (a *Service) GetClipboardItemContext(ctx context.Context, id int) (ClipboardItem, error) {
	if id <= 0 {
		return ClipboardItem{}, &ValidationError{Field: "id", Message: "clipboard item ID must be positive"}
	}
	var item ClipboardItem
	var pinned int
	err := a.db.QueryRowContext(ctx, `
		SELECT id, kind, content, byte_size, first_copied_at, last_copied_at, copy_count, is_pinned
		FROM clipboard_items WHERE id = ?
	`, id).Scan(&item.ID, &item.Kind, &item.Content, &item.ByteSize, &item.FirstCopiedAt, &item.LastCopiedAt, &item.CopyCount, &pinned)
	if err == sql.ErrNoRows {
		return ClipboardItem{}, &NotFoundError{Resource: "clipboard item", Key: fmt.Sprint(id)}
	}
	if err != nil {
		return ClipboardItem{}, fmt.Errorf("read clipboard item: %w", err)
	}
	item.Pinned = pinned != 0
	return item, nil
}

func (a *Service) SetClipboardItemPinnedContext(ctx context.Context, id int, pinned bool) error {
	if id <= 0 {
		return &ValidationError{Field: "id", Message: "clipboard item ID must be positive"}
	}
	result, err := a.db.ExecContext(ctx, `UPDATE clipboard_items SET is_pinned = ? WHERE id = ?`, boolDatabaseValue(pinned), id)
	if err != nil {
		return fmt.Errorf("update clipboard item: %w", err)
	}
	if err := requireSingleMutation(result, "update clipboard item", "clipboard item", false); err != nil {
		return err
	}
	a.emitClipboardChanged()
	return nil
}

func (a *Service) DeleteClipboardItemContext(ctx context.Context, id int) error {
	if id <= 0 {
		return &ValidationError{Field: "id", Message: "clipboard item ID must be positive"}
	}
	result, err := a.db.ExecContext(ctx, `DELETE FROM clipboard_items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete clipboard item: %w", err)
	}
	if err := requireSingleMutation(result, "delete clipboard item", "clipboard item", false); err != nil {
		return err
	}
	a.emitClipboardChanged()
	return nil
}

func (a *Service) ClearClipboardHistoryContext(ctx context.Context, keepPinned bool) error {
	query := `DELETE FROM clipboard_items`
	if keepPinned {
		query += ` WHERE is_pinned = 0`
	}
	if _, err := a.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("clear clipboard history: %w", err)
	}
	a.emitClipboardChanged()
	return nil
}

func (a *Service) PruneClipboardHistoryContext(ctx context.Context, settings ClipboardSettings) error {
	threshold := time.Now().UTC().AddDate(0, 0, -settings.RetentionDays).Format(time.RFC3339Nano)
	if _, err := a.db.ExecContext(ctx, `DELETE FROM clipboard_items WHERE is_pinned = 0 AND last_copied_at < ?`, threshold); err != nil {
		return fmt.Errorf("prune expired clipboard history: %w", err)
	}
	if _, err := a.db.ExecContext(ctx, `
		DELETE FROM clipboard_items WHERE id IN (
			SELECT id FROM clipboard_items WHERE is_pinned = 0
			ORDER BY last_copied_at DESC, id DESC LIMIT -1 OFFSET ?
		)
	`, settings.MaximumItems); err != nil {
		return fmt.Errorf("prune excess clipboard history: %w", err)
	}
	return nil
}

func clipboardKind(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
		return "url"
	}
	return "text"
}

func (a *Service) emitClipboardChanged() {
	ctx := a.requestContext()
	if ctx.Value("events") != nil {
		runtime.EventsEmit(ctx, clipboardChangedEvent)
	}
}
