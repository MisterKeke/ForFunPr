package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maximumGitWorkspaceNameLength  = 120
	maximumGitRepositoryNameLength = 200
	maximumGitRemoteDisplayLength  = 512
	maximumGitRemoteURLLength      = 4096
	maximumGitStatusTextLength     = 512
	maximumGitWorkspaceWorkers     = 64
	maximumGitWorkspaceStaleDays   = 3650
)

type GitWorkspaceRoot struct {
	ID            int    `json:"id"`
	DisplayName   string `json:"display_name"`
	RootPath      string `json:"-"`
	RootPathKey   string `json:"-"`
	LastScannedAt string `json:"last_scanned_at,omitempty"`
	Revision      int    `json:"revision"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type GitWorkspaceRootCreateRequest struct {
	DisplayName string `json:"display_name"`
	RootPath    string `json:"root_path"`
}

type GitWorkspaceRootUpdateRequest struct {
	ID               int    `json:"id"`
	DisplayName      string `json:"display_name"`
	RootPath         string `json:"root_path"`
	ExpectedRevision int    `json:"expected_revision"`
}

type GitWorkspaceRootDeleteRequest struct {
	ID               int `json:"id"`
	ExpectedRevision int `json:"expected_revision"`
}

type GitRepository struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	RepositoryPath    string `json:"-"`
	RepositoryPathKey string `json:"-"`
	Missing           bool   `json:"missing"`
	LastSeenAt        string `json:"last_seen_at,omitempty"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type GitRepositoryUpsert struct {
	Name           string `json:"name"`
	RepositoryPath string `json:"repository_path"`
}

type GitRepositoryScanResult = GitRepositoryUpsert

type GitRepositoryListFilter struct {
	WorkspaceID int    `json:"workspace_id,omitempty"`
	Query       string `json:"query,omitempty"`
	Missing     *bool  `json:"missing,omitempty"`
}

type GitWorkspaceScanReconcileRequest struct {
	WorkspaceID      int                       `json:"workspace_id"`
	Repositories     []GitRepositoryScanResult `json:"repositories"`
	ExpectedRevision *int                      `json:"expected_revision,omitempty"`
}

type GitRepositoryStatus struct {
	RepositoryID        int    `json:"repository_id"`
	Branch              string `json:"branch"`
	Dirty               bool   `json:"dirty"`
	ModifiedCount       int    `json:"modified_count"`
	AddedCount          int    `json:"added_count"`
	DeletedCount        int    `json:"deleted_count"`
	RenamedCount        int    `json:"renamed_count"`
	UntrackedCount      int    `json:"untracked_count"`
	Upstream            string `json:"upstream"`
	AheadCount          int    `json:"ahead_count"`
	BehindCount         int    `json:"behind_count"`
	SyncState           string `json:"sync_state"`
	RemoteDisplay       string `json:"remote_display"`
	RemoteWebURL        string `json:"remote_web_url"`
	LatestCommitSummary string `json:"latest_commit_summary"`
	CheckedAt           string `json:"checked_at"`
}

type GitRepositoryStatusWriteRequest struct {
	RepositoryID        int    `json:"repository_id"`
	Branch              string `json:"branch"`
	Dirty               bool   `json:"dirty"`
	ModifiedCount       int    `json:"modified_count"`
	AddedCount          int    `json:"added_count"`
	DeletedCount        int    `json:"deleted_count"`
	RenamedCount        int    `json:"renamed_count"`
	UntrackedCount      int    `json:"untracked_count"`
	Upstream            string `json:"upstream"`
	AheadCount          int    `json:"ahead_count"`
	BehindCount         int    `json:"behind_count"`
	SyncState           string `json:"sync_state"`
	RemoteDisplay       string `json:"remote_display"`
	RemoteWebURL        string `json:"remote_web_url"`
	LatestCommitSummary string `json:"latest_commit_summary"`
	CheckedAt           string `json:"checked_at,omitempty"`
}

type GitWorkspaceSettings struct {
	EditorApplicationID *int   `json:"editor_application_id,omitempty"`
	WorkerCount         int    `json:"worker_count"`
	StaleDays           int    `json:"stale_days"`
	Revision            int    `json:"revision"`
	UpdatedAt           string `json:"updated_at"`
}

type GitWorkspaceSettingsUpdateRequest struct {
	EditorApplicationID *int `json:"editor_application_id,omitempty"`
	WorkerCount         int  `json:"worker_count"`
	StaleDays           int  `json:"stale_days"`
	ExpectedRevision    int  `json:"expected_revision"`
}

type gitWorkspaceReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (a *Service) ListGitWorkspaceRootsContext(ctx context.Context) ([]GitWorkspaceRoot, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, display_name, root_path, root_path_key, last_scanned_at,
		       revision, created_at, updated_at
		FROM git_workspace_roots
		ORDER BY display_name COLLATE NOCASE ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list Git workspace roots: %w", err)
	}
	defer rows.Close()

	roots := make([]GitWorkspaceRoot, 0)
	for rows.Next() {
		root, err := scanGitWorkspaceRoot(rows)
		if err != nil {
			return nil, fmt.Errorf("scan Git workspace root: %w", err)
		}
		roots = append(roots, root)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Git workspace roots: %w", err)
	}
	return roots, nil
}

func (a *Service) GetGitWorkspaceRootContext(ctx context.Context, id int) (GitWorkspaceRoot, error) {
	if err := validateGitWorkspaceRootID(id); err != nil {
		return GitWorkspaceRoot{}, err
	}
	return loadGitWorkspaceRootContext(ctx, a.db, id)
}

func (a *Service) CreateGitWorkspaceRootContext(
	ctx context.Context,
	request GitWorkspaceRootCreateRequest,
) (GitWorkspaceRoot, error) {
	name, rootPath, rootPathKey, err := normalizeGitWorkspaceRootWrite(request.DisplayName, request.RootPath)
	if err != nil {
		return GitWorkspaceRoot{}, err
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return GitWorkspaceRoot{}, fmt.Errorf("begin Git workspace root creation: %w", err)
	}
	defer tx.Rollback()
	if err := ensureGitWorkspaceRootPathAvailable(ctx, tx, rootPathKey, 0); err != nil {
		return GitWorkspaceRoot{}, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO git_workspace_roots (display_name, root_path, root_path_key)
		VALUES (?, ?, ?)
	`, name, rootPath, rootPathKey)
	if err != nil {
		return GitWorkspaceRoot{}, fmt.Errorf("create Git workspace root: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return GitWorkspaceRoot{}, fmt.Errorf("read Git workspace root ID: %w", err)
	}
	root, err := loadGitWorkspaceRootContext(ctx, tx, int(id))
	if err != nil {
		return GitWorkspaceRoot{}, err
	}
	if err := tx.Commit(); err != nil {
		return GitWorkspaceRoot{}, fmt.Errorf("commit Git workspace root creation: %w", err)
	}
	a.emitGitWorkspaceInventoryChanged()
	return root, nil
}

func (a *Service) UpdateGitWorkspaceRootContext(
	ctx context.Context,
	request GitWorkspaceRootUpdateRequest,
) (GitWorkspaceRoot, error) {
	if err := validateGitWorkspaceRootID(request.ID); err != nil {
		return GitWorkspaceRoot{}, err
	}
	if request.ExpectedRevision <= 0 {
		return GitWorkspaceRoot{}, &ValidationError{Field: "expected_revision", Message: "expected revision must be positive"}
	}
	name, rootPath, rootPathKey, err := normalizeGitWorkspaceRootWrite(request.DisplayName, request.RootPath)
	if err != nil {
		return GitWorkspaceRoot{}, err
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return GitWorkspaceRoot{}, fmt.Errorf("begin Git workspace root update: %w", err)
	}
	defer tx.Rollback()
	current, err := loadGitWorkspaceRootContext(ctx, tx, request.ID)
	if err != nil {
		return GitWorkspaceRoot{}, err
	}
	if current.Revision != request.ExpectedRevision {
		return GitWorkspaceRoot{}, staleGitWorkspaceRootRevision(request.ID, request.ExpectedRevision, current.Revision)
	}
	if err := ensureGitWorkspaceRootPathAvailable(ctx, tx, rootPathKey, request.ID); err != nil {
		return GitWorkspaceRoot{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE git_workspace_roots
		SET display_name = ?, root_path = ?, root_path_key = ?,
		    revision = revision + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND revision = ?
	`, name, rootPath, rootPathKey, request.ID, request.ExpectedRevision)
	if err != nil {
		return GitWorkspaceRoot{}, fmt.Errorf("update Git workspace root: %w", err)
	}
	if err := requireGitWorkspaceRootRevisionMutation(result, request.ID, request.ExpectedRevision, current.Revision); err != nil {
		return GitWorkspaceRoot{}, err
	}
	updated, err := loadGitWorkspaceRootContext(ctx, tx, request.ID)
	if err != nil {
		return GitWorkspaceRoot{}, err
	}
	if err := tx.Commit(); err != nil {
		return GitWorkspaceRoot{}, fmt.Errorf("commit Git workspace root update: %w", err)
	}
	a.emitGitWorkspaceInventoryChanged()
	return updated, nil
}

func (a *Service) DeleteGitWorkspaceRootContext(ctx context.Context, request GitWorkspaceRootDeleteRequest) error {
	if err := validateGitWorkspaceRootID(request.ID); err != nil {
		return err
	}
	if request.ExpectedRevision <= 0 {
		return &ValidationError{Field: "expected_revision", Message: "expected revision must be positive"}
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Git workspace root deletion: %w", err)
	}
	defer tx.Rollback()
	current, err := loadGitWorkspaceRootContext(ctx, tx, request.ID)
	if err != nil {
		return err
	}
	if current.Revision != request.ExpectedRevision {
		return staleGitWorkspaceRootRevision(request.ID, request.ExpectedRevision, current.Revision)
	}
	result, err := tx.ExecContext(ctx, `
		DELETE FROM git_workspace_roots WHERE id = ? AND revision = ?
	`, request.ID, request.ExpectedRevision)
	if err != nil {
		return fmt.Errorf("delete Git workspace root: %w", err)
	}
	if err := requireGitWorkspaceRootRevisionMutation(result, request.ID, request.ExpectedRevision, current.Revision); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Git workspace root deletion: %w", err)
	}
	a.emitGitWorkspaceInventoryChanged()
	return nil
}

func (a *Service) DeleteGitWorkspaceRootByIDContext(ctx context.Context, id int, expectedRevision int) error {
	return a.DeleteGitWorkspaceRootContext(ctx, GitWorkspaceRootDeleteRequest{
		ID: id, ExpectedRevision: expectedRevision,
	})
}

func (a *Service) UpsertGitRepositoriesContext(
	ctx context.Context,
	repositories []GitRepositoryUpsert,
) ([]GitRepository, error) {
	normalized, err := normalizeGitRepositoryUpserts(repositories)
	if err != nil {
		return []GitRepository{}, err
	}
	if len(normalized) == 0 {
		return []GitRepository{}, nil
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return []GitRepository{}, fmt.Errorf("begin Git repository upsert: %w", err)
	}
	defer tx.Rollback()
	result, err := upsertGitRepositoriesTx(ctx, tx, normalized)
	if err != nil {
		return []GitRepository{}, err
	}
	if err := tx.Commit(); err != nil {
		return []GitRepository{}, fmt.Errorf("commit Git repository upsert: %w", err)
	}
	return result, nil
}

func (a *Service) GetGitRepositoryContext(ctx context.Context, id int) (GitRepository, error) {
	if err := validateGitRepositoryID(id); err != nil {
		return GitRepository{}, err
	}
	return loadGitRepositoryContext(ctx, a.db, id)
}

func (a *Service) ListGitRepositoriesContext(
	ctx context.Context,
	filter GitRepositoryListFilter,
) ([]GitRepository, error) {
	where := []string{"1 = 1"}
	args := []any{}
	if filter.WorkspaceID != 0 {
		if err := validateGitWorkspaceRootID(filter.WorkspaceID); err != nil {
			return []GitRepository{}, err
		}
		where = append(where, `EXISTS (
			SELECT 1 FROM git_workspace_repositories AS memberships
			WHERE memberships.repository_id = repositories.id
			  AND memberships.workspace_id = ?
		)`)
		args = append(args, filter.WorkspaceID)
	}
	if filter.Missing != nil {
		where = append(where, "repositories.missing = ?")
		args = append(args, boolDatabaseValue(*filter.Missing))
	}
	query := strings.TrimSpace(filter.Query)
	if utf8.RuneCountInString(query) > 256 {
		return []GitRepository{}, &ValidationError{Field: "query", Message: "repository search must be 256 characters or fewer"}
	}
	if query != "" {
		pattern := collectionLikePattern(query)
		where = append(where, "(LOWER(repositories.name) LIKE ? ESCAPE '\\' OR LOWER(repositories.repository_path) LIKE ? ESCAPE '\\')")
		args = append(args, pattern, pattern)
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT repositories.id, repositories.name, repositories.repository_path,
		       repositories.repository_path_key, repositories.missing,
		       repositories.last_seen_at, repositories.created_at, repositories.updated_at
		FROM git_repositories AS repositories
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY repositories.name COLLATE NOCASE ASC,
		         repositories.repository_path_key ASC, repositories.id ASC
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("list Git repositories: %w", err)
	}
	defer rows.Close()
	repositories := make([]GitRepository, 0)
	for rows.Next() {
		repository, err := scanGitRepository(rows)
		if err != nil {
			return nil, fmt.Errorf("scan Git repository: %w", err)
		}
		repositories = append(repositories, repository)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Git repositories: %w", err)
	}
	return repositories, nil
}

func (a *Service) ReconcileGitWorkspaceScanContext(
	ctx context.Context,
	request GitWorkspaceScanReconcileRequest,
) error {
	if err := validateGitWorkspaceRootID(request.WorkspaceID); err != nil {
		return err
	}
	if request.ExpectedRevision != nil && *request.ExpectedRevision <= 0 {
		return &ValidationError{Field: "expected_revision", Message: "expected revision must be positive"}
	}
	normalized, err := normalizeGitRepositoryUpserts(request.Repositories)
	if err != nil {
		return err
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Git workspace scan reconciliation: %w", err)
	}
	defer tx.Rollback()
	root, err := loadGitWorkspaceRootContext(ctx, tx, request.WorkspaceID)
	if err != nil {
		return err
	}
	if request.ExpectedRevision != nil && root.Revision != *request.ExpectedRevision {
		return staleGitWorkspaceRootRevision(request.WorkspaceID, *request.ExpectedRevision, root.Revision)
	}

	oldRepositoryIDs, err := gitWorkspaceRepositoryIDsContext(ctx, tx, request.WorkspaceID)
	if err != nil {
		return err
	}
	upserted, err := upsertGitRepositoriesTx(ctx, tx, normalized)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM git_workspace_repositories WHERE workspace_id = ?`, request.WorkspaceID); err != nil {
		return fmt.Errorf("replace Git workspace memberships: %w", err)
	}
	for _, repository := range upserted {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO git_workspace_repositories (workspace_id, repository_id)
			VALUES (?, ?)
			ON CONFLICT(workspace_id, repository_id) DO NOTHING
		`, request.WorkspaceID, repository.ID); err != nil {
			return fmt.Errorf("save Git workspace membership: %w", err)
		}
	}
	affectedIDs := append(append([]int{}, oldRepositoryIDs...), repositoryIDs(upserted)...)
	if err := refreshGitRepositoryMissingFlags(ctx, tx, affectedIDs); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE git_workspace_roots
		SET last_scanned_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, request.WorkspaceID); err != nil {
		return fmt.Errorf("record Git workspace scan time: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Git workspace scan reconciliation: %w", err)
	}
	return nil
}

func (a *Service) MarkGitRepositoriesMissingContext(ctx context.Context, ids []int) error {
	for _, id := range ids {
		if err := validateGitRepositoryID(id); err != nil {
			return err
		}
	}
	ids = uniquePositiveIDs(ids)
	if len(ids) == 0 {
		return nil
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin marking Git repositories missing: %w", err)
	}
	defer tx.Rollback()
	for _, id := range ids {
		result, err := tx.ExecContext(ctx, `
			UPDATE git_repositories
			SET missing = 1, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, id)
		if err != nil {
			return fmt.Errorf("mark Git repository missing: %w", err)
		}
		if err := requireSingleMutation(result, "mark Git repository missing", "Git repository", false); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit marking Git repositories missing: %w", err)
	}
	return nil
}

func (a *Service) MarkMissingGitRepositoriesContext(ctx context.Context, ids []int) error {
	return a.MarkGitRepositoriesMissingContext(ctx, ids)
}

func (a *Service) PruneUnreferencedMissingGitRepositoriesContext(ctx context.Context) (int, error) {
	result, err := a.db.ExecContext(ctx, `
		DELETE FROM git_repositories
		WHERE missing = 1
		  AND NOT EXISTS (
			SELECT 1 FROM git_workspace_repositories AS memberships
			WHERE memberships.repository_id = git_repositories.id
		  )
	`)
	if err != nil {
		return 0, fmt.Errorf("prune unreferenced missing Git repositories: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count pruned Git repositories: %w", err)
	}
	return int(count), nil
}

func (a *Service) PruneGitRepositoriesContext(ctx context.Context) error {
	_, err := a.PruneUnreferencedMissingGitRepositoriesContext(ctx)
	return err
}

func (a *Service) ReadGitRepositoryStatusContext(ctx context.Context, repositoryID int) (GitRepositoryStatus, error) {
	if err := validateGitRepositoryID(repositoryID); err != nil {
		return GitRepositoryStatus{}, err
	}
	return loadGitRepositoryStatusContext(ctx, a.db, repositoryID)
}

func (a *Service) GetGitRepositoryStatusContext(ctx context.Context, repositoryID int) (GitRepositoryStatus, error) {
	return a.ReadGitRepositoryStatusContext(ctx, repositoryID)
}

func (a *Service) WriteGitRepositoryStatusContext(
	ctx context.Context,
	request GitRepositoryStatusWriteRequest,
) (GitRepositoryStatus, error) {
	if err := validateGitRepositoryID(request.RepositoryID); err != nil {
		return GitRepositoryStatus{}, err
	}
	if err := validateGitStatusCounts(request); err != nil {
		return GitRepositoryStatus{}, err
	}
	branch, err := normalizeGitStatusText("branch", request.Branch, maximumGitStatusTextLength)
	if err != nil {
		return GitRepositoryStatus{}, err
	}
	upstream, err := normalizeGitStatusText("upstream", request.Upstream, maximumGitStatusTextLength)
	if err != nil {
		return GitRepositoryStatus{}, err
	}
	syncState, err := normalizeGitStatusText("sync_state", request.SyncState, 64)
	if err != nil {
		return GitRepositoryStatus{}, err
	}
	remoteDisplay, err := sanitizeGitRemoteDisplay(request.RemoteDisplay)
	if err != nil {
		return GitRepositoryStatus{}, err
	}
	remoteWebURL, err := normalizeGitRemoteWebURL(request.RemoteWebURL)
	if err != nil {
		return GitRepositoryStatus{}, err
	}
	latestSummary, err := normalizeGitStatusText("latest_commit_summary", request.LatestCommitSummary, maximumGitStatusTextLength)
	if err != nil {
		return GitRepositoryStatus{}, err
	}
	checkedAt := strings.TrimSpace(request.CheckedAt)
	if checkedAt == "" {
		checkedAt = time.Now().UTC().Format(time.RFC3339Nano)
	} else if err := validateGitStatusText("checked_at", checkedAt, maximumGitStatusTextLength); err != nil {
		return GitRepositoryStatus{}, err
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return GitRepositoryStatus{}, fmt.Errorf("begin Git repository status write: %w", err)
	}
	defer tx.Rollback()
	if _, err := loadGitRepositoryContext(ctx, tx, request.RepositoryID); err != nil {
		return GitRepositoryStatus{}, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO git_repository_status_cache (
			repository_id, branch, dirty, modified_count, added_count, deleted_count,
			renamed_count, untracked_count, upstream, ahead_count, behind_count,
			sync_state, remote_display, remote_web_url, latest_commit_summary, checked_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repository_id) DO UPDATE SET
			branch = excluded.branch,
			dirty = excluded.dirty,
			modified_count = excluded.modified_count,
			added_count = excluded.added_count,
			deleted_count = excluded.deleted_count,
			renamed_count = excluded.renamed_count,
			untracked_count = excluded.untracked_count,
			upstream = excluded.upstream,
			ahead_count = excluded.ahead_count,
			behind_count = excluded.behind_count,
			sync_state = excluded.sync_state,
			remote_display = excluded.remote_display,
			remote_web_url = excluded.remote_web_url,
			latest_commit_summary = excluded.latest_commit_summary,
			checked_at = excluded.checked_at
	`, request.RepositoryID, branch, boolDatabaseValue(request.Dirty), request.ModifiedCount,
		request.AddedCount, request.DeletedCount, request.RenamedCount, request.UntrackedCount,
		upstream, request.AheadCount, request.BehindCount, syncState, remoteDisplay,
		remoteWebURL, latestSummary, checkedAt)
	if err != nil {
		return GitRepositoryStatus{}, fmt.Errorf("write Git repository status: %w", err)
	}
	status, err := loadGitRepositoryStatusContext(ctx, tx, request.RepositoryID)
	if err != nil {
		return GitRepositoryStatus{}, err
	}
	if err := tx.Commit(); err != nil {
		return GitRepositoryStatus{}, fmt.Errorf("commit Git repository status write: %w", err)
	}
	return status, nil
}

func (a *Service) UpsertGitRepositoryStatusContext(
	ctx context.Context,
	request GitRepositoryStatusWriteRequest,
) (GitRepositoryStatus, error) {
	return a.WriteGitRepositoryStatusContext(ctx, request)
}

func (a *Service) GetGitWorkspaceSettingsContext(ctx context.Context) (GitWorkspaceSettings, error) {
	return loadGitWorkspaceSettingsContext(ctx, a.db)
}

func (a *Service) ReadGitWorkspaceSettingsContext(ctx context.Context) (GitWorkspaceSettings, error) {
	return a.GetGitWorkspaceSettingsContext(ctx)
}

func (a *Service) UpdateGitWorkspaceSettingsContext(
	ctx context.Context,
	request GitWorkspaceSettingsUpdateRequest,
) (GitWorkspaceSettings, error) {
	if request.ExpectedRevision <= 0 {
		return GitWorkspaceSettings{}, &ValidationError{Field: "expected_revision", Message: "expected revision must be positive"}
	}
	if request.WorkerCount <= 0 || request.WorkerCount > maximumGitWorkspaceWorkers {
		return GitWorkspaceSettings{}, &ValidationError{Field: "worker_count", Message: "worker count must be between 1 and 64"}
	}
	if request.StaleDays <= 0 || request.StaleDays > maximumGitWorkspaceStaleDays {
		return GitWorkspaceSettings{}, &ValidationError{Field: "stale_days", Message: "stale days must be between 1 and 3650"}
	}
	if request.EditorApplicationID != nil {
		if err := ValidateDesktopAppID(*request.EditorApplicationID); err != nil {
			return GitWorkspaceSettings{}, err
		}
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return GitWorkspaceSettings{}, fmt.Errorf("begin Git workspace settings update: %w", err)
	}
	defer tx.Rollback()
	current, err := loadGitWorkspaceSettingsContext(ctx, tx)
	if err != nil {
		return GitWorkspaceSettings{}, err
	}
	if current.Revision != request.ExpectedRevision {
		return GitWorkspaceSettings{}, &StaleRevisionError{
			Resource: "Git workspace settings", ID: 1,
			Expected: request.ExpectedRevision, Actual: current.Revision,
		}
	}
	if request.EditorApplicationID != nil {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM desktop_apps WHERE id = ?`, *request.EditorApplicationID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
			return GitWorkspaceSettings{}, &NotFoundError{Resource: "desktop app", Key: fmt.Sprint(*request.EditorApplicationID)}
		} else if err != nil {
			return GitWorkspaceSettings{}, fmt.Errorf("check Git editor application: %w", err)
		}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE git_workspace_settings
		SET editor_application_id = ?, worker_count = ?, stale_days = ?,
		    revision = revision + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1 AND revision = ?
	`, nullableGitEditorApplicationID(request.EditorApplicationID), request.WorkerCount,
		request.StaleDays, request.ExpectedRevision)
	if err != nil {
		return GitWorkspaceSettings{}, fmt.Errorf("update Git workspace settings: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return GitWorkspaceSettings{}, fmt.Errorf("check Git workspace settings update: %w", err)
	} else if affected != 1 {
		return GitWorkspaceSettings{}, &StaleRevisionError{
			Resource: "Git workspace settings", ID: 1,
			Expected: request.ExpectedRevision, Actual: current.Revision,
		}
	}
	updated, err := loadGitWorkspaceSettingsContext(ctx, tx)
	if err != nil {
		return GitWorkspaceSettings{}, err
	}
	if err := tx.Commit(); err != nil {
		return GitWorkspaceSettings{}, fmt.Errorf("commit Git workspace settings update: %w", err)
	}
	return updated, nil
}

func loadGitWorkspaceRootContext(ctx context.Context, store gitWorkspaceReader, id int) (GitWorkspaceRoot, error) {
	var root GitWorkspaceRoot
	var lastScannedAt sql.NullString
	err := store.QueryRowContext(ctx, `
		SELECT id, display_name, root_path, root_path_key, last_scanned_at,
		       revision, created_at, updated_at
		FROM git_workspace_roots WHERE id = ?
	`, id).Scan(&root.ID, &root.DisplayName, &root.RootPath, &root.RootPathKey,
		&lastScannedAt, &root.Revision, &root.CreatedAt, &root.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return GitWorkspaceRoot{}, &NotFoundError{Resource: "Git workspace root", Key: fmt.Sprint(id)}
	}
	if err != nil {
		return GitWorkspaceRoot{}, fmt.Errorf("load Git workspace root: %w", err)
	}
	if lastScannedAt.Valid {
		root.LastScannedAt = lastScannedAt.String
	}
	return root, nil
}

func scanGitWorkspaceRoot(scanner interface{ Scan(...any) error }) (GitWorkspaceRoot, error) {
	var root GitWorkspaceRoot
	var lastScannedAt sql.NullString
	err := scanner.Scan(&root.ID, &root.DisplayName, &root.RootPath, &root.RootPathKey,
		&lastScannedAt, &root.Revision, &root.CreatedAt, &root.UpdatedAt)
	if lastScannedAt.Valid {
		root.LastScannedAt = lastScannedAt.String
	}
	return root, err
}

func loadGitRepositoryContext(ctx context.Context, store gitWorkspaceReader, id int) (GitRepository, error) {
	var repository GitRepository
	var missing int
	var lastSeenAt sql.NullString
	err := store.QueryRowContext(ctx, `
		SELECT id, name, repository_path, repository_path_key, missing,
		       last_seen_at, created_at, updated_at
		FROM git_repositories WHERE id = ?
	`, id).Scan(&repository.ID, &repository.Name, &repository.RepositoryPath,
		&repository.RepositoryPathKey, &missing, &lastSeenAt,
		&repository.CreatedAt, &repository.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return GitRepository{}, &NotFoundError{Resource: "Git repository", Key: fmt.Sprint(id)}
	}
	if err != nil {
		return GitRepository{}, fmt.Errorf("load Git repository: %w", err)
	}
	repository.Missing = missing != 0
	if lastSeenAt.Valid {
		repository.LastSeenAt = lastSeenAt.String
	}
	return repository, nil
}

func scanGitRepository(scanner interface{ Scan(...any) error }) (GitRepository, error) {
	var repository GitRepository
	var missing int
	var lastSeenAt sql.NullString
	err := scanner.Scan(&repository.ID, &repository.Name, &repository.RepositoryPath,
		&repository.RepositoryPathKey, &missing, &lastSeenAt,
		&repository.CreatedAt, &repository.UpdatedAt)
	repository.Missing = missing != 0
	if lastSeenAt.Valid {
		repository.LastSeenAt = lastSeenAt.String
	}
	return repository, err
}

func loadGitRepositoryStatusContext(ctx context.Context, store gitWorkspaceReader, repositoryID int) (GitRepositoryStatus, error) {
	var status GitRepositoryStatus
	var dirty int
	err := store.QueryRowContext(ctx, `
		SELECT repository_id, branch, dirty, modified_count, added_count,
		       deleted_count, renamed_count, untracked_count, upstream,
		       ahead_count, behind_count, sync_state, remote_display,
		       remote_web_url, latest_commit_summary, checked_at
		FROM git_repository_status_cache WHERE repository_id = ?
	`, repositoryID).Scan(&status.RepositoryID, &status.Branch, &dirty,
		&status.ModifiedCount, &status.AddedCount, &status.DeletedCount,
		&status.RenamedCount, &status.UntrackedCount, &status.Upstream,
		&status.AheadCount, &status.BehindCount, &status.SyncState,
		&status.RemoteDisplay, &status.RemoteWebURL, &status.LatestCommitSummary,
		&status.CheckedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return GitRepositoryStatus{}, &NotFoundError{Resource: "Git repository status", Key: fmt.Sprint(repositoryID)}
	}
	if err != nil {
		return GitRepositoryStatus{}, fmt.Errorf("read Git repository status: %w", err)
	}
	status.Dirty = dirty != 0
	return status, nil
}

func loadGitWorkspaceSettingsContext(ctx context.Context, store gitWorkspaceReader) (GitWorkspaceSettings, error) {
	var settings GitWorkspaceSettings
	var editorApplicationID sql.NullInt64
	err := store.QueryRowContext(ctx, `
		SELECT editor_application_id, worker_count, stale_days, revision, updated_at
		FROM git_workspace_settings WHERE id = 1
	`).Scan(&editorApplicationID, &settings.WorkerCount, &settings.StaleDays,
		&settings.Revision, &settings.UpdatedAt)
	if err != nil {
		return GitWorkspaceSettings{}, fmt.Errorf("read Git workspace settings: %w", err)
	}
	if editorApplicationID.Valid {
		value := int(editorApplicationID.Int64)
		settings.EditorApplicationID = &value
	}
	return settings, nil
}

func ensureGitWorkspaceRootPathAvailable(ctx context.Context, store gitWorkspaceReader, pathKey string, excludedID int) error {
	var existingID int
	err := store.QueryRowContext(ctx, `
		SELECT id FROM git_workspace_roots
		WHERE root_path_key = ? AND id != ?
	`, pathKey, excludedID).Scan(&existingID)
	if err == nil {
		return &ConflictError{Resource: "Git workspace root", Message: "That workspace root is already saved."}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check Git workspace root path: %w", err)
	}
	return nil
}

func upsertGitRepositoriesTx(ctx context.Context, tx *sql.Tx, repositories []GitRepositoryUpsert) ([]GitRepository, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	seen := make(map[string]bool, len(repositories))
	result := make([]GitRepository, 0, len(repositories))
	for _, repository := range repositories {
		_, path, pathKey, err := normalizeGitRepositoryWrite(repository.Name, repository.RepositoryPath)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO git_repositories (
				name, repository_path, repository_path_key, missing, last_seen_at,
				created_at, updated_at
			) VALUES (?, ?, ?, 0, ?, ?, ?)
			ON CONFLICT(repository_path_key) DO UPDATE SET
				name = excluded.name,
				repository_path = excluded.repository_path,
				missing = 0,
				last_seen_at = excluded.last_seen_at,
				updated_at = excluded.updated_at
		`, repository.Name, path, pathKey, now, now, now); err != nil {
			return nil, fmt.Errorf("upsert Git repository: %w", err)
		}
		if seen[pathKey] {
			continue
		}
		seen[pathKey] = true
		var id int
		if err := tx.QueryRowContext(ctx, `SELECT id FROM git_repositories WHERE repository_path_key = ?`, pathKey).Scan(&id); err != nil {
			return nil, fmt.Errorf("read upserted Git repository ID: %w", err)
		}
		loaded, err := loadGitRepositoryContext(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, loaded)
	}
	return result, nil
}

func normalizeGitRepositoryUpserts(repositories []GitRepositoryUpsert) ([]GitRepositoryUpsert, error) {
	result := make([]GitRepositoryUpsert, 0, len(repositories))
	seen := make(map[string]bool, len(repositories))
	for _, repository := range repositories {
		name, _, pathKey, err := normalizeGitRepositoryWrite(repository.Name, repository.RepositoryPath)
		if err != nil {
			return nil, err
		}
		if seen[pathKey] {
			continue
		}
		seen[pathKey] = true
		result = append(result, GitRepositoryUpsert{Name: name, RepositoryPath: repository.RepositoryPath})
	}
	return result, nil
}

func normalizeGitWorkspaceRootWrite(name, path string) (string, string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", "", &ValidationError{Field: "display_name", Message: "workspace root name cannot be empty"}
	}
	if utf8.RuneCountInString(name) > maximumGitWorkspaceNameLength {
		return "", "", "", &ValidationError{Field: "display_name", Message: "workspace root name must be 120 characters or fewer"}
	}
	displayPath, pathKey, err := normalizeGitAbsolutePath("root_path", path)
	return name, displayPath, pathKey, err
}

func normalizeGitRepositoryWrite(name, path string) (string, string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", "", &ValidationError{Field: "name", Message: "repository name cannot be empty"}
	}
	if utf8.RuneCountInString(name) > maximumGitRepositoryNameLength {
		return "", "", "", &ValidationError{Field: "name", Message: "repository name must be 200 characters or fewer"}
	}
	displayPath, pathKey, err := normalizeGitAbsolutePath("repository_path", path)
	return name, displayPath, pathKey, err
}

func normalizeGitAbsolutePath(field, value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", &ValidationError{Field: field, Message: "path cannot be empty"}
	}
	path := filepath.Clean(value)
	if !filepath.IsAbs(path) {
		return "", "", &ValidationError{Field: field, Message: "path must be absolute"}
	}
	return path, strings.ToLower(path), nil
}

func normalizeGitStatusText(field, value string, maximum int) (string, error) {
	value = strings.TrimSpace(value)
	if err := validateGitStatusText(field, value, maximum); err != nil {
		return "", err
	}
	return value, nil
}

func validateGitStatusText(field, value string, maximum int) error {
	if utf8.RuneCountInString(value) > maximum {
		return &ValidationError{Field: field, Message: fmt.Sprintf("%s must be %d characters or fewer", field, maximum)}
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return &ValidationError{Field: field, Message: fmt.Sprintf("%s contains unsupported control characters", field)}
		}
	}
	return nil
}

func sanitizeGitRemoteDisplay(value string) (string, error) {
	value = strings.TrimSpace(value)
	if err := validateGitStatusText("remote_display", value, maximumGitRemoteDisplayLength); err != nil {
		return "", err
	}
	if value == "" {
		return "", nil
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.User = nil
		parsed.RawQuery = ""
		parsed.Fragment = ""
		value = parsed.String()
	} else if at := strings.IndexByte(value, '@'); at > 0 {
		value = value[at+1:]
	}
	return value, nil
}

func normalizeGitRemoteWebURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if err := validateGitStatusText("remote_web_url", value, maximumGitRemoteURLLength); err != nil {
		return "", err
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return "", &ValidationError{Field: "remote_web_url", Message: "remote web URL must be an HTTPS URL without credentials or query data"}
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
		return "", &ValidationError{Field: "remote_web_url", Message: "remote web URL must be an HTTPS URL without credentials or query data"}
	}
	parsed.Scheme = "https"
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed.String(), nil
}

func validateGitStatusCounts(request GitRepositoryStatusWriteRequest) error {
	counts := []struct {
		field string
		value int
	}{
		{"modified_count", request.ModifiedCount}, {"added_count", request.AddedCount},
		{"deleted_count", request.DeletedCount}, {"renamed_count", request.RenamedCount},
		{"untracked_count", request.UntrackedCount}, {"ahead_count", request.AheadCount},
		{"behind_count", request.BehindCount},
	}
	for _, count := range counts {
		if count.value < 0 {
			return &ValidationError{Field: count.field, Message: count.field + " cannot be negative"}
		}
	}
	return nil
}

func validateGitWorkspaceRootID(id int) error {
	if id <= 0 {
		return &ValidationError{Field: "id", Message: "workspace root ID must be positive"}
	}
	return nil
}

func validateGitRepositoryID(id int) error {
	if id <= 0 {
		return &ValidationError{Field: "repository_id", Message: "repository ID must be positive"}
	}
	return nil
}

func staleGitWorkspaceRootRevision(id, expected, actual int) error {
	return &StaleRevisionError{Resource: "Git workspace root", ID: id, Expected: expected, Actual: actual}
}

func requireGitWorkspaceRootRevisionMutation(result sql.Result, id, expected, actual int) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check Git workspace root revision: %w", err)
	}
	if affected == 1 {
		return nil
	}
	if affected != 0 {
		return fmt.Errorf("Git workspace root mutation affected %d rows", affected)
	}
	return staleGitWorkspaceRootRevision(id, expected, actual)
}

func gitWorkspaceRepositoryIDsContext(ctx context.Context, store interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, workspaceID int) ([]int, error) {
	rows, err := store.QueryContext(ctx, `
		SELECT repository_id FROM git_workspace_repositories
		WHERE workspace_id = ? ORDER BY repository_id
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("read Git workspace memberships: %w", err)
	}
	defer rows.Close()
	ids := make([]int, 0)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan Git workspace membership: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Git workspace memberships: %w", err)
	}
	return ids, nil
}

func refreshGitRepositoryMissingFlags(ctx context.Context, tx *sql.Tx, ids []int) error {
	ids = uniquePositiveIDs(ids)
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	args = append(args, time.Now().UTC().Format(time.RFC3339Nano))
	for _, id := range ids {
		args = append(args, id)
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE git_repositories
		SET missing = CASE WHEN EXISTS (
			SELECT 1 FROM git_workspace_repositories AS memberships
			WHERE memberships.repository_id = git_repositories.id
		) THEN 0 ELSE 1 END,
		updated_at = ?
		WHERE id IN (`+placeholders+`)
	`, args...)
	if err != nil {
		return fmt.Errorf("refresh Git repository missing flags: %w", err)
	}
	return nil
}

func repositoryIDs(repositories []GitRepository) []int {
	ids := make([]int, 0, len(repositories))
	for _, repository := range repositories {
		ids = append(ids, repository.ID)
	}
	return ids
}

func uniquePositiveIDs(ids []int) []int {
	seen := make(map[int]bool, len(ids))
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

func nullableGitEditorApplicationID(id *int) any {
	if id == nil {
		return nil
	}
	return *id
}
