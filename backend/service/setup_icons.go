package service

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"something/backend/storage"
)

const (
	setupIconRoutePrefix  = "/setup-icons/"
	maximumSetupIconBytes = 5 * 1024 * 1024
)

var setupIconFilenamePattern = regexp.MustCompile(`^[a-f0-9]{32}\.(ico|jpg|png|webp)$`)

type preparedSetupIcon struct {
	filename  string
	mimeType  string
	byteSize  int64
	temporary string
	final     string
	installed bool
	kept      bool
}

func prepareSetupIcon(dataURL string) (*preparedSetupIcon, error) {
	dataURL = strings.TrimSpace(dataURL)
	if dataURL == "" {
		return nil, nil
	}

	// Base64 adds roughly one third to the decoded size. Reject an excessive
	// envelope before allocating the decoded byte slice.
	maximumEncodedLength := ((maximumSetupIconBytes + 2) / 3 * 4) + 128
	if len(dataURL) > maximumEncodedLength {
		return nil, &ValidationError{
			Field:   "icon",
			Message: "Setup icons cannot exceed 5 MB.",
		}
	}

	metadata, encoded, found := strings.Cut(dataURL, ",")
	if !found || !strings.HasPrefix(strings.ToLower(metadata), "data:image/") ||
		!strings.HasSuffix(strings.ToLower(metadata), ";base64") {
		return nil, &ValidationError{
			Field:   "icon",
			Message: "The setup icon data is invalid.",
		}
	}

	contents, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, &ValidationError{
			Field:   "icon",
			Message: "The setup icon data is invalid.",
		}
	}
	if len(contents) == 0 || len(contents) > maximumSetupIconBytes {
		return nil, &ValidationError{
			Field:   "icon",
			Message: "Setup icons must be between 1 byte and 5 MB.",
		}
	}

	headerLength := min(len(contents), 512)
	mimeType, extension, err := detectDesktopAppIconFormat(contents[:headerLength])
	if err != nil {
		return nil, &ValidationError{
			Field:   "icon",
			Message: "Choose an ICO, JPEG, PNG, or WebP image.",
		}
	}

	directory, err := storage.SetupIconDirectory()
	if err != nil {
		return nil, err
	}
	id, err := randomDesktopAppIconID()
	if err != nil {
		return nil, fmt.Errorf("generate setup icon ID: %w", err)
	}
	filename := id + extension

	temporary, err := os.CreateTemp(directory, ".setup-icon-upload-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temporary setup icon: %w", err)
	}
	temporaryPath := temporary.Name()
	closeAndRemove := func(cause error) (*preparedSetupIcon, error) {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return nil, cause
	}
	if err := temporary.Chmod(0o600); err != nil {
		return closeAndRemove(fmt.Errorf("secure temporary setup icon: %w", err))
	}
	written, err := temporary.Write(contents)
	if err != nil {
		return closeAndRemove(fmt.Errorf("write temporary setup icon: %w", err))
	}
	if written != len(contents) {
		return closeAndRemove(fmt.Errorf("write temporary setup icon: short write"))
	}
	if err := temporary.Sync(); err != nil {
		return closeAndRemove(fmt.Errorf("flush temporary setup icon: %w", err))
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return nil, fmt.Errorf("close temporary setup icon: %w", err)
	}

	return &preparedSetupIcon{
		filename:  filename,
		mimeType:  mimeType,
		byteSize:  int64(len(contents)),
		temporary: temporaryPath,
		final:     filepath.Join(directory, filename),
	}, nil
}

func (icon *preparedSetupIcon) install() error {
	if icon == nil {
		return nil
	}
	if err := os.Rename(icon.temporary, icon.final); err != nil {
		return fmt.Errorf("install setup icon: %w", err)
	}
	icon.installed = true
	return nil
}

func (icon *preparedSetupIcon) keep() {
	if icon != nil {
		icon.kept = true
	}
}

func (icon *preparedSetupIcon) cleanup() {
	if icon == nil || icon.kept {
		return
	}
	_ = os.Remove(icon.temporary)
	if icon.installed {
		_ = os.Remove(icon.final)
	}
}

type stagedSetupIcon struct {
	originalPath string
	stagedPath   string
	moved        bool
}

func stageSetupIconDeletion(filename string) (*stagedSetupIcon, error) {
	if filename == "" {
		return nil, nil
	}
	if !setupIconFilenamePattern.MatchString(filename) {
		return nil, fmt.Errorf("stored setup icon filename is invalid")
	}
	directory, err := storage.SetupIconDirectory()
	if err != nil {
		return nil, err
	}
	stagedID, err := randomDesktopAppIconID()
	if err != nil {
		return nil, fmt.Errorf("generate staged setup icon ID: %w", err)
	}
	staged := &stagedSetupIcon{
		originalPath: filepath.Join(directory, filename),
		stagedPath:   filepath.Join(directory, "."+stagedID+".deleting"),
	}
	if err := os.Rename(staged.originalPath, staged.stagedPath); err == nil {
		staged.moved = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stage setup icon deletion: %w", err)
	}
	return staged, nil
}

func (staged *stagedSetupIcon) restore() {
	if staged != nil && staged.moved {
		_ = os.Rename(staged.stagedPath, staged.originalPath)
	}
}

func (staged *stagedSetupIcon) finish() {
	if staged != nil && staged.moved {
		_ = os.Remove(staged.stagedPath)
	}
}

func removeSetupIconFile(filename string) {
	if !setupIconFilenamePattern.MatchString(filename) {
		return
	}
	directory, err := storage.SetupIconDirectory()
	if err != nil {
		return
	}
	_ = os.Remove(filepath.Join(directory, filename))
}

func setupIconURL(filename string) string {
	if !setupIconFilenamePattern.MatchString(filename) {
		return ""
	}
	directory, err := storage.SetupIconDirectory()
	if err != nil {
		return ""
	}
	info, err := os.Stat(filepath.Join(directory, filename))
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return setupIconRoutePrefix + filename
}

func NewSetupIconHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.URL.Path, setupIconRoutePrefix) {
			http.NotFound(w, r)
			return
		}
		filename := strings.TrimPrefix(r.URL.Path, setupIconRoutePrefix)
		if !setupIconFilenamePattern.MatchString(filename) {
			http.NotFound(w, r)
			return
		}
		directory, err := storage.SetupIconDirectory()
		if err != nil {
			http.Error(w, "setup icon storage unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, filepath.Join(directory, filename))
	})
}
