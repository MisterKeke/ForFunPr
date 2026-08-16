package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	maximumNoteTitleLength  = 200
	maximumNoteBodyBytes    = 256 * 1024
	maximumNoteSearchLength = 256
	defaultNoteListLimit    = 50
	maximumNoteListLimit    = 200
	notesChangedEvent       = "notes:changed"
)

type Note struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Pinned    bool   `json:"pinned"`
	Archived  bool   `json:"archived"`
	Revision  int    `json:"revision"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type NoteSummary struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Preview   string `json:"preview"`
	Pinned    bool   `json:"pinned"`
	Archived  bool   `json:"archived"`
	Revision  int    `json:"revision"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type NoteListFilter struct {
	Query         string `json:"query"`
	ArchiveStatus string `json:"archive_status"`
	Pinned        *bool  `json:"pinned,omitempty"`
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
}

type NoteListResult struct {
	Notes  []NoteSummary `json:"notes"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

type NoteCreateRequest struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Pinned bool   `json:"pinned"`
}

type NoteUpdateRequest struct {
	ID               int    `json:"id"`
	Title            string `json:"title"`
	Body             string `json:"body"`
	ExpectedRevision int    `json:"expected_revision"`
}

type NoteStateRequest struct {
	ID               int  `json:"id"`
	Value            bool `json:"value"`
	ExpectedRevision int  `json:"expected_revision"`
}

type noteQueryStore interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (a *Service) ListNotes(filter NoteListFilter) (NoteListResult, error) {
	return a.ListNotesContext(a.requestContext(), filter)
}

func (a *Service) ListNotesContext(ctx context.Context, filter NoteListFilter) (NoteListResult, error) {
	normalized, err := normalizeNoteFilter(filter)
	if err != nil {
		return NoteListResult{}, err
	}

	where := []string{"1 = 1"}
	args := []any{}
	switch normalized.ArchiveStatus {
	case "active":
		where = append(where, "is_archived = 0")
	case "archived":
		where = append(where, "is_archived = 1")
	}
	if normalized.Pinned != nil {
		where = append(where, "is_pinned = ?")
		args = append(args, boolDatabaseValue(*normalized.Pinned))
	}
	if normalized.Query != "" {
		where = append(where, `(LOWER(title) LIKE ? ESCAPE '\' OR LOWER(body) LIKE ? ESCAPE '\')`)
		pattern := collectionLikePattern(normalized.Query)
		args = append(args, pattern, pattern)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := a.db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM notes WHERE "+whereSQL,
		args...,
	).Scan(&total); err != nil {
		return NoteListResult{}, fmt.Errorf("count notes: %w", err)
	}

	queryArgs := append([]any(nil), args...)
	queryArgs = append(queryArgs, normalized.Limit, normalized.Offset)
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, title, substr(body, 1, 240), is_pinned, is_archived,
		       revision, created_at, updated_at
		FROM notes
		WHERE `+whereSQL+`
		ORDER BY is_pinned DESC, updated_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return NoteListResult{}, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	notes := make([]NoteSummary, 0)
	for rows.Next() {
		var note NoteSummary
		var preview string
		var pinned int
		var archived int
		if err := rows.Scan(
			&note.ID, &note.Title, &preview, &pinned, &archived,
			&note.Revision, &note.CreatedAt, &note.UpdatedAt,
		); err != nil {
			return NoteListResult{}, fmt.Errorf("scan note summary: %w", err)
		}
		note.Preview = notePreview(preview)
		note.Pinned = pinned != 0
		note.Archived = archived != 0
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return NoteListResult{}, fmt.Errorf("iterate notes: %w", err)
	}

	return NoteListResult{
		Notes: notes, Total: total, Limit: normalized.Limit, Offset: normalized.Offset,
	}, nil
}

func (a *Service) GetNote(id int) (Note, error) {
	return a.GetNoteContext(a.requestContext(), id)
}

func (a *Service) GetNoteContext(ctx context.Context, id int) (Note, error) {
	if err := validateNoteID(id); err != nil {
		return Note{}, err
	}
	return loadNoteContext(ctx, a.db, id)
}

func loadNoteContext(ctx context.Context, store noteQueryStore, id int) (Note, error) {
	var note Note
	var pinned int
	var archived int
	err := store.QueryRowContext(ctx, `
		SELECT id, title, body, is_pinned, is_archived, revision,
		       created_at, updated_at
		FROM notes
		WHERE id = ?
	`, id).Scan(
		&note.ID, &note.Title, &note.Body, &pinned, &archived, &note.Revision,
		&note.CreatedAt, &note.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, &NotFoundError{Resource: "note", Key: fmt.Sprint(id)}
	}
	if err != nil {
		return Note{}, fmt.Errorf("load note: %w", err)
	}
	note.Pinned = pinned != 0
	note.Archived = archived != 0
	return note, nil
}

func (a *Service) CreateNote(request NoteCreateRequest) (Note, error) {
	return a.CreateNoteContext(a.requestContext(), request)
}

func (a *Service) CreateNoteContext(ctx context.Context, request NoteCreateRequest) (Note, error) {
	title, body, err := normalizeNoteWrite(request.Title, request.Body)
	if err != nil {
		return Note{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return Note{}, fmt.Errorf("begin create note: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO notes (title, body, is_pinned)
		VALUES (?, ?, ?)
	`, title, body, boolDatabaseValue(request.Pinned))
	if err != nil {
		return Note{}, fmt.Errorf("create note: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Note{}, fmt.Errorf("read created note ID: %w", err)
	}
	note, err := loadNoteContext(ctx, tx, int(id))
	if err != nil {
		return Note{}, err
	}
	if err := tx.Commit(); err != nil {
		return Note{}, fmt.Errorf("commit create note: %w", err)
	}
	return note, nil
}

func (a *Service) UpdateNote(request NoteUpdateRequest) (Note, error) {
	return a.UpdateNoteContext(a.requestContext(), request)
}

func (a *Service) UpdateNoteContext(ctx context.Context, request NoteUpdateRequest) (Note, error) {
	if err := validateNoteMutation(request.ID, request.ExpectedRevision); err != nil {
		return Note{}, err
	}
	title, body, err := normalizeNoteWrite(request.Title, request.Body)
	if err != nil {
		return Note{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return Note{}, fmt.Errorf("begin update note: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE notes
		SET title = ?, body = ?, revision = revision + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND revision = ?
	`, title, body, request.ID, request.ExpectedRevision)
	if err != nil {
		return Note{}, fmt.Errorf("update note: %w", err)
	}
	if err := requireNoteRevisionMutation(ctx, tx, result, request.ID); err != nil {
		return Note{}, err
	}
	note, err := loadNoteContext(ctx, tx, request.ID)
	if err != nil {
		return Note{}, err
	}
	if err := tx.Commit(); err != nil {
		return Note{}, fmt.Errorf("commit update note: %w", err)
	}
	return note, nil
}

func (a *Service) SetNotePinned(request NoteStateRequest) (Note, error) {
	return a.SetNotePinnedContext(a.requestContext(), request)
}

func (a *Service) SetNotePinnedContext(ctx context.Context, request NoteStateRequest) (Note, error) {
	return a.setNoteStateContext(ctx, request, "is_pinned", "pin note")
}

func (a *Service) SetNoteArchived(request NoteStateRequest) (Note, error) {
	return a.SetNoteArchivedContext(a.requestContext(), request)
}

func (a *Service) SetNoteArchivedContext(ctx context.Context, request NoteStateRequest) (Note, error) {
	return a.setNoteStateContext(ctx, request, "is_archived", "archive note")
}

func (a *Service) setNoteStateContext(
	ctx context.Context,
	request NoteStateRequest,
	column string,
	operation string,
) (Note, error) {
	if err := validateNoteMutation(request.ID, request.ExpectedRevision); err != nil {
		return Note{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return Note{}, fmt.Errorf("begin %s: %w", operation, err)
	}
	defer tx.Rollback()

	current, err := loadNoteContext(ctx, tx, request.ID)
	if err != nil {
		return Note{}, err
	}
	currentValue := current.Pinned
	if column == "is_archived" {
		currentValue = current.Archived
	}
	if currentValue == request.Value {
		if err := tx.Commit(); err != nil {
			return Note{}, fmt.Errorf("commit %s: %w", operation, err)
		}
		return current, nil
	}
	if current.Revision != request.ExpectedRevision {
		return Note{}, &ConflictError{
			Resource: "note", Message: "The note changed after it was loaded.",
		}
	}

	query := fmt.Sprintf(`
		UPDATE notes
		SET %s = ?, revision = revision + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND revision = ?
	`, column)
	result, err := tx.ExecContext(
		ctx, query, boolDatabaseValue(request.Value), request.ID, request.ExpectedRevision,
	)
	if err != nil {
		return Note{}, fmt.Errorf("%s: %w", operation, err)
	}
	if err := requireNoteRevisionMutation(ctx, tx, result, request.ID); err != nil {
		return Note{}, err
	}
	note, err := loadNoteContext(ctx, tx, request.ID)
	if err != nil {
		return Note{}, err
	}
	if err := tx.Commit(); err != nil {
		return Note{}, fmt.Errorf("commit %s: %w", operation, err)
	}
	return note, nil
}

func (a *Service) DeleteNote(id int) error {
	return a.DeleteNoteContext(a.requestContext(), id)
}

func (a *Service) DeleteNoteContext(ctx context.Context, id int) error {
	if err := validateNoteID(id); err != nil {
		return err
	}
	result, err := a.db.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	return requireSingleMutation(result, "delete note", "note", false)
}

func requireNoteRevisionMutation(
	ctx context.Context,
	tx *sql.Tx,
	result sql.Result,
	id int,
) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check note mutation: %w", err)
	}
	if affected == 1 {
		return nil
	}
	if affected != 0 {
		return fmt.Errorf("note mutation affected %d rows", affected)
	}
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM notes WHERE id = ?`, id).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &NotFoundError{Resource: "note", Key: fmt.Sprint(id)}
		}
		return fmt.Errorf("check note existence: %w", err)
	}
	return &ConflictError{
		Resource: "note", Message: "The note changed after it was loaded.",
	}
}

func normalizeNoteWrite(title string, body string) (string, string, error) {
	title = strings.TrimSpace(title)
	if utf8.RuneCountInString(title) > maximumNoteTitleLength {
		return "", "", &ValidationError{
			Field: "title", Message: "note title must be 200 characters or fewer",
		}
	}
	if len(body) > maximumNoteBodyBytes {
		return "", "", &ValidationError{
			Field: "body", Message: "note body must be 256 KiB or smaller",
		}
	}
	if title == "" && strings.TrimSpace(body) == "" {
		return "", "", &ValidationError{
			Field: "note", Message: "a note needs a title or body",
		}
	}
	return title, body, nil
}

func normalizeNoteFilter(filter NoteListFilter) (NoteListFilter, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	if utf8.RuneCountInString(filter.Query) > maximumNoteSearchLength {
		return NoteListFilter{}, &ValidationError{
			Field: "query", Message: "note search must be 256 characters or fewer",
		}
	}
	filter.ArchiveStatus = strings.ToLower(strings.TrimSpace(filter.ArchiveStatus))
	if filter.ArchiveStatus == "" {
		filter.ArchiveStatus = "active"
	}
	switch filter.ArchiveStatus {
	case "active", "archived", "all":
	default:
		return NoteListFilter{}, &ValidationError{
			Field: "archive_status", Message: "archive must be active, archived, or all",
		}
	}
	limit, offset, err := normalizeCollectionPage(
		filter.Limit, filter.Offset, defaultNoteListLimit, maximumNoteListLimit,
	)
	if err != nil {
		return NoteListFilter{}, err
	}
	filter.Limit = limit
	filter.Offset = offset
	return filter, nil
}

func validateNoteID(id int) error {
	if id <= 0 {
		return &ValidationError{Field: "id", Message: "note ID must be a positive integer"}
	}
	return nil
}

func validateNoteMutation(id int, revision int) error {
	if err := validateNoteID(id); err != nil {
		return err
	}
	if revision <= 0 {
		return &ValidationError{
			Field: "expected_revision", Message: "expected revision must be a positive integer",
		}
	}
	return nil
}

func notePreview(body string) string {
	preview := strings.Join(strings.Fields(body), " ")
	runes := []rune(preview)
	if len(runes) > 160 {
		return string(runes[:160]) + "…"
	}
	return preview
}

// EmitNotesChanged lets desktop views reload mutations made through REST,
// including CLI and MCP calls. Direct Wails mutations already receive the
// canonical resource and update their own local state.
func (a *Service) EmitNotesChanged() {
	ctx := a.requestContext()
	if ctx.Value("events") != nil {
		runtime.EventsEmit(ctx, notesChangedEvent)
	}
}
