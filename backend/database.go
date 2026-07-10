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
	appDataDirectoryName = "currency-wails"
	databaseFileName     = "database.db"
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

// applicationDatabasePath returns a stable per-user location instead of
// relying on the executable's working directory.
func applicationDatabasePath() (string, error) {
	configDirectory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}

	databaseDirectory := filepath.Join(configDirectory, appDataDirectoryName)
	if err := os.MkdirAll(databaseDirectory, 0o700); err != nil {
		return "", fmt.Errorf("create database directory: %w", err)
	}

	return filepath.Join(databaseDirectory, databaseFileName), nil
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

	legacyPath, err := filepath.Abs(databaseFileName)
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

	source, err := os.Open(legacyPath)
	if err != nil {
		return fmt.Errorf("open legacy database: %w", err)
	}
	defer source.Close()

	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create migrated database: %w", err)
	}

	removeIncompleteTarget := true
	defer func() {
		_ = target.Close()
		if removeIncompleteTarget {
			_ = os.Remove(targetPath)
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

	removeIncompleteTarget = false
	return nil
}

// Close releases the SQLite connection during application shutdown.
func (a *App) Close() error {
	if a.httpClient != nil {
		a.httpClient.closeIdleConnections()
	}

	if a.db == nil {
		return nil
	}

	db := a.db
	a.db = nil
	return db.Close()
}
