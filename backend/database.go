package backend

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const (
	appDataDirectoryName        = "currency-wails"
	databaseFileName            = "database.db"
	userWallpaperDirectoryName  = "user-wallpapers"
	desktopAppIconDirectoryName = "icons"
	steamGameImageDirectoryName = "steam-game-images"
)

// openDatabase opens the application's SQLite database, configures it, and
// applies all outstanding schema migrations.
//
// The returned path is useful for diagnostics, but should not normally be
// exposed to the frontend.
func openDatabase(ctx context.Context) (*sql.DB, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	path, err := applicationDatabasePath()
	if err != nil {
		return nil, "", err
	}

	// Preserve existing local data when upgrading from the old relative-path
	// database location. The original database is retained as a backup.
	if err := copyLegacyDatabaseIfNeeded(path); err != nil {
		return nil, "", err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, "", fmt.Errorf("open database: %w", err)
	}

	// SQLite PRAGMAs such as foreign_keys are connection-specific. A single
	// connection keeps this desktop application's configuration consistent.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	closeOnError := func(cause error) (*sql.DB, string, error) {
		_ = db.Close()
		return nil, "", cause
	}

	if err := db.PingContext(ctx); err != nil {
		return closeOnError(fmt.Errorf("connect to database: %w", err))
	}

	if err := configureSQLite(ctx, db); err != nil {
		return closeOnError(err)
	}

	if err := applyMigrations(ctx, db); err != nil {
		return closeOnError(err)
	}

	return db, path, nil
}

// applicationDataDirectory returns the stable per-user directory shared by
// the SQLite database and other persistent application files.
func applicationDataDirectory() (string, error) {
	configDirectory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}

	directory := filepath.Join(configDirectory, appDataDirectoryName)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create application data directory: %w", err)
	}

	return directory, nil
}

// applicationDatabasePath returns a stable per-user location instead of
// relying on the executable's working directory.
func applicationDatabasePath() (string, error) {
	directory, err := applicationDataDirectory()
	if err != nil {
		return "", err
	}

	return filepath.Join(directory, databaseFileName), nil
}

func applicationWallpaperDirectory() (string, error) {
	directory, err := applicationDataDirectory()
	if err != nil {
		return "", err
	}

	wallpaperDirectory := filepath.Join(directory, userWallpaperDirectoryName)
	if err := os.MkdirAll(wallpaperDirectory, 0o700); err != nil {
		return "", fmt.Errorf("create wallpaper directory: %w", err)
	}

	return wallpaperDirectory, nil
}

func applicationDesktopAppIconDirectory() (string, error) {
	directory, err := applicationDataDirectory()
	if err != nil {
		return "", err
	}

	iconDirectory := filepath.Join(directory, desktopAppIconDirectoryName)
	if err := os.MkdirAll(iconDirectory, 0o700); err != nil {
		return "", fmt.Errorf("create application icon directory: %w", err)
	}

	return iconDirectory, nil
}

func applicationSteamGameImageDirectory() (string, error) {
	directory, err := applicationDataDirectory()
	if err != nil {
		return "", err
	}

	imageDirectory := filepath.Join(directory, steamGameImageDirectoryName)
	if err := os.MkdirAll(imageDirectory, 0o700); err != nil {
		return "", fmt.Errorf("create Steam game image directory: %w", err)
	}

	return imageDirectory, nil
}

func configureSQLite(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	}

	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("configure SQLite with %q: %w", pragma, err)
		}
	}

	return nil
}

// copyLegacyDatabaseIfNeeded copies the existing database.db from the old
// working-directory location only when the new application-data database does
// not yet exist. It does not delete or overwrite the legacy file.
func copyLegacyDatabaseIfNeeded(targetPath string) error {
	targetInfo, err := os.Stat(targetPath)
	if err == nil {
		if targetInfo.IsDir() {
			return fmt.Errorf("database path is a directory: %s", targetPath)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect database path: %w", err)
	}

	legacyPath, err := legacyDatabasePath()
	if err != nil {
		return fmt.Errorf("resolve legacy database path: %w", err)
	}

	if filepath.Clean(legacyPath) == filepath.Clean(targetPath) {
		return nil
	}

	legacyInfo, err := os.Stat(legacyPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect legacy database: %w", err)
	}
	if legacyInfo.IsDir() {
		return fmt.Errorf("legacy database path is a directory: %s", legacyPath)
	}
	if err := validateSQLiteDatabase(legacyPath, []string{"favorite_rates", "todos"}); err != nil {
		return fmt.Errorf("validate legacy database: %w", err)
	}

	source, err := os.Open(legacyPath)
	if err != nil {
		return fmt.Errorf("open legacy database: %w", err)
	}
	defer source.Close()

	target, err := os.CreateTemp(filepath.Dir(targetPath), ".database-migration-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary migrated database: %w", err)
	}
	temporaryPath := target.Name()
	if err := target.Chmod(0o600); err != nil {
		_ = target.Close()
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("secure temporary migrated database: %w", err)
	}

	removeIncompleteTarget := true
	defer func() {
		_ = target.Close()
		if removeIncompleteTarget {
			_ = os.Remove(temporaryPath)
			_ = os.Remove(temporaryPath + "-wal")
			_ = os.Remove(temporaryPath + "-shm")
		}
	}()

	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("copy legacy database: %w", err)
	}

	if err := target.Sync(); err != nil {
		return fmt.Errorf("flush migrated database: %w", err)
	}

	if err := target.Close(); err != nil {
		return fmt.Errorf("close migrated database: %w", err)
	}

	if err := migrateCopiedDatabase(temporaryPath); err != nil {
		return err
	}
	if err := validateSQLiteDatabase(temporaryPath, []string{
		"schema_migrations", "favorite_rates", "todos", "app_state",
	}); err != nil {
		return fmt.Errorf("validate migrated database: %w", err)
	}

	// A hard-link install is atomic and, unlike os.Rename on Unix, cannot
	// overwrite a destination that appeared during migration.
	if err := os.Link(temporaryPath, targetPath); err != nil {
		if _, statErr := os.Stat(targetPath); statErr == nil {
			removeIncompleteTarget = true
			return nil
		}
		return fmt.Errorf("install migrated database without overwrite: %w", err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return fmt.Errorf("remove installed migration temporary file: %w", err)
	}
	_ = os.Remove(temporaryPath + "-wal")
	_ = os.Remove(temporaryPath + "-shm")
	removeIncompleteTarget = false
	return nil
}

// legacyDatabasePath is deliberately anchored to the installed executable.
// It never consults the process working directory.
func legacyDatabasePath() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	executablePath, err = filepath.EvalSymlinks(executablePath)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(executablePath), databaseFileName), nil
}

func validateSQLiteDatabase(path string, expectedTables []string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	header := make([]byte, 16)
	_, readErr := io.ReadFull(file, header)
	closeErr := file.Close()
	if readErr != nil {
		return fmt.Errorf("read SQLite header: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close SQLite validation source: %w", closeErr)
	}
	if string(header) != "SQLite format 3\x00" {
		return fmt.Errorf("file does not have a SQLite header")
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open SQLite validation database: %w", err)
	}
	defer db.Close()
	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("check SQLite integrity: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("SQLite integrity check failed")
	}
	for _, table := range expectedTables {
		var found string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&found)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("expected table %q is missing", table)
		}
		if err != nil {
			return fmt.Errorf("inspect expected table %q: %w", table, err)
		}
	}
	return nil
}

func migrateCopiedDatabase(path string) error {
	ctx := context.Background()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open copied legacy database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("connect to copied legacy database: %w", err)
	}
	if err := configureSQLite(ctx, db); err != nil {
		_ = db.Close()
		return err
	}
	if err := applyMigrations(ctx, db); err != nil {
		_ = db.Close()
		return err
	}
	if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		_ = db.Close()
		return fmt.Errorf("checkpoint migrated database: %w", err)
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("close migrated database after validation: %w", err)
	}
	return nil
}

// Close releases the SQLite connection during application shutdown.
func (a *Service) Close() error {
	if a == nil {
		return nil
	}
	if a.httpClient != nil {
		a.httpClient.closeIdleConnections()
	}

	a.lifecycleMu.Lock()
	if a.db == nil {
		a.lifecycleMu.Unlock()
		return nil
	}

	db := a.db
	a.db = nil
	a.ready = false
	a.lifecycleMu.Unlock()
	return db.Close()
}
