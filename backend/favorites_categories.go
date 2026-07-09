package backend

import (
	"fmt"
	"strings"
)

const (
	favoriteSourceTelegram = "telegram"
	favoriteSourceYouTube  = "youtube"
)

type FavoriteCategory struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	Color     string `json:"color,omitempty"`
	CreatedAt string `json:"created_at"`
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
	case favoriteSourceYouTube:
		return favoriteSourceYouTube
	default:
		return favoriteSourceTelegram
	}
}

func normalizedCategoryKey(source, name string) string {
	return normalizeFavoriteSource(source) + ":" + strings.ToLower(normalizeCategoryName(name))
}

func (a *App) ListFavoriteCategories(source string) ([]FavoriteCategory, error) {
	source = normalizeFavoriteSource(source)
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

func (a *App) CreateFavoriteCategory(name string, source string) (FavoriteCategory, error) {
	name = normalizeCategoryName(name)
	if name == "" {
		return FavoriteCategory{}, fmt.Errorf("category name cannot be empty")
	}
	source = normalizeFavoriteSource(source)
	nameNormalized := normalizedCategoryKey(source, name)

	var category FavoriteCategory
	err := a.db.QueryRow(`
		SELECT id, name, source, COALESCE(color, ''), created_at
		FROM favorite_categories
		WHERE source = ? AND LOWER(name) = LOWER(?)
	`, source, name).Scan(&category.ID, &category.Name, &category.Source, &category.Color, &category.CreatedAt)
	if err == nil {
		return category, nil
	}

	err = a.db.QueryRow(`
		INSERT INTO favorite_categories (name, name_normalized, source)
		VALUES (?, ?, ?)
		RETURNING id, name, source, COALESCE(color, ''), created_at
	`, name, nameNormalized, source).Scan(&category.ID, &category.Name, &category.Source, &category.Color, &category.CreatedAt)

	if err != nil {
		return FavoriteCategory{}, err
	}

	return category, nil
}

func (a *App) ensureFavoriteCategoryExists(categoryID int, source string) error {
	var id int
	err := a.db.QueryRow(`
		SELECT id
		FROM favorite_categories
		WHERE id = ? AND source = ?
	`, categoryID, normalizeFavoriteSource(source)).Scan(&id)

	if err != nil {
		return fmt.Errorf("favorite category does not exist")
	}

	return nil
}
