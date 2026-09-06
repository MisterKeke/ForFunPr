package backend

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ScreenshotCapabilities struct {
	CaptureSupported bool `json:"capture_supported"`
	OCRSupported     bool `json:"ocr_supported"`
}

func (a *App) GetScreenshotCapabilities() ScreenshotCapabilities {
	return ScreenshotCapabilities{
		CaptureSupported: a != nil && a.screenCapture != nil && a.screenCapture.Supported(),
		OCRSupported:     a != nil && a.ocr != nil && a.ocr.Supported(),
	}
}

func (a *App) CaptureScreenshot(mode string) (Screenshot, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Screenshot{}, err
	}
	defer done()
	if a.screenCapture == nil || !a.screenCapture.Supported() {
		return Screenshot{}, fmt.Errorf("screen capture is currently supported only on Windows")
	}
	runtime.WindowHide(ctx)
	defer runtime.WindowShow(ctx)
	time.Sleep(220 * time.Millisecond)
	image, err := a.screenCapture.Capture(mode)
	if err != nil {
		return Screenshot{}, err
	}
	runtime.WindowShow(ctx)
	return service.StoreScreenshotContext(ctx, image, mode)
}

func (a *App) ListScreenshots(filter ScreenshotListFilter) (ScreenshotListResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return ScreenshotListResult{}, err
	}
	defer done()
	return service.ListScreenshotsContext(ctx, filter)
}

func (a *App) RenameScreenshot(id string, title string) (Screenshot, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Screenshot{}, err
	}
	defer done()
	return service.RenameScreenshotContext(ctx, id, title)
}

func (a *App) SaveScreenshotEdit(request ScreenshotEditRequest) (Screenshot, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Screenshot{}, err
	}
	defer done()
	return service.SaveScreenshotEditContext(ctx, request)
}

func (a *App) RunScreenshotOCR(id string) (Screenshot, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Screenshot{}, err
	}
	defer done()
	if a.ocr == nil || !a.ocr.Supported() {
		service.FailScreenshotOCRContext(ctx, id, true)
		return Screenshot{}, fmt.Errorf("local OCR is currently supported only on Windows")
	}
	path, err := service.ScreenshotFilePathContext(ctx, id)
	if err != nil {
		return Screenshot{}, err
	}
	if err := service.SetScreenshotOCRProcessingContext(ctx, id); err != nil {
		return Screenshot{}, err
	}
	text, language, err := a.ocr.Recognize(ctx, path)
	if err != nil {
		service.FailScreenshotOCRContext(ctx, id, false)
		return Screenshot{}, err
	}
	return service.CompleteScreenshotOCRContext(ctx, id, text, language)
}

func (a *App) ExportScreenshot(id string) (string, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return "", err
	}
	defer done()
	source, err := service.ScreenshotFilePathContext(ctx, id)
	if err != nil {
		return "", err
	}
	destination, err := runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{
		Title: "Export screenshot", DefaultFilename: "screenshot.png",
		Filters: []runtime.FileFilter{{DisplayName: "PNG image (*.png)", Pattern: "*.png"}},
	})
	if err != nil || destination == "" {
		return "", err
	}
	if filepath.Ext(destination) == "" {
		destination += ".png"
	}
	if filepath.Clean(source) == filepath.Clean(destination) {
		return destination, nil
	}
	input, err := os.Open(source)
	if err != nil {
		return "", fmt.Errorf("open screenshot: %w", err)
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("create exported screenshot: %w", err)
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return "", fmt.Errorf("export screenshot: %w", copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("finish screenshot export: %w", closeErr)
	}
	return destination, nil
}

func (a *App) DeleteScreenshot(id string) error {
	service, ctx, done, err := a.begin()
	if err != nil {
		return err
	}
	defer done()
	return service.DeleteScreenshotContext(ctx, id)
}
