package backend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"something/backend/screencapture"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ScreenshotCapabilities struct {
	CaptureSupported bool `json:"capture_supported"`
	OCRSupported     bool `json:"ocr_supported"`
}

type ScreenshotCaptureResult struct {
	Screenshot Screenshot `json:"screenshot"`
	Cancelled  bool       `json:"cancelled"`
}

func (a *App) GetScreenshotCapabilities() ScreenshotCapabilities {
	return ScreenshotCapabilities{
		CaptureSupported: a != nil && a.service != nil && a.service.CapabilityAvailable("screenshots") && a.screenCapture != nil && a.screenCapture.Supported(),
		OCRSupported:     a != nil && a.service != nil && a.service.CapabilityAvailable("ocr") && a.ocr != nil && a.ocr.Supported(),
	}
}

func (a *App) CaptureScreenshot(request ScreenshotCaptureRequest) (ScreenshotCaptureResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return ScreenshotCaptureResult{}, err
	}
	defer done()
	if !service.CapabilityAvailable("screenshots") || a.screenCapture == nil || !a.screenCapture.Supported() {
		return ScreenshotCaptureResult{}, fmt.Errorf("screen capture is currently supported only on Windows")
	}
	runtime.WindowHide(ctx)
	restored := false
	restore := func() {
		if !restored {
			runtime.WindowShow(ctx)
			restored = true
		}
	}
	defer restore()
	if err := a.screenCapture.WaitUntilReady(ctx); err != nil {
		return ScreenshotCaptureResult{}, err
	}
	captured, err := a.screenCapture.Capture(ctx, request)
	if errors.Is(err, screencapture.ErrCancelled) {
		return ScreenshotCaptureResult{Cancelled: true}, nil
	}
	if err != nil {
		return ScreenshotCaptureResult{}, err
	}
	if captured.Kind == screencapture.ModeRegion && !captured.RegionSelected {
		return ScreenshotCaptureResult{}, fmt.Errorf("region capture did not include a selected rectangle")
	}
	restore()
	item, err := service.StoreScreenshotContext(ctx, captured.Image, string(captured.Kind))
	if err != nil {
		return ScreenshotCaptureResult{}, err
	}
	return ScreenshotCaptureResult{Screenshot: item}, nil
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

func (a *App) RevertScreenshotEdit(id string) (Screenshot, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Screenshot{}, err
	}
	defer done()
	return service.RevertScreenshotEditContext(ctx, id)
}

func (a *App) RunScreenshotOCR(id string) (ScreenshotOCRQueueResult, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return ScreenshotOCRQueueResult{}, err
	}
	defer done()
	if !service.CapabilityAvailable("ocr") && a.ocr != nil && a.ocr.Supported() {
		return ScreenshotOCRQueueResult{}, fmt.Errorf("screenshot text recognition is temporarily unavailable")
	}
	a.ocrMu.Lock()
	alreadyRunning := a.ocrCancels[id] != nil
	a.ocrMu.Unlock()
	if alreadyRunning {
		item, err := service.GetScreenshotContext(ctx, id)
		return ScreenshotOCRQueueResult{Screenshot: item, Changed: false}, err
	}
	queued, err := service.QueueScreenshotOCRContext(ctx, id)
	if err != nil {
		return ScreenshotOCRQueueResult{}, err
	}
	if !queued.Changed {
		return queued, nil
	}
	if a.ocr == nil || !a.ocr.Supported() {
		if err := service.FailScreenshotOCRContext(ctx, id, true, "unsupported", ""); err != nil {
			return ScreenshotOCRQueueResult{}, err
		}
		item, err := service.GetScreenshotContext(ctx, id)
		return ScreenshotOCRQueueResult{Screenshot: item, Changed: true}, err
	}
	jobContext, cancel := context.WithCancel(service.OperationContext())
	a.ocrMu.Lock()
	if existing := a.ocrCancels[id]; existing != nil {
		a.ocrMu.Unlock()
		cancel()
		return queued, nil
	}
	a.ocrCancels[id] = cancel
	a.ocrMu.Unlock()
	service.SetCapability("ocr", true, true, false, "")
	go a.runScreenshotOCR(jobContext, id, cancel)
	return queued, nil
}

func (a *App) runScreenshotOCR(jobContext context.Context, id string, cancel context.CancelFunc) {
	defer func() {
		cancel()
		a.ocrMu.Lock()
		delete(a.ocrCancels, id)
		running := len(a.ocrCancels) > 0
		a.ocrMu.Unlock()
		a.service.SetCapability("ocr", true, running, false, "")
	}()
	ctx, done, err := a.service.BeginOperation(jobContext)
	if err != nil {
		return
	}
	defer done()
	started, err := a.service.SetScreenshotOCRProcessingContext(ctx, id)
	if err != nil || !started {
		if err != nil {
			slog.Error("Screenshot OCR could not start", "error", err)
		}
		return
	}
	path, err := a.service.ScreenshotFilePathContext(ctx, id)
	if err != nil {
		if failureErr := a.service.FailScreenshotOCRContext(ctx, id, false, "image_unavailable", "The screenshot image is unavailable."); failureErr != nil {
			slog.Error("Screenshot OCR failure could not be recorded", "error", failureErr)
		}
		return
	}
	text, language, err := a.ocr.Recognize(ctx, path)
	if err != nil {
		if ctx.Err() == nil {
			if failureErr := a.service.FailScreenshotOCRContext(ctx, id, false, "recognition_failed", "Text extraction failed. You can try again."); failureErr != nil {
				slog.Error("Screenshot OCR failure could not be recorded", "error", failureErr)
			}
		}
		return
	}
	if _, err := a.service.CompleteScreenshotOCRContext(ctx, id, text, language); err != nil {
		slog.Error("Screenshot OCR completion could not be recorded", "error", err)
	}
}

func (a *App) CancelScreenshotOCR(id string) (Screenshot, error) {
	service, ctx, done, err := a.begin()
	if err != nil {
		return Screenshot{}, err
	}
	defer done()
	a.ocrMu.Lock()
	cancel := a.ocrCancels[id]
	a.ocrMu.Unlock()
	if cancel != nil {
		cancel()
	}
	item, _, err := service.CancelScreenshotOCRContext(ctx, id)
	return item, err
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
