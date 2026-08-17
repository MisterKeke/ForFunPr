package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maximumSetupNameLength        = 120
	maximumSetupDescriptionLength = 300
)

type Setup struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AppIDs      []int  `json:"app_ids"`
	IconURL     string `json:"icon_url,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type SetupCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AppIDs      []int  `json:"app_ids"`
	IconDataURL string `json:"icon_data_url,omitempty"`
	RemoveIcon  bool   `json:"remove_icon,omitempty"`
}

type SetupUpdateRequest struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AppIDs      []int  `json:"app_ids"`
	IconDataURL string `json:"icon_data_url,omitempty"`
	RemoveIcon  bool   `json:"remove_icon,omitempty"`
}

type SetupLaunchFailure struct {
	AppID   int    `json:"app_id"`
	AppName string `json:"app_name"`
	Error   string `json:"error"`
}

type SetupStartResult struct {
	Attempted int                  `json:"attempted"`
	Launched  int                  `json:"launched"`
	Failures  []SetupLaunchFailure `json:"failures"`
}

type setupReadStore interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (a *Service) ListSetupsContext(ctx context.Context) ([]Setup, error) {
	return listSetupsContext(ctx, a.db, 0)
}

func (a *Service) GetSetupContext(ctx context.Context, id int) (Setup, error) {
	if err := ValidateSetupID(id); err != nil {
		return Setup{}, err
	}
	setups, err := listSetupsContext(ctx, a.db, id)
	if err != nil {
		return Setup{}, err
	}
	if len(setups) == 0 {
		return Setup{}, &NotFoundError{Resource: "setup", Key: fmt.Sprint(id)}
	}
	return setups[0], nil
}

func (a *Service) CreateSetupContext(
	ctx context.Context,
	request SetupCreateRequest,
) (Setup, error) {
	name, description, appIDs, err := normalizeSetupMutation(
		request.Name,
		request.Description,
		request.AppIDs,
		request.IconDataURL,
		request.RemoveIcon,
	)
	if err != nil {
		return Setup{}, err
	}
	icon, err := prepareSetupIcon(request.IconDataURL)
	if err != nil {
		return Setup{}, err
	}
	defer icon.cleanup()

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return Setup{}, fmt.Errorf("begin setup creation: %w", err)
	}
	defer tx.Rollback()

	if err := ensureSetupNameAvailable(ctx, tx, name, 0); err != nil {
		return Setup{}, err
	}
	if err := ensureSetupAppsExist(ctx, tx, appIDs); err != nil {
		return Setup{}, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO setups (name, description)
		VALUES (?, ?)
	`, name, description)
	if err != nil {
		return Setup{}, fmt.Errorf("create setup: %w", err)
	}
	setupID, err := result.LastInsertId()
	if err != nil {
		return Setup{}, fmt.Errorf("read setup ID: %w", err)
	}
	if err := replaceSetupApps(ctx, tx, int(setupID), appIDs); err != nil {
		return Setup{}, err
	}
	if icon != nil {
		if err := icon.install(); err != nil {
			return Setup{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO setup_icons (setup_id, filename, mime_type, byte_size)
			VALUES (?, ?, ?, ?)
		`, setupID, icon.filename, icon.mimeType, icon.byteSize); err != nil {
			return Setup{}, fmt.Errorf("save setup icon metadata: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Setup{}, fmt.Errorf("commit setup creation: %w", err)
	}
	icon.keep()
	return a.GetSetupContext(ctx, int(setupID))
}

func (a *Service) UpdateSetupContext(
	ctx context.Context,
	request SetupUpdateRequest,
) (Setup, error) {
	if err := ValidateSetupID(request.ID); err != nil {
		return Setup{}, err
	}
	name, description, appIDs, err := normalizeSetupMutation(
		request.Name,
		request.Description,
		request.AppIDs,
		request.IconDataURL,
		request.RemoveIcon,
	)
	if err != nil {
		return Setup{}, err
	}
	icon, err := prepareSetupIcon(request.IconDataURL)
	if err != nil {
		return Setup{}, err
	}
	defer icon.cleanup()

	previousIcon, err := loadSetupIconFilenameContext(ctx, a.db, request.ID)
	if err != nil {
		return Setup{}, err
	}
	var staged *stagedSetupIcon
	if request.RemoveIcon && previousIcon != "" {
		staged, err = stageSetupIconDeletion(previousIcon)
		if err != nil {
			return Setup{}, err
		}
	}
	restoreStaged := true
	defer func() {
		if restoreStaged {
			staged.restore()
		}
	}()

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return Setup{}, fmt.Errorf("begin setup update: %w", err)
	}
	defer tx.Rollback()
	if err := ensureSetupNameAvailable(ctx, tx, name, request.ID); err != nil {
		return Setup{}, err
	}
	if err := ensureSetupAppsExist(ctx, tx, appIDs); err != nil {
		return Setup{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE setups
		SET name = ?, description = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, name, description, request.ID)
	if err != nil {
		return Setup{}, fmt.Errorf("update setup: %w", err)
	}
	if err := requireSingleMutation(result, "update setup", "setup", false); err != nil {
		return Setup{}, err
	}
	if err := replaceSetupApps(ctx, tx, request.ID, appIDs); err != nil {
		return Setup{}, err
	}

	if icon != nil {
		if err := icon.install(); err != nil {
			return Setup{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO setup_icons (setup_id, filename, mime_type, byte_size)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(setup_id) DO UPDATE SET
				filename = excluded.filename,
				mime_type = excluded.mime_type,
				byte_size = excluded.byte_size,
				updated_at = CURRENT_TIMESTAMP
		`, request.ID, icon.filename, icon.mimeType, icon.byteSize); err != nil {
			return Setup{}, fmt.Errorf("replace setup icon metadata: %w", err)
		}
	} else if request.RemoveIcon {
		if _, err := tx.ExecContext(ctx, `DELETE FROM setup_icons WHERE setup_id = ?`, request.ID); err != nil {
			return Setup{}, fmt.Errorf("delete setup icon metadata: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Setup{}, fmt.Errorf("commit setup update: %w", err)
	}
	icon.keep()
	restoreStaged = false
	staged.finish()
	if icon != nil && previousIcon != "" && previousIcon != icon.filename {
		removeSetupIconFile(previousIcon)
	}
	return a.GetSetupContext(ctx, request.ID)
}

func (a *Service) DeleteSetupContext(ctx context.Context, id int) error {
	if err := ValidateSetupID(id); err != nil {
		return err
	}
	filename, err := loadSetupIconFilenameContext(ctx, a.db, id)
	if err != nil {
		return err
	}
	staged, err := stageSetupIconDeletion(filename)
	if err != nil {
		return err
	}
	restoreStaged := true
	defer func() {
		if restoreStaged {
			staged.restore()
		}
	}()

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin setup deletion: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM setups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete setup: %w", err)
	}
	if err := requireSingleMutation(result, "delete setup", "setup", false); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit setup deletion: %w", err)
	}
	restoreStaged = false
	staged.finish()
	return nil
}

func listSetupsContext(ctx context.Context, store setupReadStore, id int) ([]Setup, error) {
	query := `
		SELECT setups.id, setups.name, setups.description,
		       setup_icons.filename, setups.created_at, setups.updated_at,
		       setup_apps.app_id
		FROM setups
		LEFT JOIN setup_icons ON setup_icons.setup_id = setups.id
		LEFT JOIN setup_apps ON setup_apps.setup_id = setups.id
	`
	args := []any{}
	if id > 0 {
		query += ` WHERE setups.id = ?`
		args = append(args, id)
	}
	query += ` ORDER BY setups.created_at, setups.id, setup_apps.position`

	rows, err := store.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list setups: %w", err)
	}
	defer rows.Close()

	setups := []Setup{}
	indexes := map[int]int{}
	for rows.Next() {
		var (
			setup        Setup
			iconFilename sql.NullString
			appID        sql.NullInt64
		)
		if err := rows.Scan(
			&setup.ID,
			&setup.Name,
			&setup.Description,
			&iconFilename,
			&setup.CreatedAt,
			&setup.UpdatedAt,
			&appID,
		); err != nil {
			return nil, fmt.Errorf("scan setup: %w", err)
		}
		index, exists := indexes[setup.ID]
		if !exists {
			setup.AppIDs = []int{}
			if iconFilename.Valid {
				setup.IconURL = setupIconURL(iconFilename.String)
			}
			setups = append(setups, setup)
			index = len(setups) - 1
			indexes[setup.ID] = index
		}
		if appID.Valid {
			setups[index].AppIDs = append(setups[index].AppIDs, int(appID.Int64))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate setups: %w", err)
	}
	return setups, nil
}

func normalizeSetupMutation(
	name string,
	description string,
	appIDs []int,
	iconDataURL string,
	removeIcon bool,
) (string, string, []int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", nil, &ValidationError{Field: "name", Message: "Enter a setup name."}
	}
	if utf8.RuneCountInString(name) > maximumSetupNameLength {
		return "", "", nil, &ValidationError{Field: "name", Message: "Setup names must be 120 characters or fewer."}
	}
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > maximumSetupDescriptionLength {
		return "", "", nil, &ValidationError{Field: "description", Message: "Setup descriptions must be 300 characters or fewer."}
	}
	normalizedAppIDs, err := normalizeSetupAppIDs(appIDs)
	if err != nil {
		return "", "", nil, err
	}
	if removeIcon && strings.TrimSpace(iconDataURL) != "" {
		return "", "", nil, &ValidationError{Field: "icon", Message: "Choose either a new icon or reset the current icon."}
	}
	return name, description, normalizedAppIDs, nil
}

func normalizeSetupAppIDs(appIDs []int) ([]int, error) {
	if len(appIDs) == 0 {
		return nil, &ValidationError{Field: "apps", Message: "Choose at least one application."}
	}
	normalized := make([]int, 0, len(appIDs))
	seen := make(map[int]struct{}, len(appIDs))
	for _, id := range appIDs {
		if err := ValidateDesktopAppID(id); err != nil {
			return nil, err
		}
		if _, exists := seen[id]; exists {
			return nil, &ValidationError{Field: "apps", Message: "An application cannot appear in a setup more than once."}
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

func ensureSetupNameAvailable(ctx context.Context, store setupReadStore, name string, excludedID int) error {
	var existingID int
	err := store.QueryRowContext(ctx, `
		SELECT id
		FROM setups
		WHERE name = ? COLLATE NOCASE AND id != ?
	`, name, excludedID).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check setup name: %w", err)
	}
	return &ConflictError{Resource: "setup", Message: "A setup with that name already exists."}
}

func ensureSetupAppsExist(ctx context.Context, store setupReadStore, appIDs []int) error {
	placeholders := make([]string, len(appIDs))
	args := make([]any, len(appIDs))
	for index, id := range appIDs {
		placeholders[index] = "?"
		args[index] = id
	}
	rows, err := store.QueryContext(ctx, `
		SELECT id
		FROM desktop_apps
		WHERE id IN (`+strings.Join(placeholders, ",")+`)
	`, args...)
	if err != nil {
		return fmt.Errorf("check setup applications: %w", err)
	}
	defer rows.Close()
	found := make(map[int]struct{}, len(appIDs))
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan setup application: %w", err)
		}
		found[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate setup applications: %w", err)
	}
	for _, id := range appIDs {
		if _, exists := found[id]; !exists {
			return &ValidationError{Field: "apps", Message: fmt.Sprintf("Application %d is no longer available.", id)}
		}
	}
	return nil
}

func replaceSetupApps(ctx context.Context, tx *sql.Tx, setupID int, appIDs []int) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM setup_apps WHERE setup_id = ?`, setupID); err != nil {
		return fmt.Errorf("clear setup applications: %w", err)
	}
	for position, appID := range appIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO setup_apps (setup_id, app_id, position)
			VALUES (?, ?, ?)
		`, setupID, appID, position); err != nil {
			return fmt.Errorf("save setup application %d: %w", appID, err)
		}
	}
	return nil
}

func loadSetupIconFilenameContext(ctx context.Context, store setupReadStore, setupID int) (string, error) {
	var filename sql.NullString
	err := store.QueryRowContext(ctx, `
		SELECT setup_icons.filename
		FROM setups
		LEFT JOIN setup_icons ON setup_icons.setup_id = setups.id
		WHERE setups.id = ?
	`, setupID).Scan(&filename)
	if errors.Is(err, sql.ErrNoRows) {
		return "", &NotFoundError{Resource: "setup", Key: fmt.Sprint(setupID)}
	}
	if err != nil {
		return "", fmt.Errorf("load setup icon: %w", err)
	}
	if filename.Valid {
		return filename.String, nil
	}
	return "", nil
}

func ValidateSetupID(id int) error {
	if id <= 0 {
		return &ValidationError{Field: "id", Message: "Choose a valid setup."}
	}
	return nil
}
