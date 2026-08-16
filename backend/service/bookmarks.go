package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	maximumBookmarkURLBytes          = 4096
	maximumBookmarkTitleLength       = 200
	maximumBookmarkDescriptionBytes = 16 * 1024
	maximumBookmarkSearchLength      = 256
	maximumBookmarkTags              = 32
	maximumBookmarkTagLength         = 64
	defaultBookmarkListLimit         = 50
	maximumBookmarkListLimit         = 200
	bookmarksChangedEvent            = "bookmarks:changed"
)

type Bookmark struct {
	ID          int      `json:"id"`
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Read        bool     `json:"read"`
	ReadAt      string   `json:"read_at"`
	Tags        []string `json:"tags"`
	Revision    int      `json:"revision"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type BookmarkFilter struct {
	Query  string   `json:"query"`
	Status string   `json:"status"`
	Tags   []string `json:"tags"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
}

type BookmarkListResult struct {
	Bookmarks []Bookmark `json:"bookmarks"`
	Total     int        `json:"total"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
}

type BookmarkCreateRequest struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type BookmarkUpdateRequest struct {
	ID               int      `json:"id"`
	URL              string   `json:"url"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Tags             []string `json:"tags"`
	ExpectedRevision int      `json:"expected_revision"`
}

type BookmarkReadRequest struct {
	ID               int  `json:"id"`
	Read             bool `json:"read"`
	ExpectedRevision int  `json:"expected_revision"`
}

type bookmarkQueryStore interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (a *Service) ListBookmarks(filter BookmarkFilter) (BookmarkListResult, error) {
	return a.ListBookmarksContext(a.requestContext(), filter)
}

func (a *Service) ListBookmarksContext(ctx context.Context, filter BookmarkFilter) (BookmarkListResult, error) {
	normalized, err := normalizeBookmarkFilter(filter)
	if err != nil {
		return BookmarkListResult{}, err
	}
	where, args := bookmarkFilterSQL(normalized)

	var total int
	if err := a.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM bookmarks AS b WHERE "+where, args...).Scan(&total); err != nil {
		return BookmarkListResult{}, fmt.Errorf("count bookmarks: %w", err)
	}

	queryArgs := append([]any(nil), args...)
	queryArgs = append(queryArgs, normalized.Limit, normalized.Offset)
	rows, err := a.db.QueryContext(ctx, `
		SELECT b.id, b.url, b.title, b.description, b.is_read, b.read_at,
		       b.revision, b.created_at, b.updated_at
		FROM bookmarks AS b
		WHERE `+where+`
		ORDER BY b.is_read ASC, b.created_at DESC, b.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return BookmarkListResult{}, fmt.Errorf("list bookmarks: %w", err)
	}
	bookmarks, err := scanBookmarks(rows)
	if err != nil {
		return BookmarkListResult{}, err
	}
	bookmarks, err = hydrateBookmarkTags(ctx, a.db, bookmarks)
	if err != nil {
		return BookmarkListResult{}, err
	}
	return BookmarkListResult{
		Bookmarks: bookmarks,
		Total: total,
		Limit: normalized.Limit,
		Offset: normalized.Offset,
	}, nil
}

func bookmarkFilterSQL(filter BookmarkFilter) (string, []any) {
	where := []string{"1 = 1"}
	args := []any{}
	switch filter.Status {
	case "read":
		where = append(where, "b.is_read = 1")
	case "unread":
		where = append(where, "b.is_read = 0")
	}
	if filter.Query != "" {
		pattern := collectionLikePattern(filter.Query)
		where = append(where, `(
			LOWER(b.title) LIKE ? ESCAPE '\'
			OR LOWER(b.url) LIKE ? ESCAPE '\'
			OR LOWER(b.description) LIKE ? ESCAPE '\'
			OR EXISTS (
				SELECT 1
				FROM bookmark_tag_assignments AS search_bta
				JOIN bookmark_tags AS search_bt ON search_bt.id = search_bta.tag_id
				WHERE search_bta.bookmark_id = b.id
				  AND search_bt.name_normalized LIKE ? ESCAPE '\'
			)
		)`)
		args = append(args, pattern, pattern, pattern, pattern)
	}
	for _, tag := range filter.Tags {
		where = append(where, `EXISTS (
			SELECT 1
			FROM bookmark_tag_assignments AS filter_bta
			JOIN bookmark_tags AS filter_bt ON filter_bt.id = filter_bta.tag_id
			WHERE filter_bta.bookmark_id = b.id
			  AND filter_bt.name_normalized = ?
		)`)
		args = append(args, strings.ToLower(tag))
	}
	return strings.Join(where, " AND "), args
}

func (a *Service) GetBookmark(id int) (Bookmark, error) {
	return a.GetBookmarkContext(a.requestContext(), id)
}

func (a *Service) GetBookmarkContext(ctx context.Context, id int) (Bookmark, error) {
	if err := validateBookmarkID(id); err != nil {
		return Bookmark{}, err
	}
	return loadBookmarkContext(ctx, a.db, id)
}

func loadBookmarkContext(ctx context.Context, store bookmarkQueryStore, id int) (Bookmark, error) {
	var bookmark Bookmark
	var read int
	var readAt sql.NullString
	err := store.QueryRowContext(ctx, `
		SELECT id, url, title, description, is_read, read_at,
		       revision, created_at, updated_at
		FROM bookmarks
		WHERE id = ?
	`, id).Scan(
		&bookmark.ID, &bookmark.URL, &bookmark.Title, &bookmark.Description,
		&read, &readAt, &bookmark.Revision, &bookmark.CreatedAt, &bookmark.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Bookmark{}, &NotFoundError{Resource: "bookmark", Key: fmt.Sprint(id)}
	}
	if err != nil {
		return Bookmark{}, fmt.Errorf("load bookmark: %w", err)
	}
	bookmark.Read = read != 0
	bookmark.ReadAt = readAt.String
	items, err := hydrateBookmarkTags(ctx, store, []Bookmark{bookmark})
	if err != nil {
		return Bookmark{}, err
	}
	return items[0], nil
}

func scanBookmarks(rows *sql.Rows) ([]Bookmark, error) {
	defer rows.Close()
	items := make([]Bookmark, 0)
	for rows.Next() {
		var bookmark Bookmark
		var read int
		var readAt sql.NullString
		if err := rows.Scan(
			&bookmark.ID, &bookmark.URL, &bookmark.Title, &bookmark.Description,
			&read, &readAt, &bookmark.Revision, &bookmark.CreatedAt, &bookmark.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan bookmark: %w", err)
		}
		bookmark.Read = read != 0
		bookmark.ReadAt = readAt.String
		bookmark.Tags = []string{}
		items = append(items, bookmark)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bookmarks: %w", err)
	}
	return items, nil
}

func hydrateBookmarkTags(ctx context.Context, store bookmarkQueryStore, bookmarks []Bookmark) ([]Bookmark, error) {
	if len(bookmarks) == 0 {
		return bookmarks, nil
	}
	placeholders := make([]string, 0, len(bookmarks))
	args := make([]any, 0, len(bookmarks))
	positions := make(map[int]int, len(bookmarks))
	for index := range bookmarks {
		bookmarks[index].Tags = []string{}
		positions[bookmarks[index].ID] = index
		placeholders = append(placeholders, "?")
		args = append(args, bookmarks[index].ID)
	}
	rows, err := store.QueryContext(ctx, `
		SELECT bta.bookmark_id, bt.name
		FROM bookmark_tag_assignments AS bta
		JOIN bookmark_tags AS bt ON bt.id = bta.tag_id
		WHERE bta.bookmark_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY bt.name_normalized ASC
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("load bookmark tags: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var bookmarkID int
		var name string
		if err := rows.Scan(&bookmarkID, &name); err != nil {
			return nil, fmt.Errorf("scan bookmark tag: %w", err)
		}
		if index, exists := positions[bookmarkID]; exists {
			bookmarks[index].Tags = append(bookmarks[index].Tags, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bookmark tags: %w", err)
	}
	return bookmarks, nil
}

func (a *Service) CreateBookmark(request BookmarkCreateRequest) (Bookmark, error) {
	return a.CreateBookmarkContext(a.requestContext(), request)
}

func (a *Service) CreateBookmarkContext(ctx context.Context, request BookmarkCreateRequest) (Bookmark, error) {
	displayURL, normalizedURL, title, description, tags, err := normalizeBookmarkWrite(
		request.URL, request.Title, request.Description, request.Tags,
	)
	if err != nil {
		return Bookmark{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return Bookmark{}, fmt.Errorf("begin create bookmark: %w", err)
	}
	defer tx.Rollback()
	if err := ensureBookmarkURLAvailable(ctx, tx, normalizedURL, 0); err != nil {
		return Bookmark{}, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO bookmarks (url, url_normalized, title, description)
		VALUES (?, ?, ?, ?)
	`, displayURL, normalizedURL, title, description)
	if err != nil {
		return Bookmark{}, fmt.Errorf("create bookmark: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Bookmark{}, fmt.Errorf("read created bookmark ID: %w", err)
	}
	if err := replaceBookmarkTagsContext(ctx, tx, int(id), tags); err != nil {
		return Bookmark{}, err
	}
	bookmark, err := loadBookmarkContext(ctx, tx, int(id))
	if err != nil {
		return Bookmark{}, err
	}
	if err := tx.Commit(); err != nil {
		return Bookmark{}, fmt.Errorf("commit create bookmark: %w", err)
	}
	return bookmark, nil
}

func (a *Service) UpdateBookmark(request BookmarkUpdateRequest) (Bookmark, error) {
	return a.UpdateBookmarkContext(a.requestContext(), request)
}

func (a *Service) UpdateBookmarkContext(ctx context.Context, request BookmarkUpdateRequest) (Bookmark, error) {
	if err := validateBookmarkMutation(request.ID, request.ExpectedRevision); err != nil {
		return Bookmark{}, err
	}
	displayURL, normalizedURL, title, description, tags, err := normalizeBookmarkWrite(
		request.URL, request.Title, request.Description, request.Tags,
	)
	if err != nil {
		return Bookmark{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return Bookmark{}, fmt.Errorf("begin update bookmark: %w", err)
	}
	defer tx.Rollback()
	if err := ensureBookmarkURLAvailable(ctx, tx, normalizedURL, request.ID); err != nil {
		return Bookmark{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE bookmarks
		SET url = ?, url_normalized = ?, title = ?, description = ?,
		    revision = revision + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND revision = ?
	`, displayURL, normalizedURL, title, description, request.ID, request.ExpectedRevision)
	if err != nil {
		return Bookmark{}, fmt.Errorf("update bookmark: %w", err)
	}
	if err := requireBookmarkRevisionMutation(ctx, tx, result, request.ID); err != nil {
		return Bookmark{}, err
	}
	if err := replaceBookmarkTagsContext(ctx, tx, request.ID, tags); err != nil {
		return Bookmark{}, err
	}
	bookmark, err := loadBookmarkContext(ctx, tx, request.ID)
	if err != nil {
		return Bookmark{}, err
	}
	if err := tx.Commit(); err != nil {
		return Bookmark{}, fmt.Errorf("commit update bookmark: %w", err)
	}
	return bookmark, nil
}

func (a *Service) SetBookmarkRead(request BookmarkReadRequest) (Bookmark, error) {
	return a.SetBookmarkReadContext(a.requestContext(), request)
}

func (a *Service) SetBookmarkReadContext(ctx context.Context, request BookmarkReadRequest) (Bookmark, error) {
	if err := validateBookmarkMutation(request.ID, request.ExpectedRevision); err != nil {
		return Bookmark{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return Bookmark{}, fmt.Errorf("begin set bookmark read state: %w", err)
	}
	defer tx.Rollback()
	current, err := loadBookmarkContext(ctx, tx, request.ID)
	if err != nil {
		return Bookmark{}, err
	}
	if current.Read == request.Read {
		if err := tx.Commit(); err != nil {
			return Bookmark{}, fmt.Errorf("commit bookmark read state: %w", err)
		}
		return current, nil
	}
	if current.Revision != request.ExpectedRevision {
		return Bookmark{}, &ConflictError{Resource: "bookmark", Message: "The bookmark changed after it was loaded."}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE bookmarks
		SET is_read = ?,
		    read_at = CASE WHEN ? = 1 THEN CURRENT_TIMESTAMP ELSE NULL END,
		    revision = revision + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND revision = ?
	`, boolDatabaseValue(request.Read), boolDatabaseValue(request.Read), request.ID, request.ExpectedRevision)
	if err != nil {
		return Bookmark{}, fmt.Errorf("set bookmark read state: %w", err)
	}
	if err := requireBookmarkRevisionMutation(ctx, tx, result, request.ID); err != nil {
		return Bookmark{}, err
	}
	bookmark, err := loadBookmarkContext(ctx, tx, request.ID)
	if err != nil {
		return Bookmark{}, err
	}
	if err := tx.Commit(); err != nil {
		return Bookmark{}, fmt.Errorf("commit bookmark read state: %w", err)
	}
	return bookmark, nil
}

func (a *Service) DeleteBookmark(id int) error {
	return a.DeleteBookmarkContext(a.requestContext(), id)
}

func (a *Service) DeleteBookmarkContext(ctx context.Context, id int) error {
	if err := validateBookmarkID(id); err != nil {
		return err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete bookmark: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM bookmarks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete bookmark: %w", err)
	}
	if err := requireSingleMutation(result, "delete bookmark", "bookmark", false); err != nil {
		return err
	}
	if err := removeUnusedBookmarkTagsContext(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete bookmark: %w", err)
	}
	return nil
}

func (a *Service) ListBookmarkTags() ([]string, error) {
	return a.ListBookmarkTagsContext(a.requestContext())
}

func (a *Service) ListBookmarkTagsContext(ctx context.Context) ([]string, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT name FROM bookmark_tags ORDER BY name_normalized ASC`)
	if err != nil {
		return nil, fmt.Errorf("list bookmark tags: %w", err)
	}
	defer rows.Close()
	tags := make([]string, 0)
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan bookmark tag: %w", err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bookmark tags: %w", err)
	}
	return tags, nil
}

func replaceBookmarkTagsContext(ctx context.Context, tx *sql.Tx, bookmarkID int, tags []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM bookmark_tag_assignments WHERE bookmark_id = ?`, bookmarkID); err != nil {
		return fmt.Errorf("clear bookmark tags: %w", err)
	}
	for _, tag := range tags {
		normalized := strings.ToLower(tag)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO bookmark_tags (name, name_normalized)
			VALUES (?, ?)
			ON CONFLICT(name_normalized) DO NOTHING
		`, tag, normalized); err != nil {
			return fmt.Errorf("store bookmark tag: %w", err)
		}
		var tagID int
		if err := tx.QueryRowContext(ctx, `SELECT id FROM bookmark_tags WHERE name_normalized = ?`, normalized).Scan(&tagID); err != nil {
			return fmt.Errorf("load bookmark tag ID: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO bookmark_tag_assignments (bookmark_id, tag_id)
			VALUES (?, ?)
		`, bookmarkID, tagID); err != nil {
			return fmt.Errorf("assign bookmark tag: %w", err)
		}
	}
	return removeUnusedBookmarkTagsContext(ctx, tx)
}

func removeUnusedBookmarkTagsContext(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM bookmark_tags
		WHERE NOT EXISTS (
			SELECT 1 FROM bookmark_tag_assignments
			WHERE bookmark_tag_assignments.tag_id = bookmark_tags.id
		)
	`); err != nil {
		return fmt.Errorf("remove unused bookmark tags: %w", err)
	}
	return nil
}

func ensureBookmarkURLAvailable(ctx context.Context, tx *sql.Tx, normalizedURL string, excludedID int) error {
	query := `SELECT id FROM bookmarks WHERE url_normalized = ?`
	args := []any{normalizedURL}
	if excludedID > 0 {
		query += ` AND id <> ?`
		args = append(args, excludedID)
	}
	var existingID int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check bookmark URL: %w", err)
	}
	return &ConflictError{Resource: "bookmark", Message: "This URL is already bookmarked."}
}

func requireBookmarkRevisionMutation(ctx context.Context, tx *sql.Tx, result sql.Result, id int) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check bookmark mutation: %w", err)
	}
	if affected == 1 {
		return nil
	}
	if affected != 0 {
		return fmt.Errorf("bookmark mutation affected %d rows", affected)
	}
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM bookmarks WHERE id = ?`, id).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &NotFoundError{Resource: "bookmark", Key: fmt.Sprint(id)}
		}
		return fmt.Errorf("check bookmark existence: %w", err)
	}
	return &ConflictError{Resource: "bookmark", Message: "The bookmark changed after it was loaded."}
}

func normalizeBookmarkWrite(urlValue, titleValue, descriptionValue string, tagValues []string) (string, string, string, string, []string, error) {
	displayURL, normalizedURL, err := NormalizeBookmarkURL(urlValue)
	if err != nil {
		return "", "", "", "", nil, err
	}
	title := strings.TrimSpace(titleValue)
	if title == "" {
		return "", "", "", "", nil, &ValidationError{Field: "title", Message: "bookmark title cannot be empty"}
	}
	if utf8.RuneCountInString(title) > maximumBookmarkTitleLength {
		return "", "", "", "", nil, &ValidationError{Field: "title", Message: "bookmark title must be 200 characters or fewer"}
	}
	description := strings.TrimSpace(descriptionValue)
	if len(description) > maximumBookmarkDescriptionBytes {
		return "", "", "", "", nil, &ValidationError{Field: "description", Message: "bookmark description must be 16 KiB or smaller"}
	}
	tags, err := normalizeBookmarkTags(tagValues)
	if err != nil {
		return "", "", "", "", nil, err
	}
	return displayURL, normalizedURL, title, description, tags, nil
}

func NormalizeBookmarkURL(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", &ValidationError{Field: "url", Message: "bookmark URL cannot be empty"}
	}
	if len(value) > maximumBookmarkURLBytes {
		return "", "", &ValidationError{Field: "url", Message: "bookmark URL must be 4096 bytes or fewer"}
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return "", "", &ValidationError{Field: "url", Message: "bookmark URL cannot contain control characters"}
		}
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", "", &ValidationError{Field: "url", Message: "bookmark URL is invalid"}
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", &ValidationError{Field: "url", Message: "bookmark URL must use HTTP or HTTPS"}
	}
	if parsed.User != nil {
		return "", "", &ValidationError{Field: "url", Message: "bookmark URL cannot contain credentials"}
	}
	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" {
		return "", "", &ValidationError{Field: "url", Message: "bookmark URL must include a hostname"}
	}
	port := parsed.Port()
	if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		parsed.Host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		parsed.Host = "[" + hostname + "]"
	} else {
		parsed.Host = hostname
	}
	display := parsed.String()
	if len(display) > maximumBookmarkURLBytes {
		return "", "", &ValidationError{Field: "url", Message: "bookmark URL must be 4096 bytes or fewer"}
	}
	return display, display, nil
}

func normalizeBookmarkTags(values []string) ([]string, error) {
	if len(values) > maximumBookmarkTags {
		return nil, &ValidationError{Field: "tags", Message: "a bookmark can have at most 32 tags"}
	}
	tags := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		tag := strings.TrimSpace(value)
		if tag == "" {
			return nil, &ValidationError{Field: "tags", Message: "bookmark tags cannot be empty"}
		}
		if utf8.RuneCountInString(tag) > maximumBookmarkTagLength {
			return nil, &ValidationError{Field: "tags", Message: "bookmark tags must be 64 characters or fewer"}
		}
		normalized := strings.ToLower(tag)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		tags = append(tags, tag)
	}
	return tags, nil
}

func normalizeBookmarkFilter(filter BookmarkFilter) (BookmarkFilter, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	if utf8.RuneCountInString(filter.Query) > maximumBookmarkSearchLength {
		return BookmarkFilter{}, &ValidationError{Field: "query", Message: "bookmark search must be 256 characters or fewer"}
	}
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	if filter.Status == "" {
		filter.Status = "all"
	}
	switch filter.Status {
	case "all", "read", "unread":
	default:
		return BookmarkFilter{}, &ValidationError{Field: "status", Message: "status must be all, read, or unread"}
	}
	tags, err := normalizeBookmarkTags(filter.Tags)
	if err != nil {
		return BookmarkFilter{}, err
	}
	filter.Tags = tags
	limit, offset, err := normalizeCollectionPage(filter.Limit, filter.Offset, defaultBookmarkListLimit, maximumBookmarkListLimit)
	if err != nil {
		return BookmarkFilter{}, err
	}
	filter.Limit = limit
	filter.Offset = offset
	return filter, nil
}

func validateBookmarkID(id int) error {
	if id <= 0 {
		return &ValidationError{Field: "id", Message: "bookmark ID must be a positive integer"}
	}
	return nil
}

func validateBookmarkMutation(id int, revision int) error {
	if err := validateBookmarkID(id); err != nil {
		return err
	}
	if revision <= 0 {
		return &ValidationError{Field: "expected_revision", Message: "expected revision must be a positive integer"}
	}
	return nil
}

func (a *Service) EmitBookmarksChanged() {
	ctx := a.requestContext()
	if ctx.Value("events") != nil {
		runtime.EventsEmit(ctx, bookmarksChangedEvent)
	}
}
