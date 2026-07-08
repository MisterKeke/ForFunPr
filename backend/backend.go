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
}
