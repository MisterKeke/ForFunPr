package backend

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
