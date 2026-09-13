package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	maximumLegacyGitWorkspaceConfigBytes        = 1 << 20
	maximumLegacyGitWorkspaceConfigWorkspaces   = 1024
	maximumLegacyGitWorkspaceConfigRepositories = 10000
)

// LegacyGitWorkspaceImportRoot describes one legacy workspace root during the
// preview. RootPath is exposed only through the Wails facade.
type LegacyGitWorkspaceImportRoot struct {
	DisplayName string `json:"display_name"`
	RootPath    string `json:"root_path"`
	Status      string `json:"status"`
	Reason      string `json:"reason,omitempty"`
}

// LegacyGitWorkspaceImportPreview is read-only and safe to show before the
// user confirms the one-time copy.
type LegacyGitWorkspaceImportPreview struct {
	Source                 string                         `json:"source"`
	EditorSuggestion       string                         `json:"editor_suggestion,omitempty"`
	WorkspaceCount         int                            `json:"workspace_count"`
	RepositoryCount        int                            `json:"repository_count"`
	ImportableRootCount    int                            `json:"importable_root_count"`
	DuplicateRootCount     int                            `json:"duplicate_root_count"`
	MissingRootCount       int                            `json:"missing_root_count"`
	InvalidRootCount       int                            `json:"invalid_root_count"`
	SkippedRootCount       int                            `json:"skipped_root_count"`
	SkippedRepositoryCount int                            `json:"skipped_repository_count"`
	ScanErrorCount         int                            `json:"scan_error_count"`
	Roots                  []LegacyGitWorkspaceImportRoot `json:"roots"`
}

// LegacyGitWorkspaceImportResult reports the outcome after the user confirms
// the preview. Imported roots are Something-owned after this call.
type LegacyGitWorkspaceImportResult struct {
	Preview                 LegacyGitWorkspaceImportPreview `json:"preview"`
	ImportedRootCount       int                             `json:"imported_root_count"`
	ImportedRepositoryCount int                             `json:"imported_repository_count"`
	DuplicateRootCount      int                             `json:"duplicate_root_count"`
	MissingRootCount        int                             `json:"missing_root_count"`
	InvalidRootCount        int                             `json:"invalid_root_count"`
	SkippedRootCount        int                             `json:"skipped_root_count"`
	SkippedRepositoryCount  int                             `json:"skipped_repository_count"`
	InvalidRepositoryCount  int                             `json:"invalid_repository_count"`
	ScanErrorCount          int                             `json:"scan_error_count"`
}

type legacyGitWorkspaceConfig struct {
	Workspaces   []string                       `json:"workspaces"`
	Repositories []legacyGitWorkspaceRepository `json:"repositories"`
	Editor       string                         `json:"editor,omitempty"`
}

type legacyGitWorkspaceRepository struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// DetectLegacyGitWorkspaceConfigContext previews the standard
// <user-home>/.gw/config.json file without changing Something's database.
func (a *Service) DetectLegacyGitWorkspaceConfigContext(ctx context.Context) (LegacyGitWorkspaceImportPreview, error) {
	path, err := legacyGitWorkspaceConfigPath()
	if err != nil {
		return LegacyGitWorkspaceImportPreview{}, &ValidationError{Field: "legacy_config", Message: "The legacy GitWorkspaceFun configuration location is unavailable."}
	}
	return a.DetectLegacyGitWorkspaceConfigAtContext(ctx, path)
}

// DetectLegacyGitWorkspaceConfigAtContext is the service seam used by the
// native picker bridge. Callers outside the bridge must not pass user input
// directly to this method.
func (a *Service) DetectLegacyGitWorkspaceConfigAtContext(ctx context.Context, path string) (LegacyGitWorkspaceImportPreview, error) {
	cfg, err := readLegacyGitWorkspaceConfig(path)
	if err != nil {
		return LegacyGitWorkspaceImportPreview{}, err
	}
	return a.previewLegacyGitWorkspaceConfig(ctx, cfg, legacyGitWorkspaceSource(path))
}

// ImportLegacyGitWorkspaceConfigContext imports the standard legacy file after
// the UI has shown its preview and obtained confirmation.
func (a *Service) ImportLegacyGitWorkspaceConfigContext(ctx context.Context) (LegacyGitWorkspaceImportResult, error) {
	path, err := legacyGitWorkspaceConfigPath()
	if err != nil {
		return LegacyGitWorkspaceImportResult{}, &ValidationError{Field: "legacy_config", Message: "The legacy GitWorkspaceFun configuration location is unavailable."}
	}
	return a.ImportLegacyGitWorkspaceConfigAtContext(ctx, path)
}

// ImportLegacyGitWorkspaceConfigAtContext imports a native-picker-selected
// file. The file is read and closed before any database or Git work begins.
func (a *Service) ImportLegacyGitWorkspaceConfigAtContext(ctx context.Context, path string) (LegacyGitWorkspaceImportResult, error) {
	cfg, err := readLegacyGitWorkspaceConfig(path)
	if err != nil {
		return LegacyGitWorkspaceImportResult{}, err
	}
	preview, err := a.previewLegacyGitWorkspaceConfig(ctx, cfg, legacyGitWorkspaceSource(path))
	if err != nil {
		return LegacyGitWorkspaceImportResult{}, err
	}

	result := LegacyGitWorkspaceImportResult{
		Preview:                preview,
		DuplicateRootCount:     preview.DuplicateRootCount,
		MissingRootCount:       preview.MissingRootCount,
		InvalidRootCount:       preview.InvalidRootCount,
		SkippedRootCount:       preview.SkippedRootCount,
		SkippedRepositoryCount: preview.SkippedRepositoryCount,
	}
	for _, candidate := range preview.Roots {
		if candidate.Status != "importable" {
			continue
		}
		root, err := a.CreateGitWorkspaceRootContext(ctx, GitWorkspaceRootCreateRequest{
			DisplayName: candidate.DisplayName,
			RootPath:    candidate.RootPath,
		})
		if err != nil {
			var conflict *ConflictError
			if errors.As(err, &conflict) {
				result.DuplicateRootCount++
				result.SkippedRootCount++
				continue
			}
			return result, err
		}
		result.ImportedRootCount++

		provider, providerErr := a.gitWorkspaceProviderOrError()
		if providerErr != nil {
			result.ScanErrorCount++
			continue
		}
		scan, scanErr := provider.Scan(ctx, root.RootPath)
		if scanErr != nil {
			// Do not import a partial provider result. The root itself is a
			// valid user-confirmed record, while its repository membership must
			// remain empty until a complete scan succeeds.
			result.ScanErrorCount++
			continue
		}
		validRepositories := make([]GitRepositoryScanResult, 0, len(scan.Repositories))
		for _, repository := range scan.Repositories {
			name, _, _, normalizeErr := normalizeGitRepositoryWrite(repository.Name, repository.Path)
			if normalizeErr != nil {
				result.InvalidRepositoryCount++
				continue
			}
			validRepositories = append(validRepositories, GitRepositoryScanResult{Name: name, RepositoryPath: repository.Path})
		}
		if len(validRepositories) > 0 {
			if err := a.ReconcileGitWorkspaceScanContext(ctx, GitWorkspaceScanReconcileRequest{
				WorkspaceID: root.ID, Repositories: validRepositories, ExpectedRevision: &root.Revision,
			}); err != nil {
				return result, err
			}
			result.ImportedRepositoryCount += countUniqueGitRepositoryPaths(validRepositories)
		}
	}
	if result.ImportedRootCount > 0 {
		a.emitGitWorkspaceInventoryChanged()
	}
	return result, nil
}

func (a *Service) previewLegacyGitWorkspaceConfig(ctx context.Context, cfg legacyGitWorkspaceConfig, source string) (LegacyGitWorkspaceImportPreview, error) {
	roots, err := a.ListGitWorkspaceRootsContext(ctx)
	if err != nil {
		return LegacyGitWorkspaceImportPreview{}, err
	}
	existing := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		existing[root.RootPathKey] = struct{}{}
	}

	preview := LegacyGitWorkspaceImportPreview{
		Source:                 source,
		EditorSuggestion:       strings.TrimSpace(cfg.Editor),
		WorkspaceCount:         len(cfg.Workspaces),
		RepositoryCount:        len(cfg.Repositories),
		SkippedRepositoryCount: len(cfg.Repositories),
		Roots:                  make([]LegacyGitWorkspaceImportRoot, 0, len(cfg.Workspaces)),
	}
	seen := make(map[string]struct{}, len(cfg.Workspaces))
	for _, rawPath := range cfg.Workspaces {
		candidate := LegacyGitWorkspaceImportRoot{RootPath: strings.TrimSpace(rawPath)}
		path, pathKey, normalizeErr := normalizeGitAbsolutePath("workspace_path", rawPath)
		if normalizeErr != nil {
			candidate.Status = "invalid"
			candidate.Reason = normalizeErr.Error()
			preview.InvalidRootCount++
			preview.SkippedRootCount++
			preview.Roots = append(preview.Roots, candidate)
			continue
		}
		candidate.RootPath = path
		candidate.DisplayName = filepath.Base(path)
		if candidate.DisplayName == "" || candidate.DisplayName == "." || candidate.DisplayName == string(filepath.Separator) {
			candidate.Status = "invalid"
			candidate.Reason = "workspace path does not have a usable name"
			preview.InvalidRootCount++
			preview.SkippedRootCount++
			preview.Roots = append(preview.Roots, candidate)
			continue
		}
		if _, duplicate := seen[pathKey]; duplicate {
			candidate.Status = "duplicate"
			candidate.Reason = "duplicate in the legacy configuration"
			preview.DuplicateRootCount++
			preview.SkippedRootCount++
			preview.Roots = append(preview.Roots, candidate)
			continue
		}
		seen[pathKey] = struct{}{}
		if _, duplicate := existing[pathKey]; duplicate {
			candidate.Status = "duplicate"
			candidate.Reason = "already tracked by Something"
			preview.DuplicateRootCount++
			preview.SkippedRootCount++
			preview.Roots = append(preview.Roots, candidate)
			continue
		}
		info, statErr := os.Stat(path)
		switch {
		case errors.Is(statErr, os.ErrNotExist):
			candidate.Status = "missing"
			candidate.Reason = "workspace folder does not exist"
			preview.MissingRootCount++
			preview.SkippedRootCount++
		case statErr != nil:
			candidate.Status = "invalid"
			candidate.Reason = "workspace folder could not be inspected"
			preview.InvalidRootCount++
			preview.SkippedRootCount++
		case !info.IsDir():
			candidate.Status = "invalid"
			candidate.Reason = "workspace path is not a folder"
			preview.InvalidRootCount++
			preview.SkippedRootCount++
		default:
			candidate.Status = "importable"
			preview.ImportableRootCount++
		}
		preview.Roots = append(preview.Roots, candidate)
	}
	return preview, nil
}

func readLegacyGitWorkspaceConfig(path string) (legacyGitWorkspaceConfig, error) {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(filepath.Clean(path)) {
		return legacyGitWorkspaceConfig{}, &ValidationError{Field: "legacy_config", Message: "The legacy configuration file must be selected by the native file picker."}
	}
	file, err := os.Open(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return legacyGitWorkspaceConfig{}, &NotFoundError{Resource: "GitWorkspaceFun configuration", Key: "selected file"}
	}
	if err != nil {
		return legacyGitWorkspaceConfig{}, &ValidationError{Field: "legacy_config", Message: "The legacy GitWorkspaceFun configuration could not be opened."}
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maximumLegacyGitWorkspaceConfigBytes+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return legacyGitWorkspaceConfig{}, &ValidationError{Field: "legacy_config", Message: "The legacy GitWorkspaceFun configuration could not be read."}
	}
	if len(data) > maximumLegacyGitWorkspaceConfigBytes {
		return legacyGitWorkspaceConfig{}, &ValidationError{Field: "legacy_config", Message: "The legacy GitWorkspaceFun configuration is limited to 1 MiB."}
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return legacyGitWorkspaceConfig{}, &ValidationError{Field: "legacy_config", Message: "The legacy GitWorkspaceFun configuration must be one JSON object."}
	}
	if err := validateLegacyGitWorkspaceJSONTypes(object); err != nil {
		return legacyGitWorkspaceConfig{}, err
	}
	var cfg legacyGitWorkspaceConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return legacyGitWorkspaceConfig{}, &ValidationError{Field: "legacy_config", Message: "The legacy GitWorkspaceFun configuration has invalid fields or types."}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return legacyGitWorkspaceConfig{}, &ValidationError{Field: "legacy_config", Message: "The legacy GitWorkspaceFun configuration must contain one JSON object."}
	}
	for _, field := range []string{"workspaces", "repositories", "editor"} {
		if raw, ok := object[field]; ok && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return legacyGitWorkspaceConfig{}, &ValidationError{Field: field, Message: "legacy configuration fields cannot be null"}
		}
	}
	if len(cfg.Workspaces) > maximumLegacyGitWorkspaceConfigWorkspaces {
		return legacyGitWorkspaceConfig{}, &ValidationError{Field: "workspaces", Message: "legacy configuration contains too many workspace roots"}
	}
	if len(cfg.Repositories) > maximumLegacyGitWorkspaceConfigRepositories {
		return legacyGitWorkspaceConfig{}, &ValidationError{Field: "repositories", Message: "legacy configuration contains too many repositories"}
	}
	for index, repository := range cfg.Repositories {
		if utf8.RuneCountInString(repository.Name) > maximumGitRepositoryNameLength || utf8.RuneCountInString(repository.Path) > maximumLegacyGitWorkspaceConfigBytes {
			return legacyGitWorkspaceConfig{}, &ValidationError{Field: fmt.Sprintf("repositories[%d]", index), Message: "legacy repository fields are too long"}
		}
	}
	return cfg, nil
}

func validateLegacyGitWorkspaceJSONTypes(object map[string]json.RawMessage) error {
	if raw, ok := object["workspaces"]; ok {
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil || values == nil {
			return &ValidationError{Field: "workspaces", Message: "legacy workspace roots must be an array of strings"}
		}
		for _, value := range values {
			var path string
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) || json.Unmarshal(value, &path) != nil {
				return &ValidationError{Field: "workspaces", Message: "legacy workspace roots must be an array of strings"}
			}
		}
	}
	if raw, ok := object["repositories"]; ok {
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil || values == nil {
			return &ValidationError{Field: "repositories", Message: "legacy repositories must be an array of objects"}
		}
		for _, value := range values {
			var repository map[string]json.RawMessage
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) || json.Unmarshal(value, &repository) != nil || repository == nil {
				return &ValidationError{Field: "repositories", Message: "legacy repositories must be an array of objects"}
			}
			for _, field := range []string{"name", "path"} {
				if rawField, present := repository[field]; present {
					var text string
					if bytes.Equal(bytes.TrimSpace(rawField), []byte("null")) || json.Unmarshal(rawField, &text) != nil {
						return &ValidationError{Field: "repositories." + field, Message: "legacy repository fields must be strings"}
					}
				}
			}
		}
	}
	if raw, ok := object["editor"]; ok {
		var editor string
		if json.Unmarshal(raw, &editor) != nil {
			return &ValidationError{Field: "editor", Message: "legacy editor must be a string"}
		}
	}
	return nil
}

func legacyGitWorkspaceConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gw", "config.json"), nil
}

func legacyGitWorkspaceSource(path string) string {
	if standard, err := legacyGitWorkspaceConfigPath(); err == nil && strings.EqualFold(filepath.Clean(path), filepath.Clean(standard)) {
		return "standard .gw/config.json"
	}
	return "selected config.json"
}

func countUniqueGitRepositoryPaths(repositories []GitRepositoryScanResult) int {
	seen := make(map[string]struct{}, len(repositories))
	for _, repository := range repositories {
		_, _, key, err := normalizeGitRepositoryWrite(repository.Name, repository.RepositoryPath)
		if err == nil {
			seen[key] = struct{}{}
		}
	}
	return len(seen)
}
