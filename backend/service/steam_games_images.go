package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"something/backend/storage"
)

const (
	steamGameImageRoutePrefix  = "/steam-game-images/"
	maximumSteamGameImageBytes = externalMaxResponseBytes
)

var (
	steamGameImageFilenamePattern = regexp.MustCompile(`^[1-9][0-9]*-[a-f0-9]{64}\.(jpg|png|webp)$`)
	steamGameImageExtensions       = map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
	}
)

func normalizeSteamArtworkURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	allowed := host == "steamstatic.com" ||
		strings.HasSuffix(host, ".steamstatic.com") ||
		host == "steamcdn-a.akamaihd.net"
	if !allowed {
		return ""
	}
	return parsed.String()
}

func sameSteamArtworkAsset(left string, right string) bool {
	left = normalizeSteamArtworkURL(left)
	right = normalizeSteamArtworkURL(right)
	if left == "" || right == "" {
		return false
	}
	leftURL, leftErr := url.Parse(left)
	rightURL, rightErr := url.Parse(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return strings.EqualFold(leftURL.Scheme, rightURL.Scheme) &&
		strings.EqualFold(leftURL.Host, rightURL.Host) &&
		leftURL.EscapedPath() == rightURL.EscapedPath()
}

func detectSteamGameImageFormat(body []byte) (string, string, error) {
	if len(body) == 0 {
		return "", "", fmt.Errorf("Steam artwork response was empty")
	}
	header := body
	if len(header) > 512 {
		header = header[:512]
	}
	mimeType := http.DetectContentType(header)
	if len(body) >= 12 && bytes.Equal(body[:4], []byte("RIFF")) && bytes.Equal(body[8:12], []byte("WEBP")) {
		mimeType = "image/webp"
	}
	extension, supported := steamGameImageExtensions[mimeType]
	if !supported {
		return "", "", fmt.Errorf("Steam artwork used unsupported content type %q", mimeType)
	}
	return mimeType, extension, nil
}

func (a *Service) cacheSteamGameImageContext(
	ctx context.Context,
	gameID int,
	steamAppID uint32,
	sourceURL string,
) error {
	sourceURL = normalizeSteamArtworkURL(sourceURL)
	if sourceURL == "" {
		return nil
	}
	requestURL, err := url.Parse(sourceURL)
	if err != nil {
		return fmt.Errorf("parse Steam artwork URL: %w", err)
	}
	headers := make(http.Header)
	headers.Set("Accept", "image/avif,image/webp,image/png,image/jpeg,*/*;q=0.5")
	headers.Set("User-Agent", "Something/1.0")
	body, _, err := a.httpClient.get(
		ctx,
		providerSteamImages,
		requestURL,
		headers,
		http.StatusOK,
	)
	if err != nil {
		return err
	}
	if len(body) == 0 || int64(len(body)) > maximumSteamGameImageBytes {
		return fmt.Errorf("Steam artwork must be between 1 byte and %d bytes", maximumSteamGameImageBytes)
	}
	mimeType, extension, err := detectSteamGameImageFormat(body)
	if err != nil {
		return err
	}

	digest := sha256.Sum256(body)
	filename := fmt.Sprintf("%d-%x%s", steamAppID, digest, extension)
	if !steamGameImageFilenamePattern.MatchString(filename) {
		return fmt.Errorf("generated Steam artwork filename is invalid")
	}
	directory, err := storage.SteamGameImageDirectory()
	if err != nil {
		return err
	}
	destinationPath := filepath.Join(directory, filename)
	installedNewFile := false
	if info, statErr := os.Stat(destinationPath); statErr == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Steam artwork destination is not a regular file")
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect cached Steam artwork: %w", statErr)
	} else {
		temporary, createErr := os.CreateTemp(directory, ".steam-game-image-*.tmp")
		if createErr != nil {
			return fmt.Errorf("create temporary Steam artwork: %w", createErr)
		}
		temporaryPath := temporary.Name()
		keepTemporary := true
		defer func() {
			_ = temporary.Close()
			if keepTemporary {
				_ = os.Remove(temporaryPath)
			}
		}()
		if err := temporary.Chmod(0o600); err != nil {
			return fmt.Errorf("secure temporary Steam artwork: %w", err)
		}
		if _, err := temporary.Write(body); err != nil {
			return fmt.Errorf("write temporary Steam artwork: %w", err)
		}
		if err := temporary.Sync(); err != nil {
			return fmt.Errorf("flush temporary Steam artwork: %w", err)
		}
		if err := temporary.Close(); err != nil {
			return fmt.Errorf("close temporary Steam artwork: %w", err)
		}
		if renameErr := os.Rename(temporaryPath, destinationPath); renameErr != nil {
			if info, statErr := os.Stat(destinationPath); statErr != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("install Steam artwork: %w", renameErr)
			}
			_ = os.Remove(temporaryPath)
		} else {
			installedNewFile = true
		}
		keepTemporary = false
	}

	var previousFilename string
	previousErr := a.db.QueryRowContext(
		ctx,
		`SELECT filename FROM steam_game_images WHERE game_id = ?`,
		gameID,
	).Scan(&previousFilename)
	if previousErr != nil && !errors.Is(previousErr, sql.ErrNoRows) {
		if installedNewFile {
			_ = os.Remove(destinationPath)
		}
		return fmt.Errorf("load previous Steam artwork: %w", previousErr)
	}
	if _, err := a.db.ExecContext(ctx, `
		INSERT INTO steam_game_images (game_id, filename, mime_type, byte_size)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(game_id) DO UPDATE SET
			filename = excluded.filename,
			mime_type = excluded.mime_type,
			byte_size = excluded.byte_size,
			updated_at = CURRENT_TIMESTAMP
	`, gameID, filename, mimeType, len(body)); err != nil {
		if installedNewFile {
			_ = os.Remove(destinationPath)
		}
		return fmt.Errorf("save cached Steam artwork: %w", err)
	}
	if steamGameImageFilenamePattern.MatchString(previousFilename) && previousFilename != filename {
		_ = os.Remove(filepath.Join(directory, previousFilename))
	}
	return nil
}

func steamGameImageURL(filename string) string {
	if !steamGameImageFilenamePattern.MatchString(filename) {
		return ""
	}
	directory, err := storage.SteamGameImageDirectory()
	if err != nil {
		return ""
	}
	info, err := os.Stat(filepath.Join(directory, filename))
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return steamGameImageRoutePrefix + filename
}

type stagedSteamGameImage struct {
	originalPath string
	stagedPath   string
	moved        bool
}

func randomSteamGameImageID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate Steam artwork staging ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func stageSteamGameImageDeletion(filename string) (*stagedSteamGameImage, error) {
	if !steamGameImageFilenamePattern.MatchString(filename) {
		return nil, fmt.Errorf("stored Steam artwork filename is invalid")
	}
	directory, err := storage.SteamGameImageDirectory()
	if err != nil {
		return nil, err
	}
	stagedID, err := randomSteamGameImageID()
	if err != nil {
		return nil, err
	}
	staged := &stagedSteamGameImage{
		originalPath: filepath.Join(directory, filename),
		stagedPath:   filepath.Join(directory, "."+stagedID+".deleting"),
	}
	if err := os.Rename(staged.originalPath, staged.stagedPath); err == nil {
		staged.moved = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stage Steam artwork deletion: %w", err)
	}
	return staged, nil
}

func (s *stagedSteamGameImage) restore() {
	if s != nil && s.moved {
		_ = os.Rename(s.stagedPath, s.originalPath)
	}
}

func (s *stagedSteamGameImage) finish() {
	if s != nil && s.moved {
		_ = os.Remove(s.stagedPath)
	}
}

func NewSteamGameImageHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.URL.Path, steamGameImageRoutePrefix) {
			http.NotFound(w, r)
			return
		}
		filename := strings.TrimPrefix(r.URL.Path, steamGameImageRoutePrefix)
		if !steamGameImageFilenamePattern.MatchString(filename) {
			http.NotFound(w, r)
			return
		}
		directory, err := storage.SteamGameImageDirectory()
		if err != nil {
			http.Error(w, "Steam artwork storage unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, filepath.Join(directory, filename))
	})
}
