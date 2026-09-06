package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"something/backend/storage"
)

const (
	desktopAppIconRoutePrefix  = "/app-icons/"
	maximumDesktopAppIconBytes = 5 * 1024 * 1024
)

var (
	desktopAppIconFilenamePattern  = regexp.MustCompile(`^[a-f0-9]{32}\.(ico|jpg|png|webp)$`)
	desktopAppIconExtensionsByMIME = map[string]string{
		"image/jpeg":   ".jpg",
		"image/png":    ".png",
		"image/webp":   ".webp",
		"image/x-icon": ".ico",
	}
)

func (a *Service) ImportDesktopAppIconFromPathContext(
	ctx context.Context,
	appID int,
	sourcePath string,
) (DesktopApp, error) {
	if _, err := loadDesktopAppContext(ctx, a.db, appID); err != nil {
		return DesktopApp{}, err
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return DesktopApp{}, fmt.Errorf("open selected application icon: %w", err)
	}
	defer source.Close()

	info, err := source.Stat()
	if err != nil {
		return DesktopApp{}, fmt.Errorf("inspect selected application icon: %w", err)
	}
	if !info.Mode().IsRegular() {
		return DesktopApp{}, &ValidationError{Field: "icon", Message: "Choose a regular image file."}
	}
	if info.Size() <= 0 || info.Size() > maximumDesktopAppIconBytes {
		return DesktopApp{}, &ValidationError{Field: "icon", Message: "Application icons must be between 1 byte and 5 MB."}
	}

	header := make([]byte, 512)
	headerLength, readErr := io.ReadFull(source, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return DesktopApp{}, fmt.Errorf("inspect selected application icon contents: %w", readErr)
	}
	if headerLength == 0 {
		return DesktopApp{}, &ValidationError{Field: "icon", Message: "The selected application icon is empty."}
	}
	mimeType, extension, err := detectDesktopAppIconFormat(header[:headerLength])
	if err != nil {
		return DesktopApp{}, err
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return DesktopApp{}, fmt.Errorf("rewind selected application icon: %w", err)
	}

	directory, err := storage.DesktopAppIconDirectory()
	if err != nil {
		return DesktopApp{}, err
	}
	id, err := randomDesktopAppIconID()
	if err != nil {
		return DesktopApp{}, err
	}
	filename := id + extension
	destinationPath := filepath.Join(directory, filename)

	temporary, err := os.CreateTemp(directory, ".app-icon-upload-*.tmp")
	if err != nil {
		return DesktopApp{}, fmt.Errorf("create temporary application icon: %w", err)
	}
	temporaryPath := temporary.Name()
	keepTemporary := true
	defer func() {
		_ = temporary.Close()
		if keepTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return DesktopApp{}, fmt.Errorf("secure temporary application icon: %w", err)
	}

	written, err := io.Copy(temporary, io.LimitReader(source, maximumDesktopAppIconBytes+1))
	if err != nil {
		return DesktopApp{}, fmt.Errorf("copy selected application icon: %w", err)
	}
	if written > maximumDesktopAppIconBytes {
		return DesktopApp{}, &ValidationError{Field: "icon", Message: "Application icons cannot exceed 5 MB."}
	}
	if err := temporary.Sync(); err != nil {
		return DesktopApp{}, fmt.Errorf("flush selected application icon: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return DesktopApp{}, fmt.Errorf("close selected application icon: %w", err)
	}
	if err := os.Rename(temporaryPath, destinationPath); err != nil {
		return DesktopApp{}, fmt.Errorf("install selected application icon: %w", err)
	}
	keepTemporary = false

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		_ = os.Remove(destinationPath)
		return DesktopApp{}, fmt.Errorf("begin application icon update: %w", err)
	}
	defer tx.Rollback()
	var previousFilename string
	previousErr := tx.QueryRowContext(
		ctx,
		`SELECT filename FROM desktop_app_icons WHERE app_id = ?`,
		appID,
	).Scan(&previousFilename)
	if previousErr != nil && !errors.Is(previousErr, sql.ErrNoRows) {
		_ = os.Remove(destinationPath)
		return DesktopApp{}, fmt.Errorf("load previous application icon: %w", previousErr)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO desktop_app_icons (app_id, filename, mime_type, byte_size)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(app_id) DO UPDATE SET
			filename = excluded.filename,
			mime_type = excluded.mime_type,
			byte_size = excluded.byte_size,
			updated_at = CURRENT_TIMESTAMP
	`, appID, filename, mimeType, written); err != nil {
		_ = os.Remove(destinationPath)
		return DesktopApp{}, fmt.Errorf("save application icon: %w", err)
	}
	if err := tx.Commit(); err != nil {
		_ = os.Remove(destinationPath)
		return DesktopApp{}, fmt.Errorf("commit application icon update: %w", err)
	}
	if desktopAppIconFilenamePattern.MatchString(previousFilename) && previousFilename != filename {
		_ = os.Remove(filepath.Join(directory, previousFilename))
	}

	return loadDesktopAppContext(ctx, a.db, appID)
}

func (a *Service) DeleteDesktopAppIconContext(ctx context.Context, appID int) (DesktopApp, error) {
	app, err := loadDesktopAppContext(ctx, a.db, appID)
	if err != nil {
		return DesktopApp{}, err
	}
	var filename string
	err = a.db.QueryRowContext(
		ctx,
		`SELECT filename FROM desktop_app_icons WHERE app_id = ?`,
		appID,
	).Scan(&filename)
	if errors.Is(err, sql.ErrNoRows) {
		return app, nil
	}
	if err != nil {
		return DesktopApp{}, fmt.Errorf("load application icon: %w", err)
	}

	staged, err := stageDesktopAppIconDeletion(filename)
	if err != nil {
		return DesktopApp{}, err
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		staged.restore()
		return DesktopApp{}, fmt.Errorf("begin application icon deletion: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM desktop_app_icons WHERE app_id = ?`, appID)
	if err != nil {
		staged.restore()
		return DesktopApp{}, fmt.Errorf("delete application icon: %w", err)
	}
	if err := requireSingleMutation(result, "delete application icon", "application icon", false); err != nil {
		staged.restore()
		return DesktopApp{}, err
	}
	if err := tx.Commit(); err != nil {
		staged.restore()
		return DesktopApp{}, fmt.Errorf("commit application icon deletion: %w", err)
	}
	staged.finish()
	return loadDesktopAppContext(ctx, a.db, appID)
}

func (a *Service) deleteDesktopAppAndIconContext(ctx context.Context, appID int) error {
	if err := ValidateDesktopAppID(appID); err != nil {
		return err
	}
	var filename string
	iconErr := a.db.QueryRowContext(
		ctx,
		`SELECT filename FROM desktop_app_icons WHERE app_id = ?`,
		appID,
	).Scan(&filename)
	if iconErr != nil && !errors.Is(iconErr, sql.ErrNoRows) {
		return fmt.Errorf("load application icon before deletion: %w", iconErr)
	}

	var staged *stagedDesktopAppIcon
	var err error
	if iconErr == nil {
		staged, err = stageDesktopAppIconDeletion(filename)
		if err != nil {
			return err
		}
	}
	restoreIcon := func() {
		if staged != nil {
			staged.restore()
		}
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		restoreIcon()
		return fmt.Errorf("begin desktop app deletion: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM desktop_apps WHERE id = ?`, appID)
	if err != nil {
		restoreIcon()
		return fmt.Errorf("delete desktop app: %w", err)
	}
	if err := requireSingleMutation(result, "delete desktop app", "desktop app", false); err != nil {
		restoreIcon()
		return err
	}
	if err := tx.Commit(); err != nil {
		restoreIcon()
		return fmt.Errorf("commit desktop app deletion: %w", err)
	}
	if staged != nil {
		staged.finish()
	}
	return nil
}

func detectDesktopAppIconFormat(header []byte) (string, string, error) {
	if len(header) >= 4 && bytes.Equal(header[:4], []byte{0x00, 0x00, 0x01, 0x00}) {
		return "image/x-icon", desktopAppIconExtensionsByMIME["image/x-icon"], nil
	}
	mimeType := http.DetectContentType(header)
	extension, supported := desktopAppIconExtensionsByMIME[mimeType]
	if !supported {
		return "", "", &ValidationError{
			Field:   "icon",
			Message: "Choose an ICO, JPEG, PNG, or WebP image.",
		}
	}
	return mimeType, extension, nil
}

func desktopAppIconURL(filename string) string {
	if !desktopAppIconFilenamePattern.MatchString(filename) {
		return ""
	}
	directory, err := storage.DesktopAppIconDirectory()
	if err != nil {
		return ""
	}
	info, err := os.Stat(filepath.Join(directory, filename))
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return desktopAppIconRoutePrefix + filename
}

func randomDesktopAppIconID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate application icon ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}

type stagedDesktopAppIcon struct {
	originalPath string
	stagedPath   string
	moved        bool
}

func stageDesktopAppIconDeletion(filename string) (*stagedDesktopAppIcon, error) {
	if !desktopAppIconFilenamePattern.MatchString(filename) {
		return nil, fmt.Errorf("stored application icon filename is invalid")
	}
	directory, err := storage.DesktopAppIconDirectory()
	if err != nil {
		return nil, err
	}
	stagedID, err := randomDesktopAppIconID()
	if err != nil {
		return nil, err
	}
	staged := &stagedDesktopAppIcon{
		originalPath: filepath.Join(directory, filename),
		stagedPath:   filepath.Join(directory, "."+stagedID+".deleting"),
	}
	if err := os.Rename(staged.originalPath, staged.stagedPath); err == nil {
		staged.moved = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stage application icon deletion: %w", err)
	}
	return staged, nil
}

func (s *stagedDesktopAppIcon) restore() {
	if s != nil && s.moved {
		_ = os.Rename(s.stagedPath, s.originalPath)
	}
}

func (s *stagedDesktopAppIcon) finish() {
	if s != nil && s.moved {
		_ = os.Remove(s.stagedPath)
	}
}

func NewDesktopAppIconHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.URL.Path, desktopAppIconRoutePrefix) {
			http.NotFound(w, r)
			return
		}
		filename := strings.TrimPrefix(r.URL.Path, desktopAppIconRoutePrefix)
		if !desktopAppIconFilenamePattern.MatchString(filename) {
			http.NotFound(w, r)
			return
		}
		directory, err := storage.DesktopAppIconDirectory()
		if err != nil {
			http.Error(w, "application icon storage unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, filepath.Join(directory, filename))
	})
}

func NewUserAssetHandler() http.Handler {
	wallpapers := NewUserWallpaperHandler()
	icons := NewDesktopAppIconHandler()
	setupIcons := NewSetupIconHandler()
	steamImages := NewSteamGameImageHandler()
	screenshots := NewScreenshotImageHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, desktopAppIconRoutePrefix) {
			icons.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, setupIconRoutePrefix) {
			setupIcons.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, steamGameImageRoutePrefix) {
			steamImages.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, screenshotRoutePrefix) {
			screenshots.ServeHTTP(w, r)
			return
		}
		wallpapers.ServeHTTP(w, r)
	})
}
