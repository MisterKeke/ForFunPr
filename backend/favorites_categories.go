package backend

import (
	"database/sql"
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

func (a *App) ensureFavoriteCategoryNameNormalized() error {
	rows, err := a.db.Query(`PRAGMA table_info(favorite_categories)`)
	if err != nil {
		return err
	}

	hasColumn := false
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int

		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}

		if name == "name_normalized" {
			hasColumn = true
			break
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !hasColumn {
		if _, err := a.db.Exec(`ALTER TABLE favorite_categories ADD COLUMN name_normalized TEXT`); err != nil {
			return err
		}
	}

	usedKeys := map[string]bool{}
	normalizedRows, err := a.db.Query(`
		SELECT name_normalized
		FROM favorite_categories
		WHERE name_normalized IS NOT NULL AND name_normalized != ''
	`)
	if err != nil {
		return err
	}
	for normalizedRows.Next() {
		var nameNormalized string
		if err := normalizedRows.Scan(&nameNormalized); err != nil {
			normalizedRows.Close()
			return err
		}
		usedKeys[nameNormalized] = true
	}
	if err := normalizedRows.Close(); err != nil {
		return err
	}
	if err := normalizedRows.Err(); err != nil {
		return err
	}

	type categoryNormalization struct {
		id     int
		name   string
		source string
	}

	categoryRows, err := a.db.Query(`
		SELECT id, name, COALESCE(source, ?)
		FROM favorite_categories
		WHERE name_normalized IS NULL OR name_normalized = ''
	`, favoriteSourceTelegram)
	if err != nil {
		return err
	}

	categories := []categoryNormalization{}
	for categoryRows.Next() {
		var category categoryNormalization
		if err := categoryRows.Scan(&category.id, &category.name, &category.source); err != nil {
			categoryRows.Close()
			return err
		}
		categories = append(categories, category)
	}
	if err := categoryRows.Close(); err != nil {
		return err
	}
	if err := categoryRows.Err(); err != nil {
		return err
	}

	for _, category := range categories {
		nameNormalized := normalizedCategoryKey(category.source, category.name)
		if usedKeys[nameNormalized] {
			nameNormalized = fmt.Sprintf("%s:%d", nameNormalized, category.id)
		}
		usedKeys[nameNormalized] = true

		if _, err := a.db.Exec(`
			UPDATE favorite_categories
			SET name_normalized = ?
			WHERE id = ?
		`, nameNormalized, category.id); err != nil {
			return err
		}
	}

	hasUniqueName, err := a.favoriteCategoryNameHasUniqueConstraint()
	if err != nil {
		return err
	}
	if hasUniqueName {
		return a.rebuildFavoriteCategoriesTable()
	}

	_, err = a.db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_favorite_categories_name_normalized
		ON favorite_categories (name_normalized)
	`)
	return err
}

func (a *App) favoriteCategoryNameHasUniqueConstraint() (bool, error) {
	rows, err := a.db.Query(`PRAGMA index_list(favorite_categories)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var seq int
		var indexName string
		var unique int
		var origin string
		var partial int

		if err := rows.Scan(&seq, &indexName, &unique, &origin, &partial); err != nil {
			return false, err
		}
		if unique == 0 {
			continue
		}

		indexRows, err := a.db.Query(`PRAGMA index_info(` + quoteSQLiteIdentifier(indexName) + `)`)
		if err != nil {
			return false, err
		}

		columnCount := 0
		hasNameColumn := false
		for indexRows.Next() {
			var seqno int
			var cid int
			var columnName string
			if err := indexRows.Scan(&seqno, &cid, &columnName); err != nil {
				indexRows.Close()
				return false, err
			}
			columnCount++
			if columnName == "name" {
				hasNameColumn = true
			}
		}
		if err := indexRows.Close(); err != nil {
			return false, err
		}
		if err := indexRows.Err(); err != nil {
			return false, err
		}
		if columnCount == 1 && hasNameColumn {
			return true, nil
		}
	}

	return false, rows.Err()
}

func (a *App) rebuildFavoriteCategoriesTable() error {
	if _, err := a.db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer a.db.Exec(`PRAGMA foreign_keys = ON`)

	tx, err := a.db.Begin()
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	if _, err := tx.Exec(`
		CREATE TABLE favorite_categories_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			name_normalized TEXT NOT NULL UNIQUE,
			source TEXT NOT NULL DEFAULT 'telegram',
			color TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO favorite_categories_new (id, name, name_normalized, source, color, created_at)
		SELECT id, name, name_normalized, COALESCE(source, ?), color, created_at
		FROM favorite_categories
	`, favoriteSourceTelegram); err != nil {
		return err
	}

	if _, err := tx.Exec(`DROP TABLE favorite_categories`); err != nil {
		return err
	}

	if _, err := tx.Exec(`ALTER TABLE favorite_categories_new RENAME TO favorite_categories`); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func quoteSQLiteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
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

func (a *App) migrateFavoriteCategoriesBySource() error {
	rows, err := a.db.Query(`
		SELECT id, name, COALESCE(color, ''), created_at
		FROM favorite_categories
		WHERE source = ?
	`, favoriteSourceTelegram)
	if err != nil {
		return err
	}
	defer rows.Close()

	type categoryMigration struct {
		id        int
		name      string
		color     string
		createdAt string
	}

	categories := []categoryMigration{}
	for rows.Next() {
		var category categoryMigration
		if err := rows.Scan(&category.id, &category.name, &category.color, &category.createdAt); err != nil {
			return err
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, category := range categories {
		var telegramReferenced int
		err := a.db.QueryRow(`
			SELECT EXISTS(
				SELECT 1
				FROM telegram_favorites
				WHERE category_id = ?
			)
		`, category.id).Scan(&telegramReferenced)
		if err != nil {
			return err
		}

		var youtubeReferenced int
		err = a.db.QueryRow(`
			SELECT EXISTS(
				SELECT 1
				FROM youtube_favorites
				WHERE category_id = ?
			)
		`, category.id).Scan(&youtubeReferenced)
		if err != nil {
			return err
		}
		if youtubeReferenced == 0 {
			continue
		}

		if telegramReferenced == 0 {
			if _, err := a.db.Exec(`
				UPDATE favorite_categories
				SET source = ?, name_normalized = ?
				WHERE id = ?
			`, favoriteSourceYouTube, normalizedCategoryKey(favoriteSourceYouTube, category.name), category.id); err != nil {
				youtubeID, copyErr := a.ensureSourceCategoryCopy(category, favoriteSourceYouTube)
				if copyErr != nil {
					return copyErr
				}
				if _, updateErr := a.db.Exec(`
					UPDATE youtube_favorites
					SET category_id = ?
					WHERE category_id = ?
				`, youtubeID, category.id); updateErr != nil {
					return updateErr
				}
			}
			continue
		}

		youtubeID, err := a.ensureSourceCategoryCopy(category, favoriteSourceYouTube)
		if err != nil {
			return err
		}

		if _, err := a.db.Exec(`
			UPDATE youtube_favorites
			SET category_id = ?
			WHERE category_id = ?
		`, youtubeID, category.id); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) ensureSourceCategoryCopy(category struct {
	id        int
	name      string
	color     string
	createdAt string
}, source string) (int, error) {
	source = normalizeFavoriteSource(source)
	var existingID int
	err := a.db.QueryRow(`
		SELECT id
		FROM favorite_categories
		WHERE source = ? AND LOWER(name) = LOWER(?)
	`, source, category.name).Scan(&existingID)
	if err == nil {
		return existingID, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	result, err := a.db.Exec(`
		INSERT INTO favorite_categories (name, name_normalized, source, color, created_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?)
	`, category.name, normalizedCategoryKey(source, category.name), source, category.color, category.createdAt)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}
