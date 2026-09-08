package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"something/backend/storage"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	screenshotRoutePrefix  = "/screenshots/"
	maximumScreenshotBytes = 50 * 1024 * 1024
	defaultScreenshotLimit = 60
	maximumScreenshotLimit = 200
)

var screenshotFilenamePattern = regexp.MustCompile(`^[a-f0-9-]{36}(?:-[a-f0-9-]{36})?-(?:original|edited|thumb)\.png$`)

type Screenshot struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	CaptureKind       string `json:"capture_kind"`
	Width             int    `json:"width"`
	Height            int    `json:"height"`
	ByteSize          int64  `json:"byte_size"`
	OCRText           string `json:"ocr_text"`
	OCRLanguage       string `json:"ocr_language"`
	OCRStatus         string `json:"ocr_status"`
	OCRStartedAt      string `json:"ocr_started_at"`
	OCRCompletedAt    string `json:"ocr_completed_at"`
	OCRFailureCode    string `json:"ocr_failure_code"`
	OCRFailureMessage string `json:"ocr_failure_message"`
	ImageURL          string `json:"image_url"`
	ThumbnailURL      string `json:"thumbnail_url"`
	HasEdit           bool   `json:"has_edit"`
	CapturedAt        string `json:"captured_at"`
	UpdatedAt         string `json:"updated_at"`
	OriginalFilename  string `json:"-"`
	EditedFilename    string `json:"-"`
	ThumbnailFilename string `json:"-"`
}

type ScreenshotListFilter struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

type ScreenshotListResult struct {
	Screenshots []Screenshot `json:"screenshots"`
	Total       int          `json:"total"`
	Limit       int          `json:"limit"`
	Offset      int          `json:"offset"`
}

type ScreenshotEditRequest struct {
	ID      string `json:"id"`
	DataURL string `json:"data_url"`
}

type ScreenshotOCRQueueResult struct {
	Screenshot Screenshot `json:"screenshot"`
	Changed    bool       `json:"changed"`
}

type ScreenshotOCRStatusEvent struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	StartedAt      string `json:"started_at"`
	CompletedAt    string `json:"completed_at"`
	FailureCode    string `json:"failure_code"`
	FailureMessage string `json:"failure_message"`
}

func (a *Service) StoreScreenshotContext(ctx context.Context, captured image.Image, kind string) (Screenshot, error) {
	if captured == nil {
		return Screenshot{}, &ValidationError{Field: "image", Message: "captured image is empty"}
	}
	if kind != "screen" && kind != "window" && kind != "region" {
		return Screenshot{}, &ValidationError{Field: "capture_kind", Message: "capture kind must be screen, window, or region"}
	}
	bounds := captured.Bounds()
	if bounds.Dx() < 1 || bounds.Dy() < 1 || bounds.Dx() > 32768 || bounds.Dy() > 32768 {
		return Screenshot{}, &ValidationError{Field: "image", Message: "captured image dimensions are invalid"}
	}
	var original bytes.Buffer
	if err := png.Encode(&original, captured); err != nil {
		return Screenshot{}, fmt.Errorf("encode screenshot: %w", err)
	}
	if original.Len() < 1 || original.Len() > maximumScreenshotBytes {
		return Screenshot{}, &ValidationError{Field: "image", Message: "captured image exceeds the 50 MB limit"}
	}
	thumbnail := resizeScreenshot(captured, 420, 260)
	var thumbnailData bytes.Buffer
	if err := png.Encode(&thumbnailData, thumbnail); err != nil {
		return Screenshot{}, fmt.Errorf("encode screenshot thumbnail: %w", err)
	}

	id := uuid.NewString()
	originalFilename := id + "-original.png"
	thumbnailFilename := id + "-thumb.png"
	directory, err := storage.ScreenshotDirectory()
	if err != nil {
		return Screenshot{}, err
	}
	originalPath := filepath.Join(directory, originalFilename)
	thumbnailPath := filepath.Join(directory, thumbnailFilename)
	if err := writeScreenshotFile(originalPath, original.Bytes()); err != nil {
		return Screenshot{}, err
	}
	if err := writeScreenshotFile(thumbnailPath, thumbnailData.Bytes()); err != nil {
		_ = os.Remove(originalPath)
		return Screenshot{}, err
	}
	digest := sha256.Sum256(original.Bytes())
	_, err = a.db.ExecContext(ctx, `
		INSERT INTO screenshots (
			id, original_filename, thumbnail_filename, title, capture_kind,
			width, height, byte_size, sha256
		) VALUES (?, ?, ?, '', ?, ?, ?, ?, ?)
	`, id, originalFilename, thumbnailFilename, kind, bounds.Dx(), bounds.Dy(), original.Len(), hex.EncodeToString(digest[:]))
	if err != nil {
		_ = os.Remove(originalPath)
		_ = os.Remove(thumbnailPath)
		return Screenshot{}, fmt.Errorf("record screenshot: %w", err)
	}
	if _, err := a.db.ExecContext(ctx, `
		INSERT INTO screenshot_ocr_jobs (screenshot_id, status)
		VALUES (?, 'not_started')
		ON CONFLICT(screenshot_id) DO NOTHING
	`, id); err != nil {
		_, _ = a.db.ExecContext(ctx, `DELETE FROM screenshots WHERE id = ?`, id)
		_ = os.Remove(originalPath)
		_ = os.Remove(thumbnailPath)
		return Screenshot{}, fmt.Errorf("initialize screenshot OCR state: %w", err)
	}
	return a.GetScreenshotContext(ctx, id)
}

func (a *Service) ListScreenshotsContext(ctx context.Context, filter ScreenshotListFilter) (ScreenshotListResult, error) {
	limit, offset, err := normalizeCollectionPage(filter.Limit, filter.Offset, defaultScreenshotLimit, maximumScreenshotLimit)
	if err != nil {
		return ScreenshotListResult{}, err
	}
	filter.Query = strings.TrimSpace(filter.Query)
	if len([]rune(filter.Query)) > 256 {
		return ScreenshotListResult{}, &ValidationError{Field: "query", Message: "screenshot search cannot exceed 256 characters"}
	}
	where := "1 = 1"
	args := []any{}
	if filter.Query != "" {
		where = `(LOWER(title) LIKE ? ESCAPE '\' OR LOWER(ocr_text) LIKE ? ESCAPE '\')`
		pattern := collectionLikePattern(filter.Query)
		args = append(args, pattern, pattern)
	}
	var total int
	if err := a.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM screenshots WHERE "+where, args...).Scan(&total); err != nil {
		return ScreenshotListResult{}, fmt.Errorf("count screenshots: %w", err)
	}
	queryArgs := append([]any(nil), args...)
	queryArgs = append(queryArgs, limit, offset)
	rows, err := a.db.QueryContext(ctx, `
		SELECT s.id, s.original_filename, COALESCE(s.edited_filename, ''), s.thumbnail_filename,
		       s.title, s.capture_kind, s.width, s.height, s.byte_size, s.ocr_text, s.ocr_language,
		       COALESCE(j.status, s.ocr_status), COALESCE(j.started_at, ''),
		       COALESCE(j.completed_at, ''), COALESCE(j.failure_code, ''),
		       COALESCE(j.failure_message, ''), s.captured_at, s.updated_at
		FROM screenshots AS s
		LEFT JOIN screenshot_ocr_jobs AS j ON j.screenshot_id = s.id
		WHERE `+where+`
		ORDER BY s.captured_at DESC, s.id DESC LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return ScreenshotListResult{}, fmt.Errorf("list screenshots: %w", err)
	}
	defer rows.Close()
	items := make([]Screenshot, 0)
	for rows.Next() {
		item, err := scanScreenshot(rows)
		if err != nil {
			return ScreenshotListResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ScreenshotListResult{}, fmt.Errorf("iterate screenshots: %w", err)
	}
	return ScreenshotListResult{Screenshots: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (a *Service) GetScreenshotContext(ctx context.Context, id string) (Screenshot, error) {
	if err := validateScreenshotID(id); err != nil {
		return Screenshot{}, err
	}
	row := a.db.QueryRowContext(ctx, `
		SELECT s.id, s.original_filename, COALESCE(s.edited_filename, ''), s.thumbnail_filename,
		       s.title, s.capture_kind, s.width, s.height, s.byte_size, s.ocr_text, s.ocr_language,
		       COALESCE(j.status, s.ocr_status), COALESCE(j.started_at, ''),
		       COALESCE(j.completed_at, ''), COALESCE(j.failure_code, ''),
		       COALESCE(j.failure_message, ''), s.captured_at, s.updated_at
		FROM screenshots AS s
		LEFT JOIN screenshot_ocr_jobs AS j ON j.screenshot_id = s.id
		WHERE s.id = ?
	`, id)
	item, err := scanScreenshot(row)
	if err == sql.ErrNoRows {
		return Screenshot{}, &NotFoundError{Resource: "screenshot", Key: id}
	}
	return item, err
}

func (a *Service) RenameScreenshotContext(ctx context.Context, id string, title string) (Screenshot, error) {
	if err := validateScreenshotID(id); err != nil {
		return Screenshot{}, err
	}
	title = strings.TrimSpace(title)
	if len([]rune(title)) > 200 {
		return Screenshot{}, &ValidationError{Field: "title", Message: "screenshot title cannot exceed 200 characters"}
	}
	result, err := a.db.ExecContext(ctx, `UPDATE screenshots SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, title, id)
	if err != nil {
		return Screenshot{}, fmt.Errorf("rename screenshot: %w", err)
	}
	if err := requireSingleMutation(result, "rename screenshot", "screenshot", false); err != nil {
		return Screenshot{}, err
	}
	return a.GetScreenshotContext(ctx, id)
}

func (a *Service) SaveScreenshotEditContext(ctx context.Context, request ScreenshotEditRequest) (Screenshot, error) {
	if err := validateScreenshotID(request.ID); err != nil {
		return Screenshot{}, err
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(request.DataURL, prefix) {
		return Screenshot{}, &ValidationError{Field: "data_url", Message: "edited screenshot must be a PNG image"}
	}
	encoded := strings.TrimPrefix(request.DataURL, prefix)
	if len(encoded) > maximumScreenshotBytes*2 {
		return Screenshot{}, &ValidationError{Field: "data_url", Message: "edited screenshot exceeds the size limit"}
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) < 1 || len(data) > maximumScreenshotBytes {
		return Screenshot{}, &ValidationError{Field: "data_url", Message: "edited screenshot data is invalid"}
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return Screenshot{}, &ValidationError{Field: "data_url", Message: "edited screenshot is not a valid PNG"}
	}
	bounds := decoded.Bounds()
	if bounds.Dx() < 1 || bounds.Dy() < 1 || bounds.Dx() > 32768 || bounds.Dy() > 32768 {
		return Screenshot{}, &ValidationError{Field: "data_url", Message: "edited screenshot dimensions are invalid"}
	}
	current, err := a.GetScreenshotContext(ctx, request.ID)
	if err != nil {
		return Screenshot{}, err
	}
	revision := uuid.NewString()
	editedFilename := request.ID + "-" + revision + "-edited.png"
	thumbnailFilename := request.ID + "-" + revision + "-thumb.png"
	directory, err := storage.ScreenshotDirectory()
	if err != nil {
		return Screenshot{}, err
	}
	if err := writeScreenshotFile(filepath.Join(directory, editedFilename), data); err != nil {
		return Screenshot{}, err
	}
	var thumbnailData bytes.Buffer
	if err := png.Encode(&thumbnailData, resizeScreenshot(decoded, 420, 260)); err != nil {
		_ = os.Remove(filepath.Join(directory, editedFilename))
		return Screenshot{}, fmt.Errorf("encode edited screenshot thumbnail: %w", err)
	}
	if err := writeScreenshotFile(filepath.Join(directory, thumbnailFilename), thumbnailData.Bytes()); err != nil {
		_ = os.Remove(filepath.Join(directory, editedFilename))
		return Screenshot{}, err
	}
	digest := sha256.Sum256(data)
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		_ = os.Remove(filepath.Join(directory, editedFilename))
		_ = os.Remove(filepath.Join(directory, thumbnailFilename))
		return Screenshot{}, fmt.Errorf("begin save screenshot edit: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE screenshots SET edited_filename = ?, thumbnail_filename = ?, width = ?, height = ?,
		       byte_size = ?, sha256 = ?, ocr_text = '', ocr_language = '', ocr_status = 'not_started', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, editedFilename, thumbnailFilename, bounds.Dx(), bounds.Dy(), len(data), hex.EncodeToString(digest[:]), request.ID)
	if err != nil {
		_ = os.Remove(filepath.Join(directory, editedFilename))
		_ = os.Remove(filepath.Join(directory, thumbnailFilename))
		return Screenshot{}, fmt.Errorf("save screenshot edit: %w", err)
	}
	if err := requireSingleMutation(result, "save screenshot edit", "screenshot", false); err != nil {
		_ = os.Remove(filepath.Join(directory, editedFilename))
		_ = os.Remove(filepath.Join(directory, thumbnailFilename))
		return Screenshot{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO screenshot_ocr_jobs (screenshot_id, status, updated_at)
		VALUES (?, 'not_started', CURRENT_TIMESTAMP)
		ON CONFLICT(screenshot_id) DO UPDATE SET
			status = 'not_started', started_at = NULL, completed_at = NULL,
			failure_code = '', failure_message = '', updated_at = CURRENT_TIMESTAMP
	`, request.ID); err != nil {
		_ = os.Remove(filepath.Join(directory, editedFilename))
		_ = os.Remove(filepath.Join(directory, thumbnailFilename))
		return Screenshot{}, fmt.Errorf("reset screenshot OCR after edit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		_ = os.Remove(filepath.Join(directory, editedFilename))
		_ = os.Remove(filepath.Join(directory, thumbnailFilename))
		return Screenshot{}, fmt.Errorf("commit screenshot edit: %w", err)
	}
	if current.EditedFilename != "" {
		_ = os.Remove(filepath.Join(directory, current.EditedFilename))
	}
	_ = os.Remove(filepath.Join(directory, current.ThumbnailFilename))
	return a.GetScreenshotContext(ctx, request.ID)
}

func (a *Service) RevertScreenshotEditContext(ctx context.Context, id string) (Screenshot, error) {
	current, err := a.GetScreenshotContext(ctx, id)
	if err != nil {
		return Screenshot{}, err
	}
	if current.EditedFilename == "" {
		return current, nil
	}
	directory, err := storage.ScreenshotDirectory()
	if err != nil {
		return Screenshot{}, err
	}
	originalPath := filepath.Join(directory, current.OriginalFilename)
	input, err := os.Open(originalPath)
	if err != nil {
		return Screenshot{}, fmt.Errorf("open original screenshot: %w", err)
	}
	original, decodeErr := png.Decode(input)
	closeErr := input.Close()
	if decodeErr != nil {
		return Screenshot{}, fmt.Errorf("decode original screenshot: %w", decodeErr)
	}
	if closeErr != nil {
		return Screenshot{}, fmt.Errorf("close original screenshot: %w", closeErr)
	}
	originalData, err := os.ReadFile(originalPath)
	if err != nil {
		return Screenshot{}, fmt.Errorf("read original screenshot: %w", err)
	}
	var thumbnailData bytes.Buffer
	if err := png.Encode(&thumbnailData, resizeScreenshot(original, 420, 260)); err != nil {
		return Screenshot{}, fmt.Errorf("encode original screenshot thumbnail: %w", err)
	}
	thumbnailFilename := id + "-" + uuid.NewString() + "-thumb.png"
	if err := writeScreenshotFile(filepath.Join(directory, thumbnailFilename), thumbnailData.Bytes()); err != nil {
		return Screenshot{}, err
	}
	digest := sha256.Sum256(originalData)
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		_ = os.Remove(filepath.Join(directory, thumbnailFilename))
		return Screenshot{}, fmt.Errorf("begin revert screenshot edit: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE screenshots
		SET edited_filename = NULL, thumbnail_filename = ?, width = ?, height = ?,
		    byte_size = ?, sha256 = ?, ocr_text = '', ocr_language = '',
		    ocr_status = 'not_started', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, thumbnailFilename, original.Bounds().Dx(), original.Bounds().Dy(), len(originalData), hex.EncodeToString(digest[:]), id)
	if err != nil {
		_ = os.Remove(filepath.Join(directory, thumbnailFilename))
		return Screenshot{}, fmt.Errorf("revert screenshot edit: %w", err)
	}
	if err := requireSingleMutation(result, "revert screenshot edit", "screenshot", false); err != nil {
		_ = os.Remove(filepath.Join(directory, thumbnailFilename))
		return Screenshot{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE screenshot_ocr_jobs SET status = 'not_started', started_at = NULL,
			completed_at = NULL, failure_code = '', failure_message = '', updated_at = CURRENT_TIMESTAMP
		WHERE screenshot_id = ?
	`, id); err != nil {
		_ = os.Remove(filepath.Join(directory, thumbnailFilename))
		return Screenshot{}, fmt.Errorf("reset screenshot OCR after revert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		_ = os.Remove(filepath.Join(directory, thumbnailFilename))
		return Screenshot{}, fmt.Errorf("commit screenshot revert: %w", err)
	}
	_ = os.Remove(filepath.Join(directory, current.EditedFilename))
	_ = os.Remove(filepath.Join(directory, current.ThumbnailFilename))
	return a.GetScreenshotContext(ctx, id)
}

func (a *Service) ScreenshotFilePathContext(ctx context.Context, id string) (string, error) {
	item, err := a.GetScreenshotContext(ctx, id)
	if err != nil {
		return "", err
	}
	filename := item.OriginalFilename
	if item.EditedFilename != "" {
		filename = item.EditedFilename
		item.HasEdit = true
	}
	if !screenshotFilenamePattern.MatchString(filename) {
		return "", fmt.Errorf("stored screenshot filename is invalid")
	}
	directory, err := storage.ScreenshotDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, filename), nil
}

func (a *Service) QueueScreenshotOCRContext(ctx context.Context, id string) (ScreenshotOCRQueueResult, error) {
	if err := validateScreenshotID(id); err != nil {
		return ScreenshotOCRQueueResult{}, err
	}
	result, err := a.db.ExecContext(ctx, `
		INSERT INTO screenshot_ocr_jobs (
			screenshot_id, status, started_at, completed_at,
			failure_code, failure_message, updated_at
		)
		SELECT id, 'queued', NULL, NULL, '', '', CURRENT_TIMESTAMP
		FROM screenshots WHERE id = ?
		ON CONFLICT(screenshot_id) DO UPDATE SET
			status = 'queued', started_at = NULL, completed_at = NULL,
			failure_code = '', failure_message = '', updated_at = CURRENT_TIMESTAMP
		WHERE screenshot_ocr_jobs.status NOT IN ('queued', 'processing')
	`, id)
	if err != nil {
		return ScreenshotOCRQueueResult{}, fmt.Errorf("queue screenshot OCR: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ScreenshotOCRQueueResult{}, fmt.Errorf("check queued screenshot OCR: %w", err)
	}
	item, err := a.GetScreenshotContext(ctx, id)
	if err != nil {
		return ScreenshotOCRQueueResult{}, err
	}
	a.emitScreenshotOCRStatus(item)
	return ScreenshotOCRQueueResult{Screenshot: item, Changed: affected > 0}, nil
}

func (a *Service) SetScreenshotOCRProcessingContext(ctx context.Context, id string) (bool, error) {
	if err := validateScreenshotID(id); err != nil {
		return false, err
	}
	result, err := a.db.ExecContext(ctx, `
		UPDATE screenshot_ocr_jobs
		SET status = 'processing', started_at = CURRENT_TIMESTAMP, completed_at = NULL,
		    failure_code = '', failure_message = '', attempt = attempt + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE screenshot_id = ? AND status = 'queued'
	`, id)
	if err != nil {
		return false, fmt.Errorf("start screenshot OCR: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("check screenshot OCR start: %w", err)
	}
	if affected > 0 {
		if item, loadErr := a.GetScreenshotContext(ctx, id); loadErr == nil {
			a.emitScreenshotOCRStatus(item)
		}
	}
	return affected > 0, nil
}

func (a *Service) CompleteScreenshotOCRContext(ctx context.Context, id, text, language string) (Screenshot, error) {
	if err := validateScreenshotID(id); err != nil {
		return Screenshot{}, err
	}
	if len([]byte(text)) > 4*1024*1024 {
		return Screenshot{}, &ValidationError{Field: "ocr_text", Message: "recognized text exceeds the storage limit"}
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return Screenshot{}, fmt.Errorf("begin complete screenshot OCR: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE screenshot_ocr_jobs
		SET status = 'complete', completed_at = CURRENT_TIMESTAMP,
		    failure_code = '', failure_message = '', updated_at = CURRENT_TIMESTAMP
		WHERE screenshot_id = ? AND status = 'processing'
	`, id)
	if err != nil {
		return Screenshot{}, fmt.Errorf("complete screenshot OCR: %w", err)
	}
	if err := requireSingleMutation(result, "complete screenshot OCR", "screenshot", false); err != nil {
		return Screenshot{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE screenshots
		SET ocr_text = ?, ocr_language = ?, ocr_status = 'complete', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, text, language, id); err != nil {
		return Screenshot{}, fmt.Errorf("store screenshot OCR text: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Screenshot{}, fmt.Errorf("commit screenshot OCR: %w", err)
	}
	item, err := a.GetScreenshotContext(ctx, id)
	if err == nil {
		a.emitScreenshotOCRStatus(item)
	}
	return item, err
}

func (a *Service) FailScreenshotOCRContext(ctx context.Context, id string, unsupported bool, code string, message string) error {
	if err := validateScreenshotID(id); err != nil {
		return err
	}
	status := "failed"
	if unsupported {
		status = "unsupported"
		code = "unsupported"
		message = "Local OCR is unavailable on this platform."
	}
	code = normalizeOCRFailureValue(code, "ocr_failed", 64)
	message = normalizeOCRFailureValue(message, "Text extraction failed. You can try again.", 240)
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin fail screenshot OCR: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE screenshot_ocr_jobs
		SET status = ?, completed_at = CURRENT_TIMESTAMP, failure_code = ?,
		    failure_message = ?, updated_at = CURRENT_TIMESTAMP
		WHERE screenshot_id = ? AND status IN ('queued', 'processing')
	`, status, code, message, id)
	if err != nil {
		return fmt.Errorf("fail screenshot OCR: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check failed screenshot OCR: %w", err)
	}
	if affected == 0 {
		insertResult, err := tx.ExecContext(ctx, `
			INSERT INTO screenshot_ocr_jobs (
				screenshot_id, status, completed_at, failure_code, failure_message, updated_at
			)
			SELECT id, ?, CURRENT_TIMESTAMP, ?, ?, CURRENT_TIMESTAMP FROM screenshots WHERE id = ?
			ON CONFLICT(screenshot_id) DO NOTHING
		`, status, code, message, id)
		if err != nil {
			return fmt.Errorf("record failed screenshot OCR: %w", err)
		}
		affected, err = insertResult.RowsAffected()
		if err != nil {
			return fmt.Errorf("check recorded screenshot OCR failure: %w", err)
		}
		if affected == 0 {
			return nil
		}
	}
	legacyStatus := status
	if legacyStatus == "queued" {
		legacyStatus = "not_started"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE screenshots SET ocr_status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, legacyStatus, id); err != nil {
		return fmt.Errorf("update screenshot OCR failure: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit screenshot OCR failure: %w", err)
	}
	if item, loadErr := a.GetScreenshotContext(ctx, id); loadErr == nil {
		a.emitScreenshotOCRStatus(item)
	}
	return nil
}

func (a *Service) CancelScreenshotOCRContext(ctx context.Context, id string) (Screenshot, bool, error) {
	if err := validateScreenshotID(id); err != nil {
		return Screenshot{}, false, err
	}
	result, err := a.db.ExecContext(ctx, `
		UPDATE screenshot_ocr_jobs
		SET status = 'failed', completed_at = CURRENT_TIMESTAMP,
		    failure_code = 'cancelled', failure_message = 'Text extraction was cancelled.',
		    updated_at = CURRENT_TIMESTAMP
		WHERE screenshot_id = ? AND status IN ('queued', 'processing')
	`, id)
	if err != nil {
		return Screenshot{}, false, fmt.Errorf("cancel screenshot OCR: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Screenshot{}, false, fmt.Errorf("check screenshot OCR cancellation: %w", err)
	}
	item, err := a.GetScreenshotContext(ctx, id)
	if err != nil {
		return Screenshot{}, false, err
	}
	if affected > 0 {
		a.emitScreenshotOCRStatus(item)
	}
	return item, affected > 0, nil
}

func (a *Service) RecoverInterruptedScreenshotOCRContext(ctx context.Context) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE screenshot_ocr_jobs
		SET status = 'failed', completed_at = CURRENT_TIMESTAMP,
		    failure_code = 'interrupted',
		    failure_message = 'Text extraction was interrupted. You can try again.',
		    updated_at = CURRENT_TIMESTAMP
		WHERE status IN ('queued', 'processing')
	`)
	if err != nil {
		return fmt.Errorf("recover interrupted screenshot OCR: %w", err)
	}
	return nil
}

// CleanupOrphanedScreenshotFilesContext reconciles the app-owned screenshot
// directory after an interrupted edit, revert, or deletion. It deliberately
// ignores files outside the application's strict screenshot naming schemes.
func (a *Service) CleanupOrphanedScreenshotFilesContext(ctx context.Context) error {
	directory, err := storage.ScreenshotDirectory()
	if err != nil {
		return err
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT original_filename, COALESCE(edited_filename, ''), thumbnail_filename
		FROM screenshots
	`)
	if err != nil {
		return fmt.Errorf("list referenced screenshot files: %w", err)
	}
	referenced := make(map[string]struct{})
	for rows.Next() {
		var original, edited, thumbnail string
		if err := rows.Scan(&original, &edited, &thumbnail); err != nil {
			rows.Close()
			return fmt.Errorf("scan referenced screenshot files: %w", err)
		}
		for _, filename := range []string{original, edited, thumbnail} {
			if screenshotFilenamePattern.MatchString(filename) {
				referenced[filename] = struct{}{}
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate referenced screenshot files: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close referenced screenshot files: %w", err)
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("list screenshot storage: %w", err)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		name := entry.Name()
		path := filepath.Join(directory, name)
		if strings.HasSuffix(name, ".deleting") {
			originalName := strings.TrimSuffix(name, ".deleting")
			if !screenshotFilenamePattern.MatchString(originalName) {
				continue
			}
			if _, stillReferenced := referenced[originalName]; stillReferenced {
				originalPath := filepath.Join(directory, originalName)
				if _, statErr := os.Stat(originalPath); os.IsNotExist(statErr) {
					if err := os.Rename(path, originalPath); err != nil {
						return fmt.Errorf("restore staged screenshot file: %w", err)
					}
					continue
				} else if statErr != nil {
					return fmt.Errorf("inspect referenced screenshot file: %w", statErr)
				}
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove staged screenshot file: %w", err)
			}
			continue
		}
		_, stillReferenced := referenced[name]
		controlledTemporary := strings.HasPrefix(name, ".screenshot-") && strings.HasSuffix(name, ".tmp")
		if (screenshotFilenamePattern.MatchString(name) && !stillReferenced) || controlledTemporary {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove orphaned screenshot file: %w", err)
			}
		}
	}
	return nil
}

func normalizeOCRFailureValue(value, fallback string, maximum int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	runes := []rune(value)
	if len(runes) > maximum {
		value = string(runes[:maximum])
	}
	return value
}

func (a *Service) emitScreenshotOCRStatus(item Screenshot) {
	ctx := a.requestContext()
	if ctx.Value("events") == nil {
		return
	}
	runtime.EventsEmit(ctx, "screenshots:ocr-status", ScreenshotOCRStatusEvent{
		ID: item.ID, Status: item.OCRStatus, StartedAt: item.OCRStartedAt,
		CompletedAt: item.OCRCompletedAt, FailureCode: item.OCRFailureCode,
		FailureMessage: item.OCRFailureMessage,
	})
}

func (a *Service) DeleteScreenshotContext(ctx context.Context, id string) error {
	item, err := a.GetScreenshotContext(ctx, id)
	if err != nil {
		return err
	}
	directory, err := storage.ScreenshotDirectory()
	if err != nil {
		return err
	}
	filenames := []string{item.OriginalFilename, item.EditedFilename, item.ThumbnailFilename}
	staged := make(map[string]string)
	for _, filename := range filenames {
		if filename == "" || !screenshotFilenamePattern.MatchString(filename) {
			continue
		}
		original := filepath.Join(directory, filename)
		temporary := original + ".deleting"
		if err := os.Rename(original, temporary); err != nil && !os.IsNotExist(err) {
			for from, to := range staged {
				_ = os.Rename(from, to)
			}
			return fmt.Errorf("stage screenshot deletion: %w", err)
		} else if err == nil {
			staged[temporary] = original
		}
	}
	result, err := a.db.ExecContext(ctx, `DELETE FROM screenshots WHERE id = ?`, id)
	if err != nil {
		for from, to := range staged {
			_ = os.Rename(from, to)
		}
		return fmt.Errorf("delete screenshot: %w", err)
	}
	if err := requireSingleMutation(result, "delete screenshot", "screenshot", false); err != nil {
		for from, to := range staged {
			_ = os.Rename(from, to)
		}
		return err
	}
	for path := range staged {
		_ = os.Remove(path)
	}
	return nil
}

type screenshotScanner interface{ Scan(...any) error }

func scanScreenshot(scanner screenshotScanner) (Screenshot, error) {
	var item Screenshot
	err := scanner.Scan(&item.ID, &item.OriginalFilename, &item.EditedFilename, &item.ThumbnailFilename,
		&item.Title, &item.CaptureKind, &item.Width, &item.Height, &item.ByteSize, &item.OCRText,
		&item.OCRLanguage, &item.OCRStatus, &item.OCRStartedAt, &item.OCRCompletedAt,
		&item.OCRFailureCode, &item.OCRFailureMessage, &item.CapturedAt, &item.UpdatedAt)
	if err != nil {
		return Screenshot{}, err
	}
	filename := item.OriginalFilename
	if item.EditedFilename != "" {
		filename = item.EditedFilename
		item.HasEdit = true
	}
	item.ImageURL = screenshotRoutePrefix + filename
	item.ThumbnailURL = screenshotRoutePrefix + item.ThumbnailFilename
	return item, nil
}

func validateScreenshotID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return &ValidationError{Field: "id", Message: "screenshot ID is invalid"}
	}
	return nil
}

func writeScreenshotFile(path string, data []byte) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".screenshot-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary screenshot: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write screenshot: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close screenshot: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("install screenshot: %w", err)
	}
	return nil
}

func resizeScreenshot(source image.Image, maximumWidth, maximumHeight int) image.Image {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	scale := mathMin(float64(maximumWidth)/float64(width), float64(maximumHeight)/float64(height))
	if scale >= 1 {
		return source
	}
	targetWidth := maximumInt(1, int(float64(width)*scale))
	targetHeight := maximumInt(1, int(float64(height)*scale))
	target := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	for y := 0; y < targetHeight; y++ {
		sourceY := bounds.Min.Y + y*height/targetHeight
		for x := 0; x < targetWidth; x++ {
			sourceX := bounds.Min.X + x*width/targetWidth
			target.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	return target
}

func mathMin(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}
func maximumInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func NewScreenshotImageHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			writer.Header().Set("Allow", "GET, HEAD")
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		filename := strings.TrimPrefix(request.URL.Path, screenshotRoutePrefix)
		if !strings.HasPrefix(request.URL.Path, screenshotRoutePrefix) || !screenshotFilenamePattern.MatchString(filename) {
			http.NotFound(writer, request)
			return
		}
		directory, err := storage.ScreenshotDirectory()
		if err != nil {
			http.Error(writer, "screenshot storage unavailable", http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Cache-Control", "private, no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(writer, request, filepath.Join(directory, filename))
	})
}
