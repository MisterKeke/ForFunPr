package service

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitWorkspaceRootsNormalizePathsAndUseRevisions(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	rootPath := filepath.Join(t.TempDir(), "Workspace")
	root, err := service.CreateGitWorkspaceRootContext(ctx, GitWorkspaceRootCreateRequest{
		DisplayName: " Main workspace ", RootPath: rootPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if root.Revision != 1 || root.DisplayName != "Main workspace" || root.RootPathKey != strings.ToLower(root.RootPath) {
		t.Fatalf("created Git workspace root = %#v", root)
	}

	_, err = service.CreateGitWorkspaceRootContext(ctx, GitWorkspaceRootCreateRequest{
		DisplayName: "Duplicate", RootPath: strings.ToUpper(rootPath),
	})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("duplicate root error = %v, want ConflictError", err)
	}

	updated, err := service.UpdateGitWorkspaceRootContext(ctx, GitWorkspaceRootUpdateRequest{
		ID: root.ID, DisplayName: "Renamed", RootPath: rootPath, ExpectedRevision: root.Revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.DisplayName != "Renamed" {
		t.Fatalf("updated Git workspace root = %#v", updated)
	}
	_, err = service.UpdateGitWorkspaceRootContext(ctx, GitWorkspaceRootUpdateRequest{
		ID: root.ID, DisplayName: "Stale", RootPath: rootPath, ExpectedRevision: root.Revision,
	})
	var stale *StaleRevisionError
	if !errors.As(err, &stale) {
		t.Fatalf("stale root update error = %v, want StaleRevisionError", err)
	}
}

func TestGitWorkspaceReconciliationSupportsOverlapAndIsAtomic(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	rootOne, err := service.CreateGitWorkspaceRootContext(ctx, GitWorkspaceRootCreateRequest{
		DisplayName: "One", RootPath: filepath.Join(t.TempDir(), "one"),
	})
	if err != nil {
		t.Fatal(err)
	}
	rootTwo, err := service.CreateGitWorkspaceRootContext(ctx, GitWorkspaceRootCreateRequest{
		DisplayName: "Two", RootPath: filepath.Join(t.TempDir(), "two"),
	})
	if err != nil {
		t.Fatal(err)
	}
	repoOnePath := filepath.Join(t.TempDir(), "repo-one")
	repoTwoPath := filepath.Join(t.TempDir(), "repo-two")
	firstScan := []GitRepositoryScanResult{
		{Name: "Repo one", RepositoryPath: repoOnePath},
		{Name: "Repo two", RepositoryPath: repoTwoPath},
	}
	if err := service.ReconcileGitWorkspaceScanContext(ctx, GitWorkspaceScanReconcileRequest{
		WorkspaceID: rootOne.ID, Repositories: firstScan, ExpectedRevision: &rootOne.Revision,
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.ReconcileGitWorkspaceScanContext(ctx, GitWorkspaceScanReconcileRequest{
		WorkspaceID:  rootTwo.ID,
		Repositories: []GitRepositoryScanResult{{Name: "Repo one", RepositoryPath: strings.ToUpper(repoOnePath)}},
	}); err != nil {
		t.Fatal(err)
	}

	all, err := service.ListGitRepositoriesContext(ctx, GitRepositoryListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].ID == all[1].ID {
		t.Fatalf("overlapping repository inventory = %#v", all)
	}
	workspaceTwo, err := service.ListGitRepositoriesContext(ctx, GitRepositoryListFilter{WorkspaceID: rootTwo.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(workspaceTwo) != 1 || workspaceTwo[0].RepositoryPathKey != strings.ToLower(repoOnePath) {
		t.Fatalf("overlapping workspace inventory = %#v", workspaceTwo)
	}

	rootBeforeFailedScan, err := service.GetGitWorkspaceRootContext(ctx, rootOne.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ReconcileGitWorkspaceScanContext(ctx, GitWorkspaceScanReconcileRequest{
		WorkspaceID: rootOne.ID,
		Repositories: []GitRepositoryScanResult{
			{Name: "Replacement", RepositoryPath: filepath.Join(t.TempDir(), "replacement")},
			{Name: "Invalid", RepositoryPath: "relative-repository"},
		},
	}); err == nil {
		t.Fatal("invalid reconciliation unexpectedly succeeded")
	}
	rootAfterFailedScan, err := service.GetGitWorkspaceRootContext(ctx, rootOne.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rootAfterFailedScan.LastScannedAt != rootBeforeFailedScan.LastScannedAt {
		t.Fatal("failed scan changed the previous scan timestamp")
	}
	workspaceOne, err := service.ListGitRepositoriesContext(ctx, GitRepositoryListFilter{WorkspaceID: rootOne.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(workspaceOne) != 2 {
		t.Fatalf("failed scan replaced memberships: %#v", workspaceOne)
	}

	if err := service.ReconcileGitWorkspaceScanContext(ctx, GitWorkspaceScanReconcileRequest{
		WorkspaceID:  rootOne.ID,
		Repositories: []GitRepositoryScanResult{{Name: "Repo one", RepositoryPath: repoOnePath}},
	}); err != nil {
		t.Fatal(err)
	}
	missing, err := service.ListGitRepositoriesContext(ctx, GitRepositoryListFilter{Missing: boolPointer(true)})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0].Name != "Repo two" {
		t.Fatalf("missing repositories = %#v", missing)
	}
}

func TestGitRepositoryMissingPruningStatusAndSettings(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	repoPath := filepath.Join(t.TempDir(), "repo")
	repositories, err := service.UpsertGitRepositoriesContext(ctx, []GitRepositoryUpsert{{
		Name: "Repo", RepositoryPath: repoPath,
	}, {
		Name: "Duplicate path", RepositoryPath: strings.ToUpper(repoPath),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 1 {
		t.Fatalf("upserted repositories = %#v", repositories)
	}
	other, err := service.UpsertGitRepositoriesContext(ctx, []GitRepositoryUpsert{{
		Name: "Other repo", RepositoryPath: filepath.Join(t.TempDir(), "other"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	repositories = append(repositories, other...)
	if err := service.MarkGitRepositoriesMissingContext(ctx, []int{repositories[0].ID}); err != nil {
		t.Fatal(err)
	}
	count, err := service.PruneUnreferencedMissingGitRepositoriesContext(ctx)
	if err != nil || count != 1 {
		t.Fatalf("pruned count/error = %d/%v", count, err)
	}
	if _, err := service.GetGitRepositoryContext(ctx, repositories[0].ID); err == nil {
		t.Fatal("explicitly pruned repository still exists")
	}

	status, err := service.WriteGitRepositoryStatusContext(ctx, GitRepositoryStatusWriteRequest{
		RepositoryID: repositories[1].ID, Branch: "main", Dirty: true,
		ModifiedCount: 2, AddedCount: 1, DeletedCount: 3, RenamedCount: 4,
		UntrackedCount: 5, Upstream: "origin/main", AheadCount: 1, BehindCount: 2,
		SyncState: "diverged", RemoteDisplay: "https://user:secret@example.com/org/repo?token=secret",
		RemoteWebURL: "https://example.com/org/repo", LatestCommitSummary: "Initial commit",
	})
	if err != nil {
		t.Fatal(err)
	}
	if status.RemoteDisplay != "https://example.com/org/repo" || status.RemoteWebURL != "https://example.com/org/repo" || !status.Dirty {
		t.Fatalf("sanitized Git repository status = %#v", status)
	}
	if _, err := service.WriteGitRepositoryStatusContext(ctx, GitRepositoryStatusWriteRequest{
		RepositoryID: repositories[1].ID, RemoteWebURL: "https://user:secret@example.com/repo",
	}); err == nil {
		t.Fatal("credential-bearing remote web URL unexpectedly accepted")
	}

	settings, err := service.GetGitWorkspaceSettingsContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.UpdateGitWorkspaceSettingsContext(ctx, GitWorkspaceSettingsUpdateRequest{
		WorkerCount: 8, StaleDays: 14, ExpectedRevision: settings.Revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != settings.Revision+1 || updated.WorkerCount != 8 || updated.StaleDays != 14 {
		t.Fatalf("updated Git workspace settings = %#v", updated)
	}
	_, err = service.UpdateGitWorkspaceSettingsContext(ctx, GitWorkspaceSettingsUpdateRequest{
		WorkerCount: 16, StaleDays: 30, ExpectedRevision: settings.Revision,
	})
	var stale *StaleRevisionError
	if !errors.As(err, &stale) {
		t.Fatalf("stale settings update error = %v, want StaleRevisionError", err)
	}
}

func TestGitWorkspaceEmptyListsMarshalAsJSONArrays(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	roots, err := service.ListGitWorkspaceRootsContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repositories, err := service.ListGitRepositoriesContext(ctx, GitRepositoryListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]any{"roots": roots, "repositories": repositories} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) != "[]" {
			t.Fatalf("empty %s JSON = %s, want []", name, encoded)
		}
	}
}

func TestGitRepositoryListFilterUsesStableRepositoryIDs(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	repositories, err := service.UpsertGitRepositoriesContext(ctx, []GitRepositoryUpsert{
		{Name: "One", RepositoryPath: filepath.Join(t.TempDir(), "one")},
		{Name: "Two", RepositoryPath: filepath.Join(t.TempDir(), "two")},
	})
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := service.ListGitRepositoriesContext(ctx, GitRepositoryListFilter{RepositoryIDs: []int{repositories[1].ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].ID != repositories[1].ID {
		t.Fatalf("ID-filtered repositories = %#v", filtered)
	}
	if _, err := service.ListGitRepositoriesContext(ctx, GitRepositoryListFilter{RepositoryIDs: []int{0}}); err == nil {
		t.Fatal("zero repository ID was accepted")
	}
}

func boolPointer(value bool) *bool { return &value }
