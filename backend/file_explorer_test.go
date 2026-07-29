package backend

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type recordingFileExplorerShell struct {
	openedPath   string
	recycledPath string
}

func (s *recordingFileExplorerShell) OpenFile(path string) error {
	s.openedPath = path
	return nil
}

func (s *recordingFileExplorerShell) RecycleFile(path string) error {
	s.recycledPath = path
	return nil
}

func TestFileExplorerSearchStaysInCurrentDirectory(t *testing.T) {
	rootPath := t.TempDir()
	writeExplorerTestFile(t, filepath.Join(rootPath, "report-final.txt"))
	writeExplorerTestFile(t, filepath.Join(rootPath, "notes.txt"))
	if err := os.Mkdir(filepath.Join(rootPath, "Reports"), 0o700); err != nil {
		t.Fatalf("create matching folder: %v", err)
	}
	if err := os.Mkdir(filepath.Join(rootPath, "nested"), 0o700); err != nil {
		t.Fatalf("create nested folder: %v", err)
	}
	writeExplorerTestFile(t, filepath.Join(rootPath, "nested", "report-hidden.txt"))

	registry := newFileExplorerRegistry()
	place, err := registry.RegisterChosenRoot(context.Background(), rootPath)
	if err != nil {
		t.Fatalf("register explorer root: %v", err)
	}
	listing, err := registry.ListDirectory(context.Background(), FileExplorerDirectoryRequest{
		RootID: place.RootID,
		Query:  "REPORT",
		Limit:  fileExplorerDefaultPageSize,
	})
	if err != nil {
		t.Fatalf("search current directory: %v", err)
	}
	if len(listing.Entries) != 2 {
		t.Fatalf("expected two current-directory matches, got %d", len(listing.Entries))
	}
	if listing.Entries[0].Name != "Reports" || listing.Entries[1].Name != "report-final.txt" {
		t.Fatalf("unexpected matches: %#v", listing.Entries)
	}
	if listing.Query != "REPORT" {
		t.Fatalf("expected normalized query in listing, got %q", listing.Query)
	}
}

func TestFileExplorerFileActionsUseValidatedPath(t *testing.T) {
	rootPath := t.TempDir()
	filePath := filepath.Join(rootPath, "document.txt")
	writeExplorerTestFile(t, filePath)

	shell := &recordingFileExplorerShell{}
	registry := newFileExplorerRegistry()
	registry.shell = shell
	place, err := registry.RegisterChosenRoot(context.Background(), rootPath)
	if err != nil {
		t.Fatalf("register explorer root: %v", err)
	}
	request := FileExplorerFileRequest{RootID: place.RootID, Path: "document.txt"}
	if err := registry.OpenFile(context.Background(), request); err != nil {
		t.Fatalf("open explorer file: %v", err)
	}
	if shell.openedPath != filePath {
		t.Fatalf("opened path %q, want %q", shell.openedPath, filePath)
	}
	if err := registry.DeleteFile(context.Background(), request); err != nil {
		t.Fatalf("recycle explorer file: %v", err)
	}
	if shell.recycledPath != filePath {
		t.Fatalf("recycled path %q, want %q", shell.recycledPath, filePath)
	}
}

func TestFileExplorerFileActionRejectsTraversalAndDirectories(t *testing.T) {
	rootPath := t.TempDir()
	registry := newFileExplorerRegistry()
	registry.shell = &recordingFileExplorerShell{}
	place, err := registry.RegisterChosenRoot(context.Background(), rootPath)
	if err != nil {
		t.Fatalf("register explorer root: %v", err)
	}

	requests := []FileExplorerFileRequest{
		{RootID: place.RootID, Path: "../outside.txt"},
		{RootID: place.RootID, Path: "."},
	}
	for _, request := range requests {
		err := registry.OpenFile(context.Background(), request)
		var validationError *ValidationError
		if !errors.As(err, &validationError) {
			t.Fatalf("request %#v returned %v, want validation error", request, err)
		}
	}
}

func writeExplorerTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
		t.Fatalf("write explorer test file: %v", err)
	}
}
