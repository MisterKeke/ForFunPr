package backend

import (
	"context"
	"database/sql"
)

type App struct {
	ctx context.Context

	db *sql.DB
}

func NewApp() *App {
	return &App{}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	db, err := sql.Open("sqlite", "database.db")
	if err != nil {
		panic(err)
	}

	a.db = db

	_, err = a.db.Exec(`
        CREATE TABLE IF NOT EXISTS favorite_rates (
            base TEXT NOT NULL,
            quote TEXT NOT NULL,
            PRIMARY KEY (base, quote)
        )
    `)
	if err != nil {
		panic(err)
	}

	// Create todos table if not exists
	_, err = a.db.Exec(`
		CREATE TABLE IF NOT EXISTS todos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT,
			is_completed INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			due_date DATE,
			priority TEXT DEFAULT 'medium' CHECK(priority IN ('low','medium','high'))
		)
	`)
	if err != nil {
		panic(err)
	}

	// Create telegram_favorites table if not exists
	_, err = a.db.Exec(`
		CREATE TABLE IF NOT EXISTS telegram_favorites (
			username TEXT PRIMARY KEY,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		panic(err)
	}

	_, err = a.db.Exec(`
		CREATE TABLE IF NOT EXISTS youtube_favorites (
			channel_id TEXT PRIMARY KEY,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		panic(err)
	}

	// Create favorite_categories table if not exists
	_, err = a.db.Exec(`
		CREATE TABLE IF NOT EXISTS favorite_categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			name_normalized TEXT NOT NULL UNIQUE,
			color TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		panic(err)
	}

	if err := ensureColumn(a.db, "favorite_categories", "color", "color TEXT"); err != nil {
		panic(err)
	}

	if err := ensureColumn(a.db, "favorite_categories", "source", "source TEXT NOT NULL DEFAULT 'telegram'"); err != nil {
		panic(err)
	}

	if err := a.ensureFavoriteCategoryNameNormalized(); err != nil {
		panic(err)
	}

	// Add category_id to existing favorite tables if missing.
	if err := ensureColumn(a.db, "telegram_favorites", "category_id", "category_id INTEGER REFERENCES favorite_categories(id) ON DELETE SET NULL"); err != nil {
		panic(err)
	}

	if err := ensureColumn(a.db, "youtube_favorites", "category_id", "category_id INTEGER REFERENCES favorite_categories(id) ON DELETE SET NULL"); err != nil {
		panic(err)
	}

	if err := ensureColumn(a.db, "youtube_favorites", "username", "username TEXT"); err != nil {
		panic(err)
	}

	if err := a.migrateFavoriteCategoriesBySource(); err != nil {
		panic(err)
	}
}

func ensureColumn(db *sql.DB, tableName, columnName, definition string) error {
	rows, err := db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int

		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}

		if name == columnName {
			return nil
		}
	}

	_, err = db.Exec(`ALTER TABLE ` + tableName + ` ADD COLUMN ` + definition)
	return err
}
