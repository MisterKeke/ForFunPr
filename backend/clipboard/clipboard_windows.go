//go:build windows

package clipboard

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	clipboardUnicodeText = 13
	globalMemoryMoveable = 0x0002
)

var (
	user32DLL                      = windows.NewLazySystemDLL("user32.dll")
	kernel32DLL                    = windows.NewLazySystemDLL("kernel32.dll")
	getClipboardSequenceNumberProc = user32DLL.NewProc("GetClipboardSequenceNumber")
	openClipboardProc              = user32DLL.NewProc("OpenClipboard")
	closeClipboardProc             = user32DLL.NewProc("CloseClipboard")
	isClipboardFormatAvailableProc = user32DLL.NewProc("IsClipboardFormatAvailable")
	getClipboardDataProc           = user32DLL.NewProc("GetClipboardData")
	emptyClipboardProc             = user32DLL.NewProc("EmptyClipboard")
	setClipboardDataProc           = user32DLL.NewProc("SetClipboardData")
	globalAllocProc                = kernel32DLL.NewProc("GlobalAlloc")
	globalFreeProc                 = kernel32DLL.NewProc("GlobalFree")
	globalLockProc                 = kernel32DLL.NewProc("GlobalLock")
	globalUnlockProc               = kernel32DLL.NewProc("GlobalUnlock")
	globalSizeProc                 = kernel32DLL.NewProc("GlobalSize")
)

type windowsController struct {
	mu           sync.Mutex
	running      bool
	cancel       context.CancelFunc
	done         chan struct{}
	suppressText string
}

func New() Controller { return &windowsController{} }

func (*windowsController) Supported() bool { return true }

func (c *windowsController) Running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

func (c *windowsController) Start(parent context.Context, onText func(string)) error {
	if onText == nil {
		return errors.New("clipboard listener requires a callback")
	}
	if parent == nil {
		parent = context.Background()
	}
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	c.running = true
	c.cancel = cancel
	c.done = done
	c.suppressText = ""
	c.mu.Unlock()

	go c.listen(ctx, done, onText)
	return nil
}

func (c *windowsController) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	cancel := c.cancel
	done := c.done
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (c *windowsController) listen(ctx context.Context, done chan struct{}, onText func(string)) {
	defer func() {
		c.mu.Lock()
		c.running = false
		c.cancel = nil
		c.done = nil
		c.mu.Unlock()
		close(done)
	}()
	lastSequence, _, _ := getClipboardSequenceNumberProc.Call()
	ticker := time.NewTicker(350 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sequence, _, _ := getClipboardSequenceNumberProc.Call()
			if sequence == 0 || sequence == lastSequence {
				continue
			}
			lastSequence = sequence
			value, err := readText()
			if err != nil || value == "" {
				continue
			}
			c.mu.Lock()
			suppressed := c.suppressText != "" && c.suppressText == value
			if suppressed {
				c.suppressText = ""
			}
			c.mu.Unlock()
			if !suppressed {
				onText(value)
			}
		}
	}
}

func (c *windowsController) SetText(value string) error {
	encoded, err := syscall.UTF16FromString(value)
	if err != nil {
		return fmt.Errorf("encode clipboard text: %w", err)
	}
	byteCount := uintptr(len(encoded) * 2)
	handle, _, callErr := globalAllocProc.Call(globalMemoryMoveable, byteCount)
	if handle == 0 {
		return fmt.Errorf("allocate clipboard memory: %w", callErr)
	}
	owned := true
	defer func() {
		if owned {
			globalFreeProc.Call(handle)
		}
	}()

	pointer, _, callErr := globalLockProc.Call(handle)
	if pointer == 0 {
		return fmt.Errorf("lock clipboard memory: %w", callErr)
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(pointer)), len(encoded)), encoded)
	globalUnlockProc.Call(handle)
	runtime.KeepAlive(encoded)

	if err := openClipboardWithRetry(); err != nil {
		return err
	}
	defer closeClipboardProc.Call()
	if result, _, callErr := emptyClipboardProc.Call(); result == 0 {
		return fmt.Errorf("empty clipboard: %w", callErr)
	}
	if result, _, callErr := setClipboardDataProc.Call(clipboardUnicodeText, handle); result == 0 {
		return fmt.Errorf("set clipboard text: %w", callErr)
	}
	owned = false
	c.mu.Lock()
	c.suppressText = value
	c.mu.Unlock()
	return nil
}

func readText() (string, error) {
	available, _, _ := isClipboardFormatAvailableProc.Call(clipboardUnicodeText)
	if available == 0 {
		return "", nil
	}
	if err := openClipboardWithRetry(); err != nil {
		return "", err
	}
	defer closeClipboardProc.Call()
	handle, _, callErr := getClipboardDataProc.Call(clipboardUnicodeText)
	if handle == 0 {
		return "", fmt.Errorf("read clipboard data: %w", callErr)
	}
	pointer, _, callErr := globalLockProc.Call(handle)
	if pointer == 0 {
		return "", fmt.Errorf("lock clipboard data: %w", callErr)
	}
	defer globalUnlockProc.Call(handle)
	size, _, _ := globalSizeProc.Call(handle)
	if size < 2 {
		return "", nil
	}
	if size > 2*1024*1024 {
		return "", fmt.Errorf("clipboard text exceeds the supported size")
	}
	units := unsafe.Slice((*uint16)(unsafe.Pointer(pointer)), int(size/2))
	length := 0
	for length < len(units) && units[length] != 0 {
		length++
	}
	return string(utf16.Decode(units[:length])), nil
}

func openClipboardWithRetry() error {
	var lastErr error
	for attempt := 0; attempt < 8; attempt++ {
		result, _, callErr := openClipboardProc.Call(0)
		if result != 0 {
			return nil
		}
		lastErr = callErr
		time.Sleep(12 * time.Millisecond)
	}
	return fmt.Errorf("open clipboard: %w", lastErr)
}
