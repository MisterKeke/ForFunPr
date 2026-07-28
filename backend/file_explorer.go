package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	fileExplorerDefaultPageSize = 250
	fileExplorerMaximumPageSize = 500
	fileExplorerMaximumEntries  = 2000
	fileExplorerSkipBatchSize    = 250
)

type FileExplorerPlace struct {
	RootID string `json:"root_id"`
	Label  string `json:"label"`
	Kind   string `json:"kind"`
}

type FileExplorerDirectoryRequest struct {
	RootID string `json:"root_id"`
	Path   string `json:"path"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

type FileExplorerBreadcrumb struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

type FileExplorerEntry struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Type           string `json:"type"`
	Size           int64  `json:"size"`
	ModifiedAt     string `json:"modified_at,omitempty"`
	IsDirectory    bool   `json:"is_directory"`
	IsSymbolicLink bool   `json:"is_symbolic_link"`
}

type FileExplorerListing struct {
	RootID       string                   `json:"root_id"`
	RootLabel    string                   `json:"root_label"`
	Path         string                   `json:"path"`
	ParentPath   string                   `json:"parent_path"`
	CanGoUp      bool                     `json:"can_go_up"`
	Breadcrumbs  []FileExplorerBreadcrumb `json:"breadcrumbs"`
	Entries      []FileExplorerEntry       `json:"entries"`
	NextOffset   int                      `json:"next_offset"`
	HasMore      bool                     `json:"has_more"`
	Truncated    bool                     `json:"truncated"`
	MaximumItems int                      `json:"maximum_items"`
}

type fileExplorerRoot struct {
	id     string
	label  string
	kind   string
	path   string
}

type fileExplorerRegistry struct {
	mu          sync.RWMutex
	roots       map[string]fileExplorerRoot
	rootsByPath map[string]string
}

func newFileExplorerRegistry() *fileExplorerRegistry {
	return &fileExplorerRegistry{
		roots:       make(map[string]fileExplorerRoot),
		rootsByPath: make(map[string]string),
	}
}

func (r *fileExplorerRegistry) Places(ctx context.Context) ([]FileExplorerPlace, error) {
	if err := r.ensureStandardPlaces(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	places := make([]FileExplorerPlace, 0, len(r.roots))
	for _, root := range r.roots {
		places = append(places, FileExplorerPlace{
			RootID: root.id,
			Label:  root.label,
			Kind:   root.kind,
		})
	}
	r.mu.RUnlock()

	sort.Slice(places, func(i, j int) bool {
		leftOrder := fileExplorerPlaceOrder(places[i].Kind)
		rightOrder := fileExplorerPlaceOrder(places[j].Kind)
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		return strings.ToLower(places[i].Label) < strings.ToLower(places[j].Label)
	})
	return places, nil
}

func (r *fileExplorerRegistry) RegisterChosenRoot(ctx context.Context, path string) (FileExplorerPlace, error) {
	if err := contextError(ctx); err != nil {
		return FileExplorerPlace{}, err
	}
	return r.registerRoot(path, fileExplorerRootLabel(path), "custom")
}

func (r *fileExplorerRegistry) ListDirectory(
	ctx context.Context,
	request FileExplorerDirectoryRequest,
) (*FileExplorerListing, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if request.Offset < 0 || request.Offset >= fileExplorerMaximumEntries {
		return nil, &ValidationError{Field: "offset", Message: "choose a valid directory page"}
	}
	limit := request.Limit
	if limit <= 0 {
		limit = fileExplorerDefaultPageSize
	}
	if limit > fileExplorerMaximumPageSize {
		limit = fileExplorerMaximumPageSize
	}
	if remaining := fileExplorerMaximumEntries - request.Offset; limit > remaining {
		limit = remaining
	}

	root, ok := r.lookupRoot(strings.TrimSpace(request.RootID))
	if !ok {
		return nil, &ValidationError{Field: "root", Message: "choose a valid explorer location"}
	}
	targetPath, relativePath, err := resolveExplorerDirectory(root.path, request.Path)
	if err != nil {
		return nil, err
	}

	directory, err := os.Open(targetPath)
	if err != nil {
		return nil, friendlyExplorerDirectoryError(err)
	}
	defer directory.Close()

	if err := skipExplorerEntries(ctx, directory, request.Offset); err != nil {
		return nil, err
	}
	rawEntries, readErr := directory.ReadDir(limit + 1)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, friendlyExplorerDirectoryError(readErr)
	}
	hasExtraEntry := len(rawEntries) > limit
	if hasExtraEntry {
		rawEntries = rawEntries[:limit]
	}

	entries := make([]FileExplorerEntry, 0, len(rawEntries))
	for _, entry := range rawEntries {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		entries = append(entries, explorerEntry(relativePath, entry))
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDirectory != entries[j].IsDirectory {
			return entries[i].IsDirectory
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	nextOffset := request.Offset + len(entries)
	hasMore := hasExtraEntry && nextOffset < fileExplorerMaximumEntries
	return &FileExplorerListing{
		RootID:       root.id,
		RootLabel:    root.label,
		Path:         relativePath,
		ParentPath:   explorerParentPath(relativePath),
		CanGoUp:      relativePath != "",
		Breadcrumbs:  explorerBreadcrumbs(root.label, relativePath),
		Entries:      entries,
		NextOffset:   nextOffset,
		HasMore:      hasMore,
		Truncated:    hasExtraEntry && !hasMore,
		MaximumItems: fileExplorerMaximumEntries,
	}, nil
}

func (r *fileExplorerRegistry) ensureStandardPlaces(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return errors.New("your home directory is unavailable")
	}
	explorerHome := string(filepath.Separator)
	explorerHomeLabel := "Home"
	if runtime.GOOS == "windows" {
		explorerHome = `C:\`
		explorerHomeLabel = "Home (C:)"
	}
	if _, err := r.registerRoot(explorerHome, explorerHomeLabel, "home"); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		secondaryHome := `D:\`
		if info, statErr := os.Stat(secondaryHome); statErr == nil && info.IsDir() {
			if _, err := r.registerRoot(secondaryHome, "Home 2", "home-secondary"); err != nil {
				return err
			}
		}
	}

	standardPlaces := []struct {
		name string
		kind string
	}{
		{name: "Desktop", kind: "desktop"},
		{name: "Documents", kind: "documents"},
		{name: "Downloads", kind: "downloads"},
	}
	for _, place := range standardPlaces {
		if err := contextError(ctx); err != nil {
			return err
		}
		path := filepath.Join(home, place.name)
		info, statErr := os.Stat(path)
		if statErr != nil || !info.IsDir() {
			continue
		}
		if _, err := r.registerRoot(path, place.name, place.kind); err != nil {
			return err
		}
	}
	return nil
}

func (r *fileExplorerRegistry) registerRoot(
	path string,
	label string,
	kind string,
) (FileExplorerPlace, error) {
	resolvedPath, err := canonicalExplorerRoot(path)
	if err != nil {
		return FileExplorerPlace{}, err
	}
	key := explorerPathKey(resolvedPath)

	r.mu.Lock()
	defer r.mu.Unlock()
	if existingID, ok := r.rootsByPath[key]; ok {
		existing := r.roots[existingID]
		return FileExplorerPlace{RootID: existing.id, Label: existing.label, Kind: existing.kind}, nil
	}

	id, err := newExplorerRootID()
	if err != nil {
		return FileExplorerPlace{}, err
	}
	root := fileExplorerRoot{
		id:     id,
		label:  label,
		kind:   kind,
		path:   resolvedPath,
	}
	r.roots[id] = root
	r.rootsByPath[key] = id
	return FileExplorerPlace{RootID: id, Label: label, Kind: kind}, nil
}

func (r *fileExplorerRegistry) lookupRoot(id string) (fileExplorerRoot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	root, ok := r.roots[id]
	return root, ok
}

func canonicalExplorerRoot(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", &ValidationError{Field: "folder", Message: "choose a folder to browse"}
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", &ValidationError{Field: "folder", Message: "choose a valid folder"}
	}
	resolvedPath, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		return "", friendlyExplorerDirectoryError(err)
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", friendlyExplorerDirectoryError(err)
	}
	if !info.IsDir() {
		return "", &ValidationError{Field: "folder", Message: "the selected location must be a folder"}
	}
	return filepath.Clean(resolvedPath), nil
}

func resolveExplorerDirectory(rootPath string, requestedPath string) (string, string, error) {
	requestedPath = strings.TrimSpace(requestedPath)
	relativePath := filepath.FromSlash(requestedPath)
	if relativePath == "" {
		relativePath = "."
	}
	if filepath.IsAbs(relativePath) || filepath.VolumeName(relativePath) != "" {
		return "", "", &ValidationError{Field: "path", Message: "choose a folder inside this location"}
	}
	relativePath = filepath.Clean(relativePath)
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", "", &ValidationError{Field: "path", Message: "choose a folder inside this location"}
	}

	targetPath := filepath.Join(rootPath, relativePath)
	resolvedPath, err := filepath.EvalSymlinks(targetPath)
	if err != nil {
		return "", "", friendlyExplorerDirectoryError(err)
	}
	if !explorerPathWithinRoot(rootPath, resolvedPath) {
		return "", "", &ValidationError{Field: "path", Message: "that folder is outside the selected location"}
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", "", friendlyExplorerDirectoryError(err)
	}
	if !info.IsDir() {
		return "", "", &ValidationError{Field: "path", Message: "choose a folder to browse"}
	}

	normalizedPath := ""
	if relativePath != "." {
		normalizedPath = filepath.ToSlash(relativePath)
	}
	return resolvedPath, normalizedPath, nil
}

func explorerPathWithinRoot(rootPath string, candidatePath string) bool {
	relativePath, err := filepath.Rel(rootPath, candidatePath)
	if err != nil {
		return false
	}
	return relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator))
}

func skipExplorerEntries(ctx context.Context, directory *os.File, count int) error {
	remaining := count
	for remaining > 0 {
		if err := contextError(ctx); err != nil {
			return err
		}
		batchSize := fileExplorerSkipBatchSize
		if batchSize > remaining {
			batchSize = remaining
		}
		entries, err := directory.ReadDir(batchSize)
		remaining -= len(entries)
		if errors.Is(err, io.EOF) || len(entries) == 0 {
			return nil
		}
		if err != nil {
			return friendlyExplorerDirectoryError(err)
		}
	}
	return nil
}

func explorerEntry(parentPath string, entry os.DirEntry) FileExplorerEntry {
	entryPath := filepath.ToSlash(filepath.Join(filepath.FromSlash(parentPath), entry.Name()))
	if parentPath == "" {
		entryPath = filepath.ToSlash(entry.Name())
	}
	isSymbolicLink := entry.Type()&os.ModeSymlink != 0
	isDirectory := entry.IsDir() && !isSymbolicLink
	result := FileExplorerEntry{
		Name:           entry.Name(),
		Path:           entryPath,
		Type:           explorerEntryType(entry.Name(), isDirectory, isSymbolicLink),
		IsDirectory:    isDirectory,
		IsSymbolicLink: isSymbolicLink,
	}
	if info, err := entry.Info(); err == nil {
		if !isDirectory {
			result.Size = info.Size()
		}
		result.ModifiedAt = info.ModTime().UTC().Format(time.RFC3339)
	}
	return result
}

func explorerEntryType(name string, isDirectory bool, isSymbolicLink bool) string {
	if isSymbolicLink {
		return "Link"
	}
	if isDirectory {
		return "Folder"
	}
	extension := strings.TrimPrefix(filepath.Ext(name), ".")
	if extension == "" {
		return "File"
	}
	return strings.ToUpper(extension) + " file"
}

func explorerParentPath(relativePath string) string {
	if relativePath == "" {
		return ""
	}
	parent := filepath.Dir(filepath.FromSlash(relativePath))
	if parent == "." {
		return ""
	}
	return filepath.ToSlash(parent)
}

func explorerBreadcrumbs(rootLabel string, relativePath string) []FileExplorerBreadcrumb {
	breadcrumbs := []FileExplorerBreadcrumb{{Label: rootLabel, Path: ""}}
	if relativePath == "" {
		return breadcrumbs
	}
	currentPath := ""
	for _, part := range strings.Split(filepath.ToSlash(relativePath), "/") {
		if part == "" {
			continue
		}
		if currentPath == "" {
			currentPath = part
		} else {
			currentPath += "/" + part
		}
		breadcrumbs = append(breadcrumbs, FileExplorerBreadcrumb{Label: part, Path: currentPath})
	}
	return breadcrumbs
}

func fileExplorerRootLabel(path string) string {
	cleanedPath := filepath.Clean(path)
	label := filepath.Base(cleanedPath)
	if label == "." || label == string(filepath.Separator) || label == "" {
		label = filepath.VolumeName(cleanedPath)
	}
	if label == "" {
		return cleanedPath
	}
	return label
}

func fileExplorerPlaceOrder(kind string) int {
	switch kind {
	case "home":
		return 0
	case "home-secondary":
		return 1
	case "desktop":
		return 2
	case "documents":
		return 3
	case "downloads":
		return 4
	default:
		return 5
	}
}

func explorerPathKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func newExplorerRootID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate explorer location ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func contextError(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func friendlyExplorerDirectoryError(err error) error {
	if errors.Is(err, os.ErrPermission) {
		return errors.New("you do not have permission to browse that folder")
	}
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("that folder is no longer available")
	}
	return errors.New("that folder could not be read")
}
