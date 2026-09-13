package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestGitWorkspaceSchemaIsPresentInFreshInMemoryDatabase(t *testing.T) {
	ctx := context.Background()
	db, err := OpenInMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, table := range []string{
		"git_workspace_roots", "git_repositories", "git_workspace_repositories",
		"git_repository_status_cache", "git_workspace_settings",
	} {
		var count int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("table %q was not created", table)
		}
	}

	var foreignKeys int
	if err := db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
	var workerCount, staleDays, revision int
	if err := db.QueryRowContext(ctx, `
		SELECT worker_count, stale_days, revision
		FROM git_workspace_settings WHERE id = 1
	`).Scan(&workerCount, &staleDays, &revision); err != nil {
		t.Fatal(err)
	}
	if workerCount != 4 || staleDays != 30 || revision != 1 {
		t.Fatalf("default Git workspace settings = (%d, %d, %d)", workerCount, staleDays, revision)
	}
}

func TestGitWorkspaceMigrationUpgradesFromVersion22(t *testing.T) {
	ctx := context.Background()
	db := openRawStorageTestDB(t, filepath.Join(t.TempDir(), "version-22.db"))
	if err := configureSQLite(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (
		version INTEGER PRIMARY KEY, name TEXT NOT NULL,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations[:len(migrations)-1] {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := migration.up(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("apply migration %d: %v", migration.version, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations(version, name) VALUES (?, ?)`,
			migration.version, migration.name,
		); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}

	if err := applyMigrations(ctx, db); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 24 {
		t.Fatalf("latest migration version = %d, want 24", version)
	}
	var settings int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM git_workspace_settings`).Scan(&settings); err != nil {
		t.Fatal(err)
	}
	if settings != 1 {
		t.Fatalf("settings rows = %d, want 1", settings)
	}
}

func TestGitWorkspaceSchemaForeignKeysCascadeOnlyOwnedRows(t *testing.T) {
	ctx := context.Background()
	db, err := OpenInMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `
		INSERT INTO git_workspace_roots(display_name, root_path, root_path_key)
		VALUES ('Root', 'C:\\Root', 'c:\\root')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO git_repositories(name, repository_path, repository_path_key)
		VALUES ('Repo', 'C:\\Root\\Repo', 'c:\\root\\repo')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO git_workspace_repositories(workspace_id, repository_id) VALUES (1, 1)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO git_repository_status_cache(repository_id, checked_at) VALUES (1, CURRENT_TIMESTAMP)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM git_workspace_roots WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	var memberships, repositories, statuses int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM git_workspace_repositories`).Scan(&memberships); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM git_repositories`).Scan(&repositories); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM git_repository_status_cache`).Scan(&statuses); err != nil {
		t.Fatal(err)
	}
	if memberships != 0 || repositories != 1 || statuses != 1 {
		t.Fatalf("after root deletion memberships/repositories/status = %d/%d/%d", memberships, repositories, statuses)
	}
}

func TestGitWorkspaceMigrationIsLatestMonotonicAndTransactional(t *testing.T) {
	if len(migrations) == 0 {
		t.Fatal("migration list is empty")
	}
	for index := 1; index < len(migrations); index++ {
		if migrations[index].version <= migrations[index-1].version {
			t.Fatalf("migration versions are not strictly increasing at index %d", index)
		}
	}
	latest := migrations[len(migrations)-1]
	if latest.version != 24 || latest.name != "allow HTTP Git remote web URLs" {
		t.Fatalf("latest migration = (%d, %q)", latest.version, latest.name)
	}

	ctx := context.Background()
	db := openRawStorageTestDB(t, filepath.Join(t.TempDir(), "rollback.db"))
	if err := configureSQLite(ctx, db); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrateGitWorkspaceSchema(ctx, tx); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name LIKE 'git_%'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("rolled-back Git Workspace migration left %d tables", count)
	}
}
