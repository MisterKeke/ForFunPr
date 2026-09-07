package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"something/backend/storage"
)

func useTemporaryApplicationData(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	return root
}

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	value := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			value.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 120, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, value); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func pngDataURL(data []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

func TestWallpaperImportSelectionServingDeletionAndRollback(t *testing.T) {
	useTemporaryApplicationData(t)
	service := newFeatureTestService(t)
	ctx := context.Background()
	sourcePath := filepath.Join(t.TempDir(), "  My wallpaper.png")
	if err := os.WriteFile(sourcePath, testPNG(t, 4, 3), 0o600); err != nil {
		t.Fatal(err)
	}
	wallpaper, err := service.ImportWallpaperFromPathContext(ctx, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if wallpaper.MIMEType != "image/png" || wallpaper.ByteSize <= 0 || wallpaper.DisplayName != "My wallpaper" {
		t.Fatalf("imported wallpaper = %#v", wallpaper)
	}
	settings, err := service.GetWallpaperSettingsContext(ctx)
	if err != nil || settings.Selected != "custom:"+wallpaper.ID || !settings.SelectionSaved {
		t.Fatalf("wallpaper settings = %#v, %v", settings, err)
	}

	handler := NewUserWallpaperHandler()
	request := httptest.NewRequest(http.MethodGet, wallpaper.URL, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("wallpaper response = %d, %q", response.Code, response.Header().Get("Content-Type"))
	}
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodPost, wallpaper.URL, nil),
		httptest.NewRequest(http.MethodGet, "/user-wallpapers/../database.db", nil),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code == http.StatusOK {
			t.Fatalf("unsafe wallpaper request succeeded: %s %s", request.Method, request.URL.Path)
		}
	}

	badPath := filepath.Join(t.TempDir(), "forged.png")
	if err := os.WriteFile(badPath, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ImportWallpaperFromPathContext(ctx, badPath); err == nil {
		t.Fatal("forged wallpaper extension unexpectedly accepted")
	}

	directory, err := storage.WallpaperDirectory()
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.db.Exec(`CREATE TRIGGER reject_wallpaper BEFORE INSERT ON user_wallpapers
		BEGIN SELECT RAISE(ABORT, 'reject wallpaper'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ImportWallpaperFromPathContext(ctx, sourcePath); err == nil {
		t.Fatal("wallpaper metadata failure unexpectedly succeeded")
	}
	after, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("failed import leaked a file: before=%d after=%d", len(before), len(after))
	}

	wallpaperPath := filepath.Join(directory, wallpaper.Filename)
	if _, err := service.db.Exec(`CREATE TRIGGER reject_wallpaper_delete BEFORE DELETE ON user_wallpapers
		BEGIN SELECT RAISE(ABORT, 'reject wallpaper delete'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DeleteUserWallpaperContext(ctx, wallpaper.ID); err == nil {
		t.Fatal("triggered wallpaper delete unexpectedly succeeded")
	}
	if _, err := os.Stat(wallpaperPath); err != nil {
		t.Fatalf("failed delete did not restore wallpaper: %v", err)
	}
	if _, err := service.db.Exec(`DROP TRIGGER reject_wallpaper_delete`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DeleteUserWallpaperContext(ctx, wallpaper.ID); err != nil {
		t.Fatal(err)
	}
	settings, err = service.GetWallpaperSettingsContext(ctx)
	if err != nil || settings.Selected != defaultWallpaperSelection || len(settings.UserWallpapers) != 0 {
		t.Fatalf("settings after delete = %#v, %v", settings, err)
	}
}

type oversizedTestImage struct{}

func (oversizedTestImage) ColorModel() color.Model { return color.RGBAModel }
func (oversizedTestImage) Bounds() image.Rectangle { return image.Rect(0, 0, 32769, 1) }
func (oversizedTestImage) At(int, int) color.Color { return color.Black }

func TestScreenshotLifecycleResizeServingAndDeleteRecovery(t *testing.T) {
	useTemporaryApplicationData(t)
	service := newFeatureTestService(t)
	ctx := context.Background()
	if _, err := service.StoreScreenshotContext(ctx, nil, "screen"); err == nil {
		t.Fatal("nil screenshot accepted")
	}
	if _, err := service.StoreScreenshotContext(ctx, oversizedTestImage{}, "screen"); err == nil {
		t.Fatal("oversized screenshot dimensions accepted")
	}
	if _, err := service.StoreScreenshotContext(ctx, image.NewRGBA(image.Rect(0, 0, 1, 1)), "invalid"); err == nil {
		t.Fatal("invalid capture kind accepted")
	}

	originalImage := image.NewRGBA(image.Rect(10, 20, 1010, 520))
	stored, err := service.StoreScreenshotContext(ctx, originalImage, "region")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Width != 1000 || stored.Height != 500 {
		t.Fatalf("stored screenshot dimensions = %dx%d", stored.Width, stored.Height)
	}
	directory, err := storage.ScreenshotDirectory()
	if err != nil {
		t.Fatal(err)
	}
	oldThumbnail := filepath.Join(directory, stored.ThumbnailFilename)
	thumbnailFile, err := os.Open(oldThumbnail)
	if err != nil {
		t.Fatal(err)
	}
	thumbnail, err := png.Decode(thumbnailFile)
	_ = thumbnailFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if thumbnail.Bounds().Dx() != 420 || thumbnail.Bounds().Dy() != 210 {
		t.Fatalf("thumbnail dimensions = %v", thumbnail.Bounds())
	}

	edited, err := service.SaveScreenshotEditContext(ctx, ScreenshotEditRequest{
		ID: stored.ID, DataURL: pngDataURL(testPNG(t, 20, 10)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if edited.EditedFilename == "" || edited.Width != 20 || edited.Height != 10 {
		t.Fatalf("edited screenshot = %#v", edited)
	}
	if _, err := os.Stat(oldThumbnail); !os.IsNotExist(err) {
		t.Fatalf("old thumbnail was not removed: %v", err)
	}
	if _, err := service.SaveScreenshotEditContext(ctx, ScreenshotEditRequest{
		ID: stored.ID, DataURL: "data:image/png;base64,not-base64",
	}); err == nil {
		t.Fatal("invalid screenshot edit accepted")
	}

	handler := NewScreenshotImageHandler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodHead, edited.ImageURL, nil))
	if response.Code != http.StatusOK || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("screenshot response = %d, %#v", response.Code, response.Header())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/screenshots/../database.db", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("traversal response = %d", response.Code)
	}

	if _, err := service.db.Exec(`CREATE TRIGGER reject_screenshot_delete BEFORE DELETE ON screenshots
		BEGIN SELECT RAISE(ABORT, 'reject delete'); END`); err != nil {
		t.Fatal(err)
	}
	activePath, err := service.ScreenshotFilePathContext(ctx, stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteScreenshotContext(ctx, stored.ID); err == nil {
		t.Fatal("triggered screenshot delete unexpectedly succeeded")
	}
	if _, err := os.Stat(activePath); err != nil {
		t.Fatalf("failed delete did not restore screenshot: %v", err)
	}
	if _, err := service.db.Exec(`DROP TRIGGER reject_screenshot_delete`); err != nil {
		t.Fatal(err)
	}
	filenames := []string{edited.OriginalFilename, edited.EditedFilename, edited.ThumbnailFilename}
	if err := service.DeleteScreenshotContext(ctx, stored.ID); err != nil {
		t.Fatal(err)
	}
	for _, filename := range filenames {
		if _, err := os.Stat(filepath.Join(directory, filename)); !os.IsNotExist(err) {
			t.Fatalf("deleted screenshot file %q remains: %v", filename, err)
		}
	}
}

func TestScreenshotDatabaseFailureDoesNotLeakFiles(t *testing.T) {
	useTemporaryApplicationData(t)
	service := newFeatureTestService(t)
	directory, err := storage.ScreenshotDirectory()
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.db.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = service.StoreScreenshotContext(context.Background(), image.NewRGBA(image.Rect(0, 0, 4, 4)), "screen")
	if err == nil {
		t.Fatal("screenshot storage unexpectedly succeeded with a closed database")
	}
	after, readErr := os.ReadDir(directory)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(after) != len(before) {
		t.Fatalf("failed screenshot store leaked files: before=%d after=%d", len(before), len(after))
	}
}

func TestSetupIconValidationInstallCleanupAndServing(t *testing.T) {
	useTemporaryApplicationData(t)
	dataURL := pngDataURL(testPNG(t, 2, 2))
	icon, err := prepareSetupIcon(dataURL)
	if err != nil {
		t.Fatal(err)
	}
	if icon.mimeType != "image/png" || !strings.HasSuffix(icon.filename, ".png") {
		t.Fatalf("prepared setup icon = %#v", icon)
	}
	if err := icon.install(); err != nil {
		t.Fatal(err)
	}
	finalPath := icon.final
	icon.cleanup()
	if _, err := os.Stat(finalPath); !os.IsNotExist(err) {
		t.Fatalf("unkept installed icon remains: %v", err)
	}

	icon, err = prepareSetupIcon(dataURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := icon.install(); err != nil {
		t.Fatal(err)
	}
	icon.keep()
	icon.cleanup()
	if _, err := os.Stat(icon.final); err != nil {
		t.Fatalf("kept icon missing: %v", err)
	}
	defer os.Remove(icon.final)

	handler := NewSetupIconHandler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, setupIconRoutePrefix+icon.filename, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("setup icon response = %d", response.Code)
	}
	for _, invalid := range []string{
		"data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte("text")),
		"data:image/png;base64,invalid",
		"data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("not an image")),
	} {
		if _, err := prepareSetupIcon(invalid); err == nil {
			t.Fatalf("invalid setup icon accepted: %q", invalid)
		}
	}

	staged, err := stageSetupIconDeletion(icon.filename)
	if err != nil {
		t.Fatal(err)
	}
	if !staged.moved {
		t.Fatal("existing setup icon was not staged")
	}
	if _, err := os.Stat(icon.final); !os.IsNotExist(err) {
		t.Fatalf("staged icon remained at its public path: %v", err)
	}
	staged.restore()
	if _, err := os.Stat(icon.final); err != nil {
		t.Fatalf("restored setup icon missing: %v", err)
	}
	staged, err = stageSetupIconDeletion(icon.filename)
	if err != nil {
		t.Fatal(err)
	}
	staged.finish()
	if _, err := os.Stat(icon.final); !os.IsNotExist(err) {
		t.Fatalf("finished setup icon deletion left the file: %v", err)
	}
}

func TestAssetFormatDetectionAndAggregateRouteConfinement(t *testing.T) {
	cases := []struct {
		data      []byte
		mimeType  string
		extension string
	}{
		{[]byte{0, 0, 1, 0, 1}, "image/x-icon", ".ico"},
		{[]byte("\x89PNG\r\n\x1a\nrest"), "image/png", ".png"},
	}
	for _, testCase := range cases {
		mimeType, extension, err := detectDesktopAppIconFormat(testCase.data)
		if err != nil || mimeType != testCase.mimeType || extension != testCase.extension {
			t.Fatalf("format detection = %q, %q, %v", mimeType, extension, err)
		}
	}
	if _, _, err := detectDesktopAppIconFormat([]byte("plain text")); err == nil {
		t.Fatal("plain text icon accepted")
	}

	useTemporaryApplicationData(t)
	handler := NewUserAssetHandler()
	for _, path := range []string{"/icons/../database.db", "/setup-icons/invalid.png", "/steam-game-images/1.png", "/screenshots/not-a-uuid.png"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("asset path %q returned %d", path, response.Code)
		}
	}
}
