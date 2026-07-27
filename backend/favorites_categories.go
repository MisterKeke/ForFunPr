package backend

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	favoriteSourceTelegram         = "telegram"
	favoriteSourceYouTube          = "youtube"
	favoriteCategoriesChangedEvent = "favorite-categories:changed"
)

type FavoriteCategory struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	Color     string `json:"color,omitempty"`
	CreatedAt string `json:"created_at"`
}

func (a *Service) EmitFavoriteCategoriesChanged(source string) {
	ctx := a.requestContext()
	if ctx.Value("events") != nil {
		runtime.EventsEmit(ctx, favoriteCategoriesChangedEvent, source)
	}
}

type FavoriteChannel struct {
	Username   string `json:"username,omitempty"`
	ChannelID  string `json:"channel_id,omitempty"`
	CategoryID *int   `json:"category_id,omitempty"`
}

func normalizeCategoryName(name string) string {
	return strings.TrimSpace(name)
}

func normalizeFavoriteSource(source string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	switch source {
	case favoriteSourceTelegram, favoriteSourceYouTube:
		return source
	default:
		return ""
	}
}

func NormalizeFavoriteSource(source string) (string, error) {
	normalized := normalizeFavoriteSource(source)
	if normalized == "" {
		return "", &ValidationError{Field: "source", Message: "source must be telegram or youtube"}
	}
	return normalized, nil
}

func normalizedCategoryKey(source, name string) string {
	return normalizeFavoriteSource(source) + ":" + strings.ToLower(normalizeCategoryName(name))
}

func (a *Service) ListFavoriteCategories(source string) ([]FavoriteCategory, error) {
	source = normalizeFavoriteSource(source)
	if source == "" {
		return nil, &ValidationError{Field: "source", Message: "source must be telegram or youtube"}
	}
	rows, err := a.db.Query(`
		SELECT id, name, source, COALESCE(color, ''), created_at
		FROM favorite_categories
		WHERE source = ?
		ORDER BY LOWER(name) ASC
	`, source)
	if err != nil {
		return []FavoriteCategory{}, err
	}
	defer rows.Close()

	categories := []FavoriteCategory{}
	for rows.Next() {
		var category FavoriteCategory
		if err := rows.Scan(&category.ID, &category.Name, &category.Source, &category.Color, &category.CreatedAt); err != nil {
			return []FavoriteCategory{}, err
		}
		categories = append(categories, category)
	}

	return categories, rows.Err()
}

func (a *Service) CreateFavoriteCategory(name string, source string) (FavoriteCategory, error) {
	category, _, err := a.CreateFavoriteCategoryWithStatus(name, source)
	return category, err
}

func (a *Service) CreateFavoriteCategoryWithStatus(name string, source string) (FavoriteCategory, bool, error) {
	name = normalizeCategoryName(name)
	if name == "" {
		return FavoriteCategory{}, false, &ValidationError{Field: "name", Message: "category name cannot be empty"}
	}
	source = normalizeFavoriteSource(source)
	if source == "" {
		return FavoriteCategory{}, false, &ValidationError{Field: "source", Message: "source must be telegram or youtube"}
	}
	nameNormalized := normalizedCategoryKey(source, name)

	var category FavoriteCategory
	err := a.db.QueryRow(`
		INSERT INTO favorite_categories (name, name_normalized, source)
		VALUES (?, ?, ?)
		ON CONFLICT(name_normalized) DO NOTHING
		RETURNING id, name, source, COALESCE(color, ''), created_at
	`, name, nameNormalized, source).Scan(&category.ID, &category.Name, &category.Source, &category.Color, &category.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		err = a.db.QueryRow(`
			SELECT id, name, source, COALESCE(color, ''), created_at
			FROM favorite_categories
			WHERE name_normalized = ?
		`, nameNormalized).Scan(&category.ID, &category.Name, &category.Source, &category.Color, &category.CreatedAt)
		if err != nil {
			return FavoriteCategory{}, false, fmt.Errorf("load existing favorite category: %w", err)
		}
		return category, false, nil
	}
	if err != nil {
		return FavoriteCategory{}, false, fmt.Errorf("create favorite category: %w", err)
	}

	return category, true, nil
}

func (a *Service) RenameFavoriteCategory(id int, name string) (FavoriteCategory, error) {
	if id <= 0 {
		return FavoriteCategory{}, &ValidationError{
			Field: "id", Message: "category ID must be a positive integer",
		}
	}
	name = normalizeCategoryName(name)
	if name == "" {
		return FavoriteCategory{}, &ValidationError{
			Field: "name", Message: "category name cannot be empty",
		}
	}

	tx, err := a.db.Begin()
	if err != nil {
		return FavoriteCategory{}, fmt.Errorf("begin favorite category rename: %w", err)
	}
	defer tx.Rollback()

	var source string
	if err := tx.QueryRow(
		`SELECT source FROM favorite_categories WHERE id = ?`,
		id,
	).Scan(&source); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return FavoriteCategory{}, &NotFoundError{
				Resource: "favorite category", Key: fmt.Sprint(id),
			}
		}
		return FavoriteCategory{}, fmt.Errorf("load favorite category for rename: %w", err)
	}

	nameNormalized := normalizedCategoryKey(source, name)
	var conflictingID int
	err = tx.QueryRow(`
		SELECT id
		FROM favorite_categories
		WHERE name_normalized = ? AND id <> ?
	`, nameNormalized, id).Scan(&conflictingID)
	if err == nil {
		return FavoriteCategory{}, &ConflictError{
			Resource: "favorite category",
			Message:  "a favorite category with that name already exists for this source",
		}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return FavoriteCategory{}, fmt.Errorf("check favorite category rename conflict: %w", err)
	}

	var category FavoriteCategory
	err = tx.QueryRow(`
		UPDATE favorite_categories
		SET name = ?, name_normalized = ?
		WHERE id = ?
		RETURNING id, name, source, COALESCE(color, ''), created_at
	`, name, nameNormalized, id).Scan(
		&category.ID,
		&category.Name,
		&category.Source,
		&category.Color,
		&category.CreatedAt,
	)
	if err != nil {
		return FavoriteCategory{}, fmt.Errorf("rename favorite category: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return FavoriteCategory{}, fmt.Errorf("commit favorite category rename: %w", err)
	}
	return category, nil
}

func ensureFavoriteCategoryExists(ctx context.Context, store appStateStore, categoryID int, source string) error {
	source = normalizeFavoriteSource(source)
	if source == "" {
		return &ValidationError{Field: "source", Message: "source must be telegram or youtube"}
	}
	var id int
	err := store.QueryRowContext(ctx, `
		SELECT id
		FROM favorite_categories
		WHERE id = ? AND source = ?
	`, categoryID, source).Scan(&id)

	if err == sql.ErrNoRows {
		return &NotFoundError{Resource: "favorite category", Key: fmt.Sprint(categoryID)}
	}
	if err != nil {
		return fmt.Errorf("find favorite category: %w", err)
	}

	return nil
}
