package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maximumDesktopAppNameLength = 120

type DesktopApp struct {
	ID             int    `json:"id"`
	DisplayName    string `json:"display_name"`
	ExecutablePath string `json:"executable_path"`
	IconURL        string `json:"icon_url,omitempty"`
	Available      bool   `json:"available"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func (a *Service) ListDesktopAppsContext(ctx context.Context) ([]DesktopApp, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT apps.id, apps.display_name, apps.executable_path,
		       icons.filename, apps.created_at, apps.updated_at
		FROM desktop_apps AS apps
		LEFT JOIN desktop_app_icons AS icons ON icons.app_id = apps.id
		ORDER BY apps.display_name COLLATE NOCASE ASC, apps.id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list desktop apps: %w", err)
	}
	defer rows.Close()

	apps := []DesktopApp{}
	for rows.Next() {
		var app DesktopApp
		var iconFilename sql.NullString
		if err := rows.Scan(
			&app.ID,
			&app.DisplayName,
			&app.ExecutablePath,
			&iconFilename,
			&app.CreatedAt,
			&app.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan desktop app: %w", err)
		}
		if iconFilename.Valid {
			app.IconURL = desktopAppIconURL(iconFilename.String)
		}
		app.Available = desktopExecutableAvailable(app.ExecutablePath)
		apps = append(apps, app)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate desktop apps: %w", err)
	}
	return apps, nil
}

func (a *Service) AddDesktopAppFromPathContext(ctx context.Context, selectedPath string) (DesktopApp, error) {
	executablePath, err := normalizeDesktopExecutablePath(selectedPath)
	if err != nil {
		return DesktopApp{}, err
	}
	displayName, err := normalizeDesktopAppName(strings.TrimSuffix(
		filepath.Base(executablePath),
		filepath.Ext(executablePath),
	))
	if err != nil {
		return DesktopApp{}, err
	}
	if err := ensureDesktopAppPathAvailable(ctx, a.db, executablePath, 0); err != nil {
		return DesktopApp{}, err
	}

	result, err := a.db.ExecContext(ctx, `
		INSERT INTO desktop_apps (display_name, executable_path)
		VALUES (?, ?)
	`, displayName, executablePath)
	if err != nil {
		return DesktopApp{}, fmt.Errorf("save desktop app: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return DesktopApp{}, fmt.Errorf("read desktop app ID: %w", err)
	}
	return loadDesktopAppContext(ctx, a.db, int(id))
}

func (a *Service) RenameDesktopAppContext(ctx context.Context, id int, displayName string) (DesktopApp, error) {
	if err := ValidateDesktopAppID(id); err != nil {
		return DesktopApp{}, err
	}
	name, err := normalizeDesktopAppName(displayName)
	if err != nil {
		return DesktopApp{}, err
	}
	result, err := a.db.ExecContext(ctx, `
		UPDATE desktop_apps
		SET display_name = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, name, id)
	if err != nil {
		return DesktopApp{}, fmt.Errorf("rename desktop app: %w", err)
	}
	if err := requireSingleMutation(result, "rename desktop app", "desktop app", false); err != nil {
		return DesktopApp{}, err
	}
	return loadDesktopAppContext(ctx, a.db, id)
}

func (a *Service) RelocateDesktopAppContext(ctx context.Context, id int, selectedPath string) (DesktopApp, error) {
	if err := ValidateDesktopAppID(id); err != nil {
		return DesktopApp{}, err
	}
	executablePath, err := normalizeDesktopExecutablePath(selectedPath)
	if err != nil {
		return DesktopApp{}, err
	}
	if err := ensureDesktopAppPathAvailable(ctx, a.db, executablePath, id); err != nil {
		return DesktopApp{}, err
	}
	result, err := a.db.ExecContext(ctx, `
		UPDATE desktop_apps
		SET executable_path = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, executablePath, id)
	if err != nil {
		return DesktopApp{}, fmt.Errorf("update desktop app executable: %w", err)
	}
	if err := requireSingleMutation(result, "update desktop app executable", "desktop app", false); err != nil {
		return DesktopApp{}, err
	}
	return loadDesktopAppContext(ctx, a.db, id)
}

func (a *Service) DeleteDesktopAppContext(ctx context.Context, id int) error {
	return a.deleteDesktopAppAndIconContext(ctx, id)
}

func (a *Service) DesktopAppExecutableContext(ctx context.Context, id int) (string, error) {
	app, err := loadDesktopAppContext(ctx, a.db, id)
	if err != nil {
		return "", err
	}
	executablePath, err := normalizeDesktopExecutablePath(app.ExecutablePath)
	if err != nil {
		return "", &ValidationError{
			Field:   "executable",
			Message: "That application is no longer available. Locate its executable and try again.",
		}
	}
	return executablePath, nil
}

func loadDesktopAppContext(ctx context.Context, store appStateStore, id int) (DesktopApp, error) {
	if err := ValidateDesktopAppID(id); err != nil {
		return DesktopApp{}, err
	}
	var app DesktopApp
	var iconFilename sql.NullString
	err := store.QueryRowContext(ctx, `
		SELECT apps.id, apps.display_name, apps.executable_path,
		       icons.filename, apps.created_at, apps.updated_at
		FROM desktop_apps AS apps
		LEFT JOIN desktop_app_icons AS icons ON icons.app_id = apps.id
		WHERE apps.id = ?
	`, id).Scan(
		&app.ID,
		&app.DisplayName,
		&app.ExecutablePath,
		&iconFilename,
		&app.CreatedAt,
		&app.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DesktopApp{}, &NotFoundError{Resource: "desktop app", Key: fmt.Sprint(id)}
		}
		return DesktopApp{}, fmt.Errorf("load desktop app: %w", err)
	}
	if iconFilename.Valid {
		app.IconURL = desktopAppIconURL(iconFilename.String)
	}
	app.Available = desktopExecutableAvailable(app.ExecutablePath)
	return app, nil
}

func ensureDesktopAppPathAvailable(ctx context.Context, store appStateStore, path string, excludedID int) error {
	var existingID int
	err := store.QueryRowContext(ctx, `
		SELECT id
		FROM desktop_apps
		WHERE executable_path = ? COLLATE NOCASE AND id != ?
	`, path, excludedID).Scan(&existingID)
	if err == nil {
		return &ConflictError{
			Resource: "desktop app",
			Message:  "That application has already been added.",
		}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check desktop app executable: %w", err)
	}
	return nil
}

// GetDesktopAppContext returns one saved application for internal adapters.
// Automation-facing adapters must redact ExecutablePath from their responses.
func (a *Service) GetDesktopAppContext(ctx context.Context, id int) (DesktopApp, error) {
	return loadDesktopAppContext(ctx, a.db, id)
}

func ValidateDesktopAppID(id int) error {
	if id <= 0 {
		return &ValidationError{Field: "id", Message: "Choose a valid application."}
	}
	return nil
}

func normalizeDesktopAppName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", &ValidationError{Field: "name", Message: "Application name cannot be empty."}
	}
	if utf8.RuneCountInString(name) > maximumDesktopAppNameLength {
		return "", &ValidationError{Field: "name", Message: "Application name must be 120 characters or fewer."}
	}
	return name, nil
}

func normalizeDesktopExecutablePath(value string) (string, error) {
	path := filepath.Clean(strings.TrimSpace(value))
	if path == "." || !filepath.IsAbs(path) {
		return "", &ValidationError{Field: "executable", Message: "Choose an application executable."}
	}
	if !strings.EqualFold(filepath.Ext(path), ".exe") {
		return "", &ValidationError{Field: "executable", Message: "Choose a Windows .exe file."}
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", &ValidationError{Field: "executable", Message: "That application executable no longer exists."}
		}
		if errors.Is(err, os.ErrPermission) {
			return "", &ValidationError{Field: "executable", Message: "That application executable cannot be accessed."}
		}
		return "", fmt.Errorf("inspect application executable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", &ValidationError{Field: "executable", Message: "Choose a regular .exe file."}
	}
	return path, nil
}

func desktopExecutableAvailable(path string) bool {
	_, err := normalizeDesktopExecutablePath(path)
	return err == nil
}
