package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

type migration struct {
	version int
	name    string
	up      func(context.Context, *sql.Tx) error
}

var migrations = []migration{
	{
		version: 1,
		name:    "create core currency and todo tables",
		up:      migrateCoreSchema,
	},
	{
		version: 2,
		name:    "create favorite categories and source favorites",
		up:      migrateFavoriteSchema,
	},
	{
		version: 3,
		name:    "create application state table",
		up:      migrateAppStateSchema,
	},
	{
		version: 4,
		name:    "create favorite update tracking tables",
		up:      migrateFavoriteUpdateSchema,
	},
	{
		version: 5,
		name:    "create saved weather location table",
		up:      migrateWeatherLocationSchema,
	},
	{
		version: 6,
		name:    "cache saved weather forecasts",
		up:      migrateWeatherForecastCacheSchema,
	},
	{
		version: 7,
		name:    "persist favorite news scan results",
		up:      migrateFavoriteNewsSchema,
	},
	{
		version: 8,
		name:    "add task difficulty tags and subtasks",
		up:      migrateTaskMetadataSchema,
	},
	{
		version: 9,
		name:    "create user wallpaper metadata",
		up:      migrateUserWallpaperSchema,
	},
	{
		version: 10,
		name:    "create notes",
		up:      migrateNotesSchema,
	},
	{
		version: 11,
		name:    "create bookmarks and bookmark tags",
		up:      migrateBookmarksSchema,
	},
	{
		version: 12,
		name:    "create desktop app launchers",
		up:      migrateDesktopAppSchema,
	},
	{
		version: 13,
		name:    "create desktop app icon metadata",
		up:      migrateDesktopAppIconSchema,
	},
	{
		version: 14,
		name:    "create Steam game price tracker",
		up:      migrateSteamGameSchema,
	},
	{
		version: 15,
		name:    "create application setups",
		up:      migrateSetupSchema,
	},
	{
		version: 16,
		name:    "create note topics and relationships",
		up:      migrateNoteRelationshipSchema,
	},
	{
		version: 17,
		name:    "create clipboard history",
		up:      migrateClipboardSchema,
	},
	{
		version: 18,
		name:    "create calculator history",
		up:      migrateCalculatorSchema,
	},
	{
		version: 19,
		name:    "create screenshot library",
		up:      migrateScreenshotSchema,
	},
	{
		version: 20,
		name:    "create saved world clocks",
		up:      migrateWorldClockSchema,
	},
	{
		version: 21,
		name:    "create website search storage",
		up:      migrateWebsiteSearchSchema,
	},
	{
		version: 22,
		name:    "add P0 revisions, ordering, and recoverable screenshot OCR",
		up:      migrateP0CorrectnessSchema,
	},
}

func applyMigrations(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	applied, err := appliedMigrationVersions(ctx, db)
	if err != nil {
		return err
	}

	ordered := append([]migration(nil), migrations...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].version < ordered[j].version
	})

	for _, migration := range ordered {
		if applied[migration.version] {
			continue
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d (%s): %w",
				migration.version, migration.name, err)
		}

		if err := migration.up(ctx, tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d (%s): %w",
				migration.version, migration.name, err)
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO schema_migrations (version, name)
			VALUES (?, ?)
		`, migration.version, migration.name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", migration.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", migration.version, err)
		}
	}

	return nil
}

func appliedMigrationVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT version
		FROM schema_migrations
	`)
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}

	return applied, nil
}

func migrateCoreSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS favorite_rates (
			base TEXT NOT NULL,
			quote TEXT NOT NULL,
			PRIMARY KEY (base, quote)
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS todos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT,
			is_completed INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			due_date DATE,
			priority TEXT NOT NULL DEFAULT 'medium'
				CHECK(priority IN ('low', 'medium', 'high'))
		)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_todos_due_incomplete_priority_created_at
		ON todos (due_date, is_completed, priority, created_at)
		`,
	)
}

func migrateFavoriteSchema(ctx context.Context, tx *sql.Tx) error {
	if err := executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS favorite_categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			name_normalized TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT 'telegram',
			color TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS telegram_favorites (
			username TEXT PRIMARY KEY,
			added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			category_id INTEGER REFERENCES favorite_categories(id) ON DELETE SET NULL
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS youtube_favorites (
			channel_id TEXT PRIMARY KEY,
			added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			category_id INTEGER REFERENCES favorite_categories(id) ON DELETE SET NULL,
			username TEXT
		)
		`,
	); err != nil {
		return err
	}

	// These additions make the migration safe for databases created by earlier
	// versions that may have had tables but not all current columns.
	if err := addColumnIfMissing(ctx, tx, "favorite_categories", "source",
		"TEXT NOT NULL DEFAULT 'telegram'"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, tx, "favorite_categories", "color",
		"TEXT"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, tx, "favorite_categories", "name_normalized",
		"TEXT"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, tx, "telegram_favorites", "category_id",
		"INTEGER REFERENCES favorite_categories(id) ON DELETE SET NULL"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, tx, "youtube_favorites", "category_id",
		"INTEGER REFERENCES favorite_categories(id) ON DELETE SET NULL"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, tx, "youtube_favorites", "username",
		"TEXT"); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE favorite_categories
		SET source = CASE
			WHEN LOWER(TRIM(source)) = 'youtube' THEN 'youtube'
			ELSE 'telegram'
		END
	`); err != nil {
		return fmt.Errorf("normalize favorite category sources: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE favorite_categories
		SET name_normalized = LOWER(TRIM(source)) || ':' || LOWER(TRIM(name))
		WHERE name_normalized IS NULL OR TRIM(name_normalized) = ''
	`); err != nil {
		return fmt.Errorf("backfill normalized category names: %w", err)
	}

	if err := ensureUniqueCategoryKey(ctx, tx); err != nil {
		return err
	}

	return executeStatements(ctx, tx,
		`
		CREATE INDEX IF NOT EXISTS idx_favorite_categories_source_name
		ON favorite_categories (source, name_normalized)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_telegram_favorites_category
		ON telegram_favorites (category_id)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_youtube_favorites_category
		ON youtube_favorites (category_id)
		`,
	)
}

func migrateTaskMetadataSchema(ctx context.Context, tx *sql.Tx) error {
	if err := addColumnIfMissing(
		ctx,
		tx,
		"todos",
		"difficulty",
		"TEXT CHECK (difficulty IS NULL OR difficulty IN ('easy', 'medium', 'hard'))",
	); err != nil {
		return err
	}

	return executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			name_normalized TEXT NOT NULL UNIQUE
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS todo_tags (
			todo_id INTEGER NOT NULL REFERENCES todos(id) ON DELETE CASCADE,
			tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
			PRIMARY KEY (todo_id, tag_id)
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS todo_subtasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			todo_id INTEGER NOT NULL REFERENCES todos(id) ON DELETE CASCADE,
			title TEXT NOT NULL CHECK (TRIM(title) <> ''),
			is_completed INTEGER NOT NULL DEFAULT 0,
			position INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_todo_tags_tag
		ON todo_tags (tag_id)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_todo_subtasks_todo_position
		ON todo_subtasks (todo_id, position, id)
		`,
	)
}

func migrateNotesSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL DEFAULT '',
			body TEXT NOT NULL DEFAULT '',
			is_pinned INTEGER NOT NULL DEFAULT 0
				CHECK (is_pinned IN (0, 1)),
			is_archived INTEGER NOT NULL DEFAULT 0
				CHECK (is_archived IN (0, 1)),
			revision INTEGER NOT NULL DEFAULT 1
				CHECK (revision > 0),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CHECK (
				length(title) <= 200
				AND length(CAST(body AS BLOB)) <= 262144
			)
		)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_notes_archive_pin_updated
		ON notes (is_archived, is_pinned DESC, updated_at DESC, id DESC)
		`,
	)
}

func migrateNoteRelationshipSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS note_topics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL CHECK (
				TRIM(title) <> '' AND length(title) <= 120
			),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS note_topic_blocks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			topic_id INTEGER NOT NULL
				REFERENCES note_topics(id) ON DELETE CASCADE,
			note_id INTEGER NOT NULL
				REFERENCES notes(id) ON DELETE CASCADE,
			position_x REAL NOT NULL DEFAULT 40
				CHECK (position_x >= 0 AND position_x <= 100000),
			position_y REAL NOT NULL DEFAULT 40
				CHECK (position_y >= 0 AND position_y <= 100000),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (topic_id, note_id),
			UNIQUE (topic_id, id)
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS note_topic_connections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			topic_id INTEGER NOT NULL
				REFERENCES note_topics(id) ON DELETE CASCADE,
			from_block_id INTEGER NOT NULL,
			to_block_id INTEGER NOT NULL,
			relation_type TEXT NOT NULL DEFAULT 'leads_to'
				CHECK (relation_type IN ('leads_to', 'related')),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CHECK (from_block_id <> to_block_id),
			UNIQUE (topic_id, from_block_id, to_block_id, relation_type),
			FOREIGN KEY (topic_id, from_block_id)
				REFERENCES note_topic_blocks(topic_id, id) ON DELETE CASCADE,
			FOREIGN KEY (topic_id, to_block_id)
				REFERENCES note_topic_blocks(topic_id, id) ON DELETE CASCADE
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS note_todo_connections (
			note_id INTEGER NOT NULL
				REFERENCES notes(id) ON DELETE CASCADE,
			todo_id INTEGER NOT NULL
				REFERENCES todos(id) ON DELETE CASCADE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (note_id, todo_id)
		)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_note_topics_updated
		ON note_topics (updated_at DESC, id DESC)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_note_topic_blocks_note
		ON note_topic_blocks (note_id, topic_id)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_note_topic_connections_target
		ON note_topic_connections (topic_id, to_block_id)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_note_todo_connections_todo
		ON note_todo_connections (todo_id, note_id)
		`,
	)
}

func migrateBookmarksSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS bookmarks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			url_normalized TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			is_read INTEGER NOT NULL DEFAULT 0
				CHECK (is_read IN (0, 1)),
			read_at DATETIME,
			revision INTEGER NOT NULL DEFAULT 1
				CHECK (revision > 0),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CHECK (
				length(CAST(url AS BLOB)) <= 4096
				AND length(title) <= 200
				AND length(CAST(description AS BLOB)) <= 16384
			)
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS bookmark_tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			name_normalized TEXT NOT NULL UNIQUE
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS bookmark_tag_assignments (
			bookmark_id INTEGER NOT NULL
				REFERENCES bookmarks(id) ON DELETE CASCADE,
			tag_id INTEGER NOT NULL
				REFERENCES bookmark_tags(id) ON DELETE CASCADE,
			PRIMARY KEY (bookmark_id, tag_id)
		)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_bookmarks_read_created
		ON bookmarks (is_read, created_at DESC, id DESC)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_bookmark_tag_assignments_tag
		ON bookmark_tag_assignments (tag_id, bookmark_id)
		`,
	)
}

func migrateAppStateSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS app_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
		`,
	)
}

func migrateFavoriteUpdateSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS favorite_update_checkpoints (
			source TEXT NOT NULL,
			source_id TEXT NOT NULL,
			checked_through TEXT NOT NULL,
			last_success_at TEXT,
			last_attempted_at TEXT,
			last_error TEXT,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (source, source_id)
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS favorite_update_seen_items (
			source TEXT NOT NULL,
			source_id TEXT NOT NULL,
			item_id TEXT NOT NULL,
			published_at TEXT NOT NULL,
			first_seen_at TEXT NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (source, source_id, item_id)
		)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_favorite_update_seen_items_source_published
		ON favorite_update_seen_items (source, source_id, published_at)
		`,
	)
}

func migrateFavoriteNewsSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS favorite_news_items (
			source TEXT NOT NULL,
			source_id TEXT NOT NULL,
			item_id TEXT NOT NULL,
			published_at TEXT NOT NULL,
			discovered_at TEXT NOT NULL,
			payload_json TEXT NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (source, source_id, item_id)
		)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_favorite_news_items_discovered
		ON favorite_news_items (discovered_at, published_at)
		`,
		`
		CREATE TABLE IF NOT EXISTS favorite_news_state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			scan_started_at TEXT NOT NULL,
			errors_json TEXT NOT NULL DEFAULT '[]',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
		`,
	)
}

// migrateWeatherLocationSchema stores the most recently resolved browser
// location. The fixed primary key deliberately permits one saved local
// weather location per application profile.
func migrateWeatherLocationSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS location (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			latitude REAL NOT NULL CHECK (latitude >= -90 AND latitude <= 90),
			longitude REAL NOT NULL CHECK (longitude >= -180 AND longitude <= 180),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
		`,
	)
}

// migrateWeatherForecastCacheSchema adds the stale-while-revalidate payload
// separately so installations that already saved a location retain it while
// gaining an immediately renderable forecast.
func migrateWeatherForecastCacheSchema(ctx context.Context, tx *sql.Tx) error {
	if err := addColumnIfMissing(ctx, tx, "location", "forecast_json", "TEXT"); err != nil {
		return err
	}
	return addColumnIfMissing(ctx, tx, "location", "forecast_updated_at", "TEXT")
}

func executeStatements(ctx context.Context, tx *sql.Tx, statements ...string) error {
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func addColumnIfMissing(
	ctx context.Context,
	tx *sql.Tx,
	table string,
	column string,
	columnDefinition string,
) error {
	exists, err := tableHasColumn(ctx, tx, table, column)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	statement := fmt.Sprintf(
		"ALTER TABLE %s ADD COLUMN %s %s",
		quoteSQLiteIdentifier(table),
		quoteSQLiteIdentifier(column),
		columnDefinition,
	)

	if _, err := tx.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, column, err)
	}

	return nil
}

func tableHasColumn(
	ctx context.Context,
	tx *sql.Tx,
	table string,
	column string,
) (bool, error) {
	rows, err := tx.QueryContext(
		ctx,
		fmt.Sprintf("PRAGMA table_info(%s)", quoteSQLiteIdentifier(table)),
	)
	if err != nil {
		return false, fmt.Errorf("inspect table %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			columnID     int
			name         string
			columnType   string
			notNull      int
			defaultValue any
			primaryKey   int
		)

		if err := rows.Scan(
			&columnID,
			&name,
			&columnType,
			&notNull,
			&defaultValue,
			&primaryKey,
		); err != nil {
			return false, fmt.Errorf("scan table metadata for %s: %w", table, err)
		}

		if name == column {
			return true, nil
		}
	}

	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate table metadata for %s: %w", table, err)
	}

	return false, nil
}

func ensureUniqueCategoryKey(ctx context.Context, tx *sql.Tx) error {
	var duplicateCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM (
			SELECT name_normalized
			FROM favorite_categories
			GROUP BY name_normalized
			HAVING COUNT(*) > 1
		)
	`).Scan(&duplicateCount); err != nil {
		return fmt.Errorf("check duplicate category keys: %w", err)
	}

	if duplicateCount > 0 {
		return fmt.Errorf(
			"cannot migrate favorite categories: %d duplicate normalized name(s) require review",
			duplicateCount,
		)
	}

	hasUniqueIndex, err := hasUniqueSingleColumnIndex(
		ctx,
		tx,
		"favorite_categories",
		"name_normalized",
	)
	if err != nil {
		return err
	}
	if hasUniqueIndex {
		return nil
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS ux_favorite_categories_name_normalized
		ON favorite_categories (name_normalized)
	`); err != nil {
		return fmt.Errorf("create unique category key index: %w", err)
	}

	return nil
}

func migrateUserWallpaperSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS user_wallpapers (
			id TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			filename TEXT NOT NULL UNIQUE,
			mime_type TEXT NOT NULL
				CHECK(mime_type IN ('image/jpeg', 'image/png', 'image/webp')),
			byte_size INTEGER NOT NULL CHECK(byte_size > 0),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
		`,
	)
}

func migrateDesktopAppSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS desktop_apps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			display_name TEXT NOT NULL
				CHECK(length(trim(display_name)) BETWEEN 1 AND 120),
			executable_path TEXT NOT NULL COLLATE NOCASE UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_desktop_apps_display_name
		ON desktop_apps (display_name COLLATE NOCASE, id)
		`,
	)
}

func migrateDesktopAppIconSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS desktop_app_icons (
			app_id INTEGER PRIMARY KEY
				REFERENCES desktop_apps(id) ON DELETE CASCADE,
			filename TEXT NOT NULL UNIQUE,
			mime_type TEXT NOT NULL
				CHECK(mime_type IN ('image/jpeg', 'image/png', 'image/webp', 'image/x-icon')),
			byte_size INTEGER NOT NULL CHECK(byte_size > 0),
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
		`,
	)
}

func migrateSteamGameSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS steam_game_settings (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			country_code TEXT NOT NULL
				CHECK(length(country_code) = 2 AND country_code GLOB '[A-Z][A-Z]'),
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
		`,
		`
		INSERT OR IGNORE INTO steam_game_settings (id, country_code)
		VALUES (1, 'TR')
		`,
		`
		CREATE TABLE IF NOT EXISTS steam_games (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			steam_app_id INTEGER NOT NULL UNIQUE CHECK(steam_app_id > 0),
			store_url TEXT NOT NULL,
			name TEXT NOT NULL CHECK(length(trim(name)) BETWEEN 1 AND 300),
			image_source_url TEXT NOT NULL DEFAULT '',
			price_status TEXT NOT NULL
				CHECK(price_status IN ('priced', 'free', 'unavailable')),
			currency TEXT
				CHECK(currency IS NULL OR (length(currency) = 3 AND currency GLOB '[A-Z][A-Z][A-Z]')),
			regular_price_minor INTEGER
				CHECK(regular_price_minor IS NULL OR regular_price_minor >= 0),
			current_price_minor INTEGER
				CHECK(current_price_minor IS NULL OR current_price_minor >= 0),
			discount_percent INTEGER NOT NULL DEFAULT 0
				CHECK(discount_percent BETWEEN 0 AND 100),
			price_country_code TEXT NOT NULL
				CHECK(length(price_country_code) = 2 AND price_country_code GLOB '[A-Z][A-Z]'),
			last_checked_at DATETIME,
			last_attempted_at DATETIME,
			last_refresh_error TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_steam_games_name
		ON steam_games (name COLLATE NOCASE, id)
		`,
		`
		CREATE TABLE IF NOT EXISTS steam_game_images (
			game_id INTEGER PRIMARY KEY
				REFERENCES steam_games(id) ON DELETE CASCADE,
			filename TEXT NOT NULL UNIQUE,
			mime_type TEXT NOT NULL
				CHECK(mime_type IN ('image/jpeg', 'image/png', 'image/webp')),
			byte_size INTEGER NOT NULL CHECK(byte_size > 0),
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
		`,
	)
}

func migrateSetupSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`
		CREATE TABLE IF NOT EXISTS setups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL COLLATE NOCASE UNIQUE
				CHECK(length(trim(name)) BETWEEN 1 AND 120),
			description TEXT NOT NULL DEFAULT ''
				CHECK(length(description) <= 300),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS setup_apps (
			setup_id INTEGER NOT NULL
				REFERENCES setups(id) ON DELETE CASCADE,
			app_id INTEGER NOT NULL
				REFERENCES desktop_apps(id) ON DELETE CASCADE,
			position INTEGER NOT NULL CHECK(position >= 0),
			PRIMARY KEY (setup_id, app_id),
			UNIQUE (setup_id, position)
		)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_setup_apps_order
		ON setup_apps (setup_id, position)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_setup_apps_app
		ON setup_apps (app_id, setup_id)
		`,
		`
		CREATE TABLE IF NOT EXISTS setup_icons (
			setup_id INTEGER PRIMARY KEY
				REFERENCES setups(id) ON DELETE CASCADE,
			filename TEXT NOT NULL UNIQUE,
			mime_type TEXT NOT NULL
				CHECK(mime_type IN ('image/jpeg', 'image/png', 'image/webp', 'image/x-icon')),
			byte_size INTEGER NOT NULL CHECK(byte_size > 0),
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
		`,
	)
}

func migrateClipboardSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`CREATE TABLE IF NOT EXISTS clipboard_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			kind TEXT NOT NULL CHECK(kind IN ('text', 'url')),
			content TEXT NOT NULL CHECK(length(content) > 0),
			content_hash TEXT NOT NULL UNIQUE,
			byte_size INTEGER NOT NULL CHECK(byte_size > 0),
			first_copied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_copied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			copy_count INTEGER NOT NULL DEFAULT 1 CHECK(copy_count > 0),
			is_pinned INTEGER NOT NULL DEFAULT 0 CHECK(is_pinned IN (0, 1))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_clipboard_items_recent
			ON clipboard_items (last_copied_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_clipboard_items_pinned
			ON clipboard_items (is_pinned DESC, last_copied_at DESC)`,
		`CREATE TABLE IF NOT EXISTS clipboard_settings (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			collection_enabled INTEGER NOT NULL DEFAULT 0 CHECK(collection_enabled IN (0, 1)),
			retention_days INTEGER NOT NULL DEFAULT 7 CHECK(retention_days BETWEEN 1 AND 3650),
			maximum_items INTEGER NOT NULL DEFAULT 500 CHECK(maximum_items BETWEEN 10 AND 10000),
			maximum_text_bytes INTEGER NOT NULL DEFAULT 262144 CHECK(maximum_text_bytes BETWEEN 1024 AND 1048576)
		)`,
		`INSERT OR IGNORE INTO clipboard_settings (
			id, collection_enabled, retention_days, maximum_items, maximum_text_bytes
		) VALUES (1, 0, 7, 500, 262144)`,
	)
}

func migrateCalculatorSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`CREATE TABLE IF NOT EXISTS calculator_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mode TEXT NOT NULL CHECK(mode IN ('expression', 'unit', 'date')),
			input_text TEXT NOT NULL CHECK(length(input_text) BETWEEN 1 AND 1000),
			result_text TEXT NOT NULL CHECK(length(result_text) BETWEEN 1 AND 1000),
			is_approximate INTEGER NOT NULL DEFAULT 0 CHECK(is_approximate IN (0, 1)),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_calculator_history_recent
			ON calculator_history (created_at DESC, id DESC)`,
	)
}

func migrateScreenshotSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`CREATE TABLE IF NOT EXISTS screenshots (
			id TEXT PRIMARY KEY,
			original_filename TEXT NOT NULL UNIQUE,
			edited_filename TEXT,
			thumbnail_filename TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL DEFAULT '' CHECK(length(title) <= 200),
			capture_kind TEXT NOT NULL CHECK(capture_kind IN ('screen', 'window', 'region')),
			width INTEGER NOT NULL CHECK(width > 0),
			height INTEGER NOT NULL CHECK(height > 0),
			byte_size INTEGER NOT NULL CHECK(byte_size > 0),
			sha256 TEXT NOT NULL,
			ocr_text TEXT NOT NULL DEFAULT '',
			ocr_language TEXT NOT NULL DEFAULT '',
			ocr_status TEXT NOT NULL DEFAULT 'not_started'
				CHECK(ocr_status IN ('not_started', 'processing', 'complete', 'failed', 'unsupported')),
			captured_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_screenshots_recent
			ON screenshots (captured_at DESC, id DESC)`,
	)
}

// migrateP0CorrectnessSchema is deliberately additive. In particular, OCR
// lifecycle data is stored separately because the original screenshots table
// has a CHECK constraint that predates the queued state. Keeping that table in
// place avoids a destructive SQLite table rebuild for existing libraries.
func migrateP0CorrectnessSchema(ctx context.Context, tx *sql.Tx) error {
	if err := addColumnIfMissing(ctx, tx, "todos", "revision", "INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0)"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, tx, "todos", "updated_at", "DATETIME"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE todos
		SET updated_at = COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)
		WHERE updated_at IS NULL
	`); err != nil {
		return fmt.Errorf("backfill todo update timestamps: %w", err)
	}
	if err := addColumnIfMissing(ctx, tx, "favorite_categories", "display_order", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, tx, "favorite_categories", "updated_at", "DATETIME"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE favorite_categories
		SET updated_at = COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)
		WHERE updated_at IS NULL
	`); err != nil {
		return fmt.Errorf("backfill favorite category update timestamps: %w", err)
	}
	if err := addColumnIfMissing(ctx, tx, "note_topics", "revision", "INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0)"); err != nil {
		return err
	}

	return executeStatements(ctx, tx,
		`CREATE INDEX IF NOT EXISTS idx_todos_updated_id
			ON todos (updated_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_todos_completion_due_id
			ON todos (is_completed, due_date, id)`,
		`CREATE INDEX IF NOT EXISTS idx_favorite_categories_source_order
			ON favorite_categories (source, display_order, id)`,
		`CREATE TABLE IF NOT EXISTS screenshot_ocr_jobs (
			screenshot_id TEXT PRIMARY KEY
				REFERENCES screenshots(id) ON DELETE CASCADE,
			status TEXT NOT NULL DEFAULT 'not_started'
				CHECK(status IN ('not_started', 'queued', 'processing', 'complete', 'failed', 'unsupported')),
			started_at DATETIME,
			completed_at DATETIME,
			failure_code TEXT NOT NULL DEFAULT '',
			failure_message TEXT NOT NULL DEFAULT '',
			attempt INTEGER NOT NULL DEFAULT 0 CHECK(attempt >= 0),
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`INSERT INTO screenshot_ocr_jobs (screenshot_id, status, started_at, completed_at, updated_at)
			SELECT id, ocr_status,
				CASE WHEN ocr_status IN ('processing', 'complete', 'failed') THEN updated_at END,
				CASE WHEN ocr_status IN ('complete', 'failed', 'unsupported') THEN updated_at END,
				updated_at
			FROM screenshots
			WHERE 1 = 1
			ON CONFLICT(screenshot_id) DO NOTHING`,
		`CREATE INDEX IF NOT EXISTS idx_screenshot_ocr_jobs_status_updated
			ON screenshot_ocr_jobs (status, updated_at, screenshot_id)`,
	)
}

func migrateWorldClockSchema(ctx context.Context, tx *sql.Tx) error {
	return executeStatements(ctx, tx,
		`CREATE TABLE IF NOT EXISTS world_clocks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label TEXT NOT NULL CHECK(length(trim(label)) BETWEEN 1 AND 80),
			time_zone_id TEXT NOT NULL CHECK(length(trim(time_zone_id)) BETWEEN 1 AND 100),
			sort_order INTEGER NOT NULL DEFAULT 0,
			working_day_start_minutes INTEGER NOT NULL DEFAULT 540
				CHECK(working_day_start_minutes BETWEEN 0 AND 1439),
			working_day_end_minutes INTEGER NOT NULL DEFAULT 1020
				CHECK(working_day_end_minutes BETWEEN 1 AND 1440),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CHECK(working_day_start_minutes < working_day_end_minutes)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_world_clocks_order
			ON world_clocks (sort_order, id)`,
	)
}

func migrateWebsiteSearchSchema(ctx context.Context, tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS website_search_targets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			normalized_url TEXT NOT NULL UNIQUE,
			last_checked_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS website_search_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query TEXT NOT NULL,
			use_browser_fallback INTEGER NOT NULL DEFAULT 0 CHECK (use_browser_fallback IN (0, 1)),
			checked_count INTEGER NOT NULL DEFAULT 0,
			found_count INTEGER NOT NULL DEFAULT 0,
			failed_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS website_search_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL REFERENCES website_search_runs(id) ON DELETE CASCADE,
			target_id INTEGER REFERENCES website_search_targets(id) ON DELETE SET NULL,
			original_url TEXT NOT NULL,
			final_url TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			hostname TEXT NOT NULL DEFAULT '',
			match_count INTEGER NOT NULL DEFAULT 0,
			snippets_json TEXT NOT NULL DEFAULT '[]',
			open_url TEXT NOT NULL DEFAULT '',
			fetch_method TEXT NOT NULL DEFAULT '',
			error_code TEXT NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT '',
			fallback_warning TEXT NOT NULL DEFAULT '',
			checked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_website_search_runs_created
			ON website_search_runs(created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_website_search_results_run
			ON website_search_results(run_id, id)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("create website search schema: %w", err)
		}
	}
	return nil
}

// hasUniqueSingleColumnIndex recognises the existing table-level UNIQUE
// constraint in legacy databases so migration 2 does not create a duplicate
// equivalent index with a new name.
func hasUniqueSingleColumnIndex(
	ctx context.Context,
	tx *sql.Tx,
	table string,
	column string,
) (bool, error) {
	indexRows, err := tx.QueryContext(
		ctx,
		fmt.Sprintf("PRAGMA index_list(%s)", quoteSQLiteIdentifier(table)),
	)
	if err != nil {
		return false, fmt.Errorf("inspect indexes for %s: %w", table, err)
	}

	uniqueIndexNames := []string{}
	for indexRows.Next() {
		var (
			sequence  int
			indexName string
			isUnique  int
			origin    string
			partial   int
		)
		if err := indexRows.Scan(&sequence, &indexName, &isUnique, &origin, &partial); err != nil {
			_ = indexRows.Close()
			return false, fmt.Errorf("scan index metadata for %s: %w", table, err)
		}
		if isUnique != 0 {
			uniqueIndexNames = append(uniqueIndexNames, indexName)
		}
	}
	if err := indexRows.Err(); err != nil {
		_ = indexRows.Close()
		return false, fmt.Errorf("iterate indexes for %s: %w", table, err)
	}
	if err := indexRows.Close(); err != nil {
		return false, fmt.Errorf("close indexes for %s: %w", table, err)
	}

	for _, indexName := range uniqueIndexNames {
		columnRows, err := tx.QueryContext(
			ctx,
			fmt.Sprintf("PRAGMA index_info(%s)", quoteSQLiteIdentifier(indexName)),
		)
		if err != nil {
			return false, fmt.Errorf("inspect index %s: %w", indexName, err)
		}

		columnCount := 0
		matchesColumn := true
		for columnRows.Next() {
			var (
				sequence int
				columnID int
				name     sql.NullString
			)
			if err := columnRows.Scan(&sequence, &columnID, &name); err != nil {
				_ = columnRows.Close()
				return false, fmt.Errorf("scan index columns for %s: %w", indexName, err)
			}
			columnCount++
			if !name.Valid || name.String != column {
				matchesColumn = false
			}
		}
		if err := columnRows.Err(); err != nil {
			_ = columnRows.Close()
			return false, fmt.Errorf("iterate index columns for %s: %w", indexName, err)
		}
		if err := columnRows.Close(); err != nil {
			return false, fmt.Errorf("close index columns for %s: %w", indexName, err)
		}

		if columnCount == 1 && matchesColumn {
			return true, nil
		}
	}

	return false, nil
}

func quoteSQLiteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
