package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openRawStorageTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestApplyMigrationsUpgradesDataAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openRawStorageTestDB(t, filepath.Join(t.TempDir(), "upgrade.db"))
	if err := configureSQLite(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (
		version INTEGER PRIMARY KEY, name TEXT NOT NULL,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrateCoreSchema(ctx, tx); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO schema_migrations(version, name) VALUES (1, 'legacy core')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO todos(title, priority) VALUES ('preserve me', 'high')`); err != nil {
		t.Fatal(err)
	}

	if err := applyMigrations(ctx, db); err != nil {
		t.Fatal(err)
	}
	assertMigrationState(t, db)
	if err := applyMigrations(ctx, db); err != nil {
		t.Fatalf("second migration pass: %v", err)
	}
	assertMigrationState(t, db)

	var title string
	var difficulty sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT title, difficulty FROM todos WHERE title = 'preserve me'`).Scan(&title, &difficulty); err != nil {
		t.Fatal(err)
	}
	if title != "preserve me" || difficulty.Valid {
		t.Fatalf("migrated todo = (%q, %#v)", title, difficulty)
	}
}

func assertMigrationState(t *testing.T, db *sql.DB) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != len(migrations) {
		t.Fatalf("migration count = %d, want %d", count, len(migrations))
	}
	for _, table := range []string{"notes", "bookmarks", "setups", "screenshots", "world_clocks", "website_search_runs"} {
		var found int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&found); err != nil {
			t.Fatal(err)
		}
		if found != 1 {
			t.Fatalf("table %q was not created", table)
		}
	}
}

func TestFavoriteMigrationRejectsDuplicateNormalizedNamesAtomically(t *testing.T) {
	ctx := context.Background()
	db := openRawStorageTestDB(t, filepath.Join(t.TempDir(), "duplicates.db"))
	if _, err := db.ExecContext(ctx, `CREATE TABLE favorite_categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		name_normalized TEXT NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO favorite_categories(name, name_normalized)
		VALUES ('News', 'telegram:news'), ('news', 'telegram:news')`); err != nil {
		t.Fatal(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = migrateFavoriteSchema(ctx, tx)
	if err == nil || !strings.Contains(err.Error(), "duplicate normalized") {
		_ = tx.Rollback()
		t.Fatalf("migration error = %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	rows, err := db.QueryContext(ctx, `PRAGMA table_info(favorite_categories)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	foundSource := false
	for rows.Next() {
		var id, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&id, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		foundSource = foundSource || name == "source"
	}
	if foundSource {
		t.Fatal("failed migration left its added column behind")
	}
}

func TestUniqueSingleColumnIndexRecognition(t *testing.T) {
	ctx := context.Background()
	db := openRawStorageTestDB(t, ":memory:")
	if _, err := db.ExecContext(ctx, `CREATE TABLE example (
		id INTEGER PRIMARY KEY,
		name TEXT UNIQUE,
		left_value TEXT,
		right_value TEXT,
		UNIQUE(left_value, right_value)
	)`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	unique, err := hasUniqueSingleColumnIndex(ctx, tx, "example", "name")
	if err != nil {
		t.Fatal(err)
	}
	if !unique {
		t.Fatal("table-level single-column UNIQUE constraint was not recognized")
	}
	unique, err = hasUniqueSingleColumnIndex(ctx, tx, "example", "left_value")
	if err != nil {
		t.Fatal(err)
	}
	if unique {
		t.Fatal("composite UNIQUE constraint was mistaken for a single-column index")
	}
}

func TestLegacyDatabaseCopyMigratesWithoutOverwriting(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "legacy.db")
	source, err := openDatabase(ctx, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.ExecContext(ctx, `INSERT INTO todos(title) VALUES ('legacy task')`); err != nil {
		_ = source.Close()
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}

	originalResolver := legacyDatabasePathResolver
	legacyDatabasePathResolver = func() (string, error) { return sourcePath, nil }
	t.Cleanup(func() { legacyDatabasePathResolver = originalResolver })

	targetPath := filepath.Join(directory, "new", "database.db")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := copyLegacyDatabaseIfNeeded(targetPath); err != nil {
		t.Fatal(err)
	}
	if err := validateSQLiteDatabase(targetPath, []string{"schema_migrations", "todos"}); err != nil {
		t.Fatal(err)
	}
	copied := openRawStorageTestDB(t, targetPath)
	var title string
	if err := copied.QueryRow(`SELECT title FROM todos`).Scan(&title); err != nil || title != "legacy task" {
		t.Fatalf("copied title/error = %q/%v", title, err)
	}
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatalf("legacy source was removed: %v", err)
	}

	existingPath := filepath.Join(directory, "existing.db")
	if err := os.WriteFile(existingPath, []byte("do not replace"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyLegacyDatabaseIfNeeded(existingPath); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "do not replace" {
		t.Fatalf("existing target was overwritten: %q", contents)
	}
}

func TestDatabaseValidationRejectsCorruptionAndMissingTables(t *testing.T) {
	corrupt := filepath.Join(t.TempDir(), "corrupt.db")
	if err := os.WriteFile(corrupt, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateSQLiteDatabase(corrupt, nil); err == nil {
		t.Fatal("corrupt database unexpectedly validated")
	}

	validPath := filepath.Join(t.TempDir(), "valid.db")
	db := openRawStorageTestDB(t, validPath)
	if _, err := db.Exec(`CREATE TABLE present(id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	if err := validateSQLiteDatabase(validPath, []string{"missing"}); err == nil {
		t.Fatal("database with a missing expected table unexpectedly validated")
	}
}

func TestLegacyCopyRejectsDirectoryTarget(t *testing.T) {
	target := filepath.Join(t.TempDir(), "database.db")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := copyLegacyDatabaseIfNeeded(target); err == nil {
		t.Fatal("directory target unexpectedly accepted")
	}
}

func TestQuoteSQLiteIdentifierEscapesQuotes(t *testing.T) {
	if got := quoteSQLiteIdentifier(`a"b`); got != `"a""b"` {
		t.Fatalf("quoted identifier = %q", got)
	}
}
