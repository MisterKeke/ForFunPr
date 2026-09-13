package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"something/backend/gitworkspace"
)

func writeLegacyGitWorkspaceConfig(t *testing.T, path string, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDetectLegacyGitWorkspaceConfigRejectsMissingInvalidAndOversizedFiles(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()

	_, err := service.DetectLegacyGitWorkspaceConfigAtContext(ctx, filepath.Join(t.TempDir(), "missing.json"))
	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("missing config error = %v, want NotFoundError", err)
	}

	invalidPath := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalidPath, []byte(`{"workspaces":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DetectLegacyGitWorkspaceConfigAtContext(ctx, invalidPath); err == nil {
		t.Fatal("invalid JSON unexpectedly accepted")
	}

	oversizedPath := filepath.Join(t.TempDir(), "oversized.json")
	if err := os.WriteFile(oversizedPath, []byte(strings.Repeat("x", maximumLegacyGitWorkspaceConfigBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DetectLegacyGitWorkspaceConfigAtContext(ctx, oversizedPath); err == nil {
		t.Fatal("oversized config unexpectedly accepted")
	}

	trailingPath := filepath.Join(t.TempDir(), "trailing.json")
	if err := os.WriteFile(trailingPath, []byte(`{} {}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DetectLegacyGitWorkspaceConfigAtContext(ctx, trailingPath); err == nil {
		t.Fatal("multiple JSON values unexpectedly accepted")
	}
}

func TestDetectLegacyGitWorkspaceConfigUsesStandardHomeLocation(t *testing.T) {
	service := newFeatureTestService(t)
	home := t.TempDir()
	standardPath := filepath.Join(home, ".gw", "config.json")
	if err := os.MkdirAll(filepath.Dir(standardPath), 0o700); err != nil {
		t.Fatal(err)
	}
	writeLegacyGitWorkspaceConfig(t, standardPath, map[string]any{"workspaces": []string{}, "repositories": []any{}})
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	} else {
		t.Setenv("HOME", home)
	}
	preview, err := service.DetectLegacyGitWorkspaceConfigContext(context.Background())
	if err != nil || preview.Source != "standard .gw/config.json" {
		t.Fatalf("standard config preview = %#v, err=%v", preview, err)
	}
}

func TestDetectLegacyGitWorkspaceConfigEnforcesWorkspaceAndRepositoryLimits(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	tooManyWorkspaces := make([]string, maximumLegacyGitWorkspaceConfigWorkspaces+1)
	workspaceBase := filepath.Join(t.TempDir(), "workspace")
	for index := range tooManyWorkspaces {
		tooManyWorkspaces[index] = fmt.Sprintf("%s-%d", workspaceBase, index)
	}
	workspaceConfig := filepath.Join(t.TempDir(), "too-many-workspaces.json")
	writeLegacyGitWorkspaceConfig(t, workspaceConfig, map[string]any{"workspaces": tooManyWorkspaces, "repositories": []any{}})
	if _, err := service.DetectLegacyGitWorkspaceConfigAtContext(ctx, workspaceConfig); err == nil {
		t.Fatal("workspace count limit was not enforced")
	}

	tooManyRepositories := make([]map[string]string, maximumLegacyGitWorkspaceConfigRepositories+1)
	repositoryBase := filepath.Join(t.TempDir(), "repository")
	for index := range tooManyRepositories {
		tooManyRepositories[index] = map[string]string{"name": "repository", "path": fmt.Sprintf("%s-%d", repositoryBase, index)}
	}
	repositoryConfig := filepath.Join(t.TempDir(), "too-many-repositories.json")
	writeLegacyGitWorkspaceConfig(t, repositoryConfig, map[string]any{"workspaces": []string{}, "repositories": tooManyRepositories})
	if _, err := service.DetectLegacyGitWorkspaceConfigAtContext(ctx, repositoryConfig); err == nil {
		t.Fatal("repository count limit was not enforced")
	}
}

func TestLegacyGitWorkspaceImportPreviewsDuplicatesMissingInvalidAndPartialScan(t *testing.T) {
	root := t.TempDir()
	notDirectory := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(notDirectory, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	repositoryPath := filepath.Join(t.TempDir(), "repository")
	configPath := filepath.Join(t.TempDir(), "config.json")
	original := writeLegacyGitWorkspaceConfig(t, configPath, map[string]any{
		"workspaces":   []string{root, root, missing, "relative-root", notDirectory},
		"repositories": []map[string]string{{"name": "legacy", "path": repositoryPath}},
		"editor":       `C:\\Users\\Public\\malicious.exe --run-this`,
	})
	provider := &fakeGitWorkspaceProvider{scan: func(context.Context, string) (gitworkspace.ScanResult, error) {
		return gitworkspace.ScanResult{Repositories: []gitworkspace.Repository{
			{Name: "discovered", Path: repositoryPath},
			{Name: "invalid", Path: "relative-repository"},
		}}, errors.New("one repository could not be inspected")
	}}
	service := newGitJobTestService(t, provider)
	ctx := context.Background()

	preview, err := service.DetectLegacyGitWorkspaceConfigAtContext(ctx, configPath)
	if err != nil {
		t.Fatal(err)
	}
	if preview.ImportableRootCount != 1 || preview.DuplicateRootCount != 1 || preview.MissingRootCount != 1 || preview.InvalidRootCount != 2 {
		t.Fatalf("legacy preview counts = %#v", preview)
	}
	if preview.EditorSuggestion != `C:\\Users\\Public\\malicious.exe --run-this` {
		t.Fatalf("editor suggestion = %q", preview.EditorSuggestion)
	}
	if roots, err := service.ListGitWorkspaceRootsContext(ctx); err != nil || len(roots) != 0 {
		t.Fatalf("preview changed stored roots: %#v, err=%v", roots, err)
	}

	result, err := service.ImportLegacyGitWorkspaceConfigAtContext(ctx, configPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.ImportedRootCount != 1 || result.ImportedRepositoryCount != 0 || result.InvalidRepositoryCount != 0 || result.SkippedRootCount != 4 || result.ScanErrorCount != 1 {
		t.Fatalf("legacy import result = %#v", result)
	}
	if result.SkippedRepositoryCount != 1 {
		t.Fatalf("skipped legacy repositories = %d", result.SkippedRepositoryCount)
	}
	settings, err := service.GetGitWorkspaceSettingsContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if settings.EditorApplicationID != nil {
		t.Fatalf("legacy editor was automatically mapped: %#v", settings.EditorApplicationID)
	}
	if got, err := os.ReadFile(configPath); err != nil || string(got) != string(original) {
		t.Fatalf("legacy config changed after import: err=%v bytesEqual=%v", err, string(got) == string(original))
	}
	roots, err := service.ListGitWorkspaceRootsContext(ctx)
	if err != nil || len(roots) != 1 || roots[0].RootPath != filepath.Clean(root) {
		t.Fatalf("stored roots = %#v, err=%v", roots, err)
	}
	repositories, err := service.ListGitRepositoriesContext(ctx, GitRepositoryListFilter{WorkspaceID: roots[0].ID})
	if err != nil || len(repositories) != 0 {
		t.Fatalf("partial scan imported repositories = %#v, err=%v", repositories, err)
	}
}

func TestLegacyGitWorkspaceImportDoesNotOverwriteExistingRoots(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	existingPath := t.TempDir()
	newPath := t.TempDir()
	existing, err := service.CreateGitWorkspaceRootContext(ctx, GitWorkspaceRootCreateRequest{DisplayName: "Original name", RootPath: existingPath})
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	writeLegacyGitWorkspaceConfig(t, configPath, map[string]any{
		"workspaces":   []string{filepath.Join(existingPath, "."), newPath},
		"repositories": []any{},
	})
	result, err := service.ImportLegacyGitWorkspaceConfigAtContext(ctx, configPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.DuplicateRootCount != 1 || result.ImportedRootCount != 1 {
		t.Fatalf("existing root import counts = %#v", result)
	}
	loaded, err := service.GetGitWorkspaceRootContext(ctx, existing.ID)
	if err != nil || loaded.DisplayName != "Original name" {
		t.Fatalf("existing root was overwritten: %#v, err=%v", loaded, err)
	}
}

func TestLegacyGitWorkspaceImportRejectsRelativePathAndUnknownOrWrongTypes(t *testing.T) {
	service := newFeatureTestService(t)
	ctx := context.Background()
	for name, value := range map[string]any{
		"relative config path": filepath.Join("relative", "config.json"),
		"unknown field":        filepath.Join(t.TempDir(), "unknown.json"),
		"wrong field type":     filepath.Join(t.TempDir(), "wrong.json"),
	} {
		switch name {
		case "relative config path":
			if _, err := service.DetectLegacyGitWorkspaceConfigAtContext(ctx, value.(string)); err == nil {
				t.Fatal("relative config path unexpectedly accepted")
			}
		case "unknown field":
			writeLegacyGitWorkspaceConfig(t, value.(string), map[string]any{"workspaces": []string{}, "repositories": []any{}, "unexpected": true})
			if _, err := service.DetectLegacyGitWorkspaceConfigAtContext(ctx, value.(string)); err == nil {
				t.Fatal("unknown field unexpectedly accepted")
			}
		case "wrong field type":
			writeLegacyGitWorkspaceConfig(t, value.(string), map[string]any{"workspaces": "not-an-array", "repositories": []any{}})
			if _, err := service.DetectLegacyGitWorkspaceConfigAtContext(ctx, value.(string)); err == nil {
				t.Fatal("wrong field type unexpectedly accepted")
			}
		}
	}
}
