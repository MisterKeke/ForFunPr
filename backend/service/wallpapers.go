package service

import (
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
	"slices"
	"strings"

	"something/backend/storage"
)

const (
	defaultWallpaperSelection = "builtin:hu-tao"
	selectedWallpaperStateKey = "selected_wallpaper"
	userWallpaperRoutePrefix  = "/user-wallpapers/"
	maximumWallpaperBytes     = 20 * 1024 * 1024
)

var (
	builtinWallpaperSelections = map[string]struct{}{
		"builtin:original": {},
		"builtin:sandrone": {},
		"builtin:hu-tao":   {},
		"builtin:skirk":    {},
	}
	userWallpaperIDPattern       = regexp.MustCompile(`^[a-f0-9]{32}$`)
	userWallpaperFilenamePattern = regexp.MustCompile(`^[a-f0-9]{32}\.(jpg|png|webp)$`)
	wallpaperExtensionsByMIME    = map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
	}
)

// ListBuiltinWallpaperSelections returns the stable selection keys accepted
// by SelectWallpaperContext. It keeps API, CLI, and MCP clients from copying
// the service's private validation table.
func ListBuiltinWallpaperSelections() []string {
	selections := make([]string, 0, len(builtinWallpaperSelections))
	for selection := range builtinWallpaperSelections {
		selections = append(selections, selection)
	}
	slices.Sort(selections)
	return selections
}

// UserWallpaper is one application-owned image stored outside the embedded
// frontend assets.
type UserWallpaper struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Filename    string `json:"filename"`
	MIMEType    string `json:"mime_type"`
	ByteSize    int64  `json:"byte_size"`
	URL         string `json:"url"`
}

// WallpaperSettings is the complete appearance payload needed by the
// frontend. SelectionSaved lets the UI migrate the previous localStorage
// preference once for existing users.
type WallpaperSettings struct {
	Selected       string          `json:"selected"`
	SelectionSaved bool            `json:"selection_saved"`
	UserWallpapers []UserWallpaper `json:"user_wallpapers"`
}

func (a *Service) GetWallpaperSettingsContext(ctx context.Context) (*WallpaperSettings, error) {
	wallpapers, err := a.listUserWallpapersContext(ctx)
	if err != nil {
		// Custom wallpaper storage is optional. Keep the built-in selections
		// functional when that directory could not be initialised at startup.
		if a.CapabilityAvailable("wallpaper") {
			return nil, err
		}
		wallpapers = []UserWallpaper{}
	}

	selected := defaultWallpaperSelection
	selectionSaved := false
	err = a.db.QueryRowContext(
		ctx,
		`SELECT value FROM app_state WHERE key = ?`,
		selectedWallpaperStateKey,
	).Scan(&selected)
	if err == nil {
		selectionSaved = wallpaperSelectionAvailable(selected, wallpapers)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("read selected wallpaper: %w", err)
	}
	if !selectionSaved {
		selected = defaultWallpaperSelection
	}

	return &WallpaperSettings{
		Selected:       selected,
		SelectionSaved: selectionSaved,
		UserWallpapers: wallpapers,
	}, nil
}

func (a *Service) listUserWallpapersContext(ctx context.Context) ([]UserWallpaper, error) {
	directory, err := storage.WallpaperDirectory()
	if err != nil {
		return nil, err
	}

	rows, err := a.db.QueryContext(ctx, `
		SELECT id, display_name, filename, mime_type, byte_size
		FROM user_wallpapers
		ORDER BY created_at DESC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list user wallpapers: %w", err)
	}
	defer rows.Close()

	wallpapers := []UserWallpaper{}
	for rows.Next() {
		var wallpaper UserWallpaper
		if err := rows.Scan(
			&wallpaper.ID,
			&wallpaper.DisplayName,
			&wallpaper.Filename,
			&wallpaper.MIMEType,
			&wallpaper.ByteSize,
		); err != nil {
			return nil, fmt.Errorf("scan user wallpaper: %w", err)
		}
		if !userWallpaperIDPattern.MatchString(wallpaper.ID) ||
			!userWallpaperFilenamePattern.MatchString(wallpaper.Filename) {
			continue
		}
		info, err := os.Stat(filepath.Join(directory, wallpaper.Filename))
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		wallpaper.URL = userWallpaperRoutePrefix + wallpaper.Filename
		wallpapers = append(wallpapers, wallpaper)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user wallpapers: %w", err)
	}
	return wallpapers, nil
}

func wallpaperSelectionAvailable(selection string, wallpapers []UserWallpaper) bool {
	if _, ok := builtinWallpaperSelections[selection]; ok {
		return true
	}
	if !strings.HasPrefix(selection, "custom:") {
		return false
	}
	id := strings.TrimPrefix(selection, "custom:")
	for _, wallpaper := range wallpapers {
		if wallpaper.ID == id {
			return true
		}
	}
	return false
}

func (a *Service) SelectWallpaperContext(ctx context.Context, selection string) (*WallpaperSettings, error) {
	selection = strings.TrimSpace(selection)
	if _, ok := builtinWallpaperSelections[selection]; !ok {
		if !strings.HasPrefix(selection, "custom:") {
			return nil, &ValidationError{Field: "wallpaper", Message: "choose a valid wallpaper"}
		}
		id := strings.TrimPrefix(selection, "custom:")
		if !userWallpaperIDPattern.MatchString(id) {
			return nil, &ValidationError{Field: "wallpaper", Message: "choose a valid wallpaper"}
		}
		var exists int
		if err := a.db.QueryRowContext(
			ctx,
			`SELECT EXISTS(SELECT 1 FROM user_wallpapers WHERE id = ?)`,
			id,
		).Scan(&exists); err != nil {
			return nil, fmt.Errorf("check selected wallpaper: %w", err)
		}
		if exists == 0 {
			return nil, &NotFoundError{Resource: "user wallpaper", Key: id}
		}
	}

	if err := upsertAppStateValue(ctx, a.db, selectedWallpaperStateKey, selection); err != nil {
		return nil, fmt.Errorf("save selected wallpaper: %w", err)
	}
	return a.GetWallpaperSettingsContext(ctx)
}

func (a *Service) ImportWallpaperFromPathContext(ctx context.Context, sourcePath string) (*UserWallpaper, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("open selected wallpaper: %w", err)
	}
	defer source.Close()

	info, err := source.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect selected wallpaper: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, &ValidationError{Field: "wallpaper", Message: "the selected item must be a file"}
	}
	if info.Size() <= 0 || info.Size() > maximumWallpaperBytes {
		return nil, &ValidationError{Field: "wallpaper", Message: "wallpapers must be between 1 byte and 20 MB"}
	}

	header := make([]byte, 512)
	headerLength, readErr := io.ReadFull(source, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("inspect selected wallpaper contents: %w", readErr)
	}
	if headerLength == 0 {
		return nil, &ValidationError{Field: "wallpaper", Message: "the selected wallpaper is empty"}
	}
	mimeType := http.DetectContentType(header[:headerLength])
	extension, supported := wallpaperExtensionsByMIME[mimeType]
	if !supported {
		return nil, &ValidationError{Field: "wallpaper", Message: "choose a JPEG, PNG, or WebP image"}
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind selected wallpaper: %w", err)
	}

	directory, err := storage.WallpaperDirectory()
	if err != nil {
		return nil, err
	}
	id, err := randomWallpaperID()
	if err != nil {
		return nil, err
	}
	filename := id + extension
	destinationPath := filepath.Join(directory, filename)

	temporary, err := os.CreateTemp(directory, ".wallpaper-upload-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temporary wallpaper: %w", err)
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
		return nil, fmt.Errorf("secure temporary wallpaper: %w", err)
	}

	written, err := io.Copy(temporary, io.LimitReader(source, maximumWallpaperBytes+1))
	if err != nil {
		return nil, fmt.Errorf("copy selected wallpaper: %w", err)
	}
	if written > maximumWallpaperBytes {
		return nil, &ValidationError{Field: "wallpaper", Message: "wallpapers cannot exceed 20 MB"}
	}
	if err := temporary.Sync(); err != nil {
		return nil, fmt.Errorf("flush selected wallpaper: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return nil, fmt.Errorf("close selected wallpaper: %w", err)
	}
	if err := os.Rename(temporaryPath, destinationPath); err != nil {
		return nil, fmt.Errorf("install selected wallpaper: %w", err)
	}
	keepTemporary = false

	displayName := wallpaperDisplayName(sourcePath)
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		_ = os.Remove(destinationPath)
		return nil, fmt.Errorf("begin wallpaper import: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_wallpapers (id, display_name, filename, mime_type, byte_size)
		VALUES (?, ?, ?, ?, ?)
	`, id, displayName, filename, mimeType, written); err != nil {
		_ = os.Remove(destinationPath)
		return nil, fmt.Errorf("save user wallpaper: %w", err)
	}
	if err := upsertAppStateValue(ctx, tx, selectedWallpaperStateKey, "custom:"+id); err != nil {
		_ = os.Remove(destinationPath)
		return nil, fmt.Errorf("select imported wallpaper: %w", err)
	}
	if err := tx.Commit(); err != nil {
		_ = os.Remove(destinationPath)
		return nil, fmt.Errorf("commit wallpaper import: %w", err)
	}

	return &UserWallpaper{
		ID:          id,
		DisplayName: displayName,
		Filename:    filename,
		MIMEType:    mimeType,
		ByteSize:    written,
		URL:         userWallpaperRoutePrefix + filename,
	}, nil
}

func wallpaperDisplayName(sourcePath string) string {
	base := filepath.Base(sourcePath)
	name := strings.TrimSpace(strings.TrimSuffix(base, filepath.Ext(base)))
	if name == "" {
		return "Uploaded wallpaper"
	}
	runes := []rune(name)
	if len(runes) > 120 {
		name = string(runes[:120])
	}
	return name
}

func randomWallpaperID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate wallpaper ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func (a *Service) DeleteUserWallpaperContext(ctx context.Context, id string) (*WallpaperSettings, error) {
	id = strings.TrimSpace(id)
	if !userWallpaperIDPattern.MatchString(id) {
		return nil, &ValidationError{Field: "wallpaper", Message: "choose a valid user wallpaper"}
	}

	var filename string
	if err := a.db.QueryRowContext(
		ctx,
		`SELECT filename FROM user_wallpapers WHERE id = ?`,
		id,
	).Scan(&filename); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &NotFoundError{Resource: "user wallpaper", Key: id}
		}
		return nil, fmt.Errorf("load user wallpaper: %w", err)
	}
	if !userWallpaperFilenamePattern.MatchString(filename) {
		return nil, fmt.Errorf("stored user wallpaper filename is invalid")
	}

	directory, err := storage.WallpaperDirectory()
	if err != nil {
		return nil, err
	}
	originalPath := filepath.Join(directory, filename)
	stagedID, err := randomWallpaperID()
	if err != nil {
		return nil, err
	}
	stagedPath := filepath.Join(directory, "."+stagedID+".deleting")
	fileStaged := false
	if err := os.Rename(originalPath, stagedPath); err == nil {
		fileStaged = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stage user wallpaper deletion: %w", err)
	}
	restoreFile := func() {
		if fileStaged {
			_ = os.Rename(stagedPath, originalPath)
		}
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		restoreFile()
		return nil, fmt.Errorf("begin user wallpaper deletion: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM user_wallpapers WHERE id = ?`, id)
	if err != nil {
		restoreFile()
		return nil, fmt.Errorf("delete user wallpaper: %w", err)
	}
	if err := requireSingleMutation(result, "delete user wallpaper", "user wallpaper", false); err != nil {
		restoreFile()
		return nil, err
	}
	var selected string
	selectionErr := tx.QueryRowContext(
		ctx,
		`SELECT value FROM app_state WHERE key = ?`,
		selectedWallpaperStateKey,
	).Scan(&selected)
	if selectionErr != nil && !errors.Is(selectionErr, sql.ErrNoRows) {
		restoreFile()
		return nil, fmt.Errorf("read selected wallpaper: %w", selectionErr)
	}
	if selected == "custom:"+id {
		if err := upsertAppStateValue(ctx, tx, selectedWallpaperStateKey, defaultWallpaperSelection); err != nil {
			restoreFile()
			return nil, fmt.Errorf("reset selected wallpaper: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		restoreFile()
		return nil, fmt.Errorf("commit user wallpaper deletion: %w", err)
	}
	if fileStaged {
		_ = os.Remove(stagedPath)
	}

	return a.GetWallpaperSettingsContext(ctx)
}

// NewUserWallpaperHandler serves only generated wallpaper filenames from the
// application-owned directory. It never accepts arbitrary filesystem paths.
func NewUserWallpaperHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.URL.Path, userWallpaperRoutePrefix) {
			http.NotFound(w, r)
			return
		}
		filename := strings.TrimPrefix(r.URL.Path, userWallpaperRoutePrefix)
		if !userWallpaperFilenamePattern.MatchString(filename) {
			http.NotFound(w, r)
			return
		}
		directory, err := storage.WallpaperDirectory()
		if err != nil {
			http.Error(w, "wallpaper storage unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		http.ServeFile(w, r, filepath.Join(directory, filename))
	})
}
