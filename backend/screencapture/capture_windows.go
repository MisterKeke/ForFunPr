//go:build windows

package screencapture

import (
	"context"
	"fmt"
	"image"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	systemMetricXVirtualScreen  = 76
	systemMetricYVirtualScreen  = 77
	systemMetricCXVirtualScreen = 78
	systemMetricCYVirtualScreen = 79
	sourceCopy                  = 0x00CC0020
	captureLayeredWindows       = 0x40000000
	dibRGBColors                = 0
	maximumCaptureDimension     = 32768
	maximumCapturePixels        = 100000000
	wmDestroy                   = 0x0002
	wmPaint                     = 0x000F
	wmClose                     = 0x0010
	wmKeyDown                   = 0x0100
	wmMouseMove                 = 0x0200
	wmLButtonDown               = 0x0201
	wmLButtonUp                 = 0x0202
	wmRButtonDown               = 0x0204
	vkEscape                    = 0x1B
	wsPopup                     = 0x80000000
	wsExTopmost                 = 0x00000008
	wsExToolWindow              = 0x00000080
	wsExLayered                 = 0x00080000
	showWindow                  = 5
	layeredAlpha                = 0x00000002
	stockBlackBrush             = 4
	stockHollowBrush            = 5
	penSolid                    = 0
	crossCursorID               = 32515
)

var (
	captureUser32DLL           = windows.NewLazySystemDLL("user32.dll")
	captureGDI32DLL            = windows.NewLazySystemDLL("gdi32.dll")
	captureKernel32DLL         = windows.NewLazySystemDLL("kernel32.dll")
	captureDWMAPIDLL           = windows.NewLazySystemDLL("dwmapi.dll")
	getSystemMetricsProc       = captureUser32DLL.NewProc("GetSystemMetrics")
	getForegroundWindowProc    = captureUser32DLL.NewProc("GetForegroundWindow")
	getWindowRectProc          = captureUser32DLL.NewProc("GetWindowRect")
	getDCProc                  = captureUser32DLL.NewProc("GetDC")
	releaseDCProc              = captureUser32DLL.NewProc("ReleaseDC")
	createCompatibleDCProc     = captureGDI32DLL.NewProc("CreateCompatibleDC")
	deleteDCProc               = captureGDI32DLL.NewProc("DeleteDC")
	createCompatibleBitmapProc = captureGDI32DLL.NewProc("CreateCompatibleBitmap")
	selectObjectProc           = captureGDI32DLL.NewProc("SelectObject")
	deleteObjectProc           = captureGDI32DLL.NewProc("DeleteObject")
	bitBltProc                 = captureGDI32DLL.NewProc("BitBlt")
	getDIBitsProc              = captureGDI32DLL.NewProc("GetDIBits")
	dwmFlushProc               = captureDWMAPIDLL.NewProc("DwmFlush")
	setThreadDPIAwarenessProc  = captureUser32DLL.NewProc("SetThreadDpiAwarenessContext")
	registerClassExProc        = captureUser32DLL.NewProc("RegisterClassExW")
	createWindowExProc         = captureUser32DLL.NewProc("CreateWindowExW")
	defWindowProc              = captureUser32DLL.NewProc("DefWindowProcW")
	destroyWindowProc          = captureUser32DLL.NewProc("DestroyWindow")
	showWindowProc             = captureUser32DLL.NewProc("ShowWindow")
	updateWindowProc           = captureUser32DLL.NewProc("UpdateWindow")
	setForegroundWindowProc    = captureUser32DLL.NewProc("SetForegroundWindow")
	setFocusProc               = captureUser32DLL.NewProc("SetFocus")
	setCaptureProc             = captureUser32DLL.NewProc("SetCapture")
	releaseCaptureProc         = captureUser32DLL.NewProc("ReleaseCapture")
	getCursorPosProc           = captureUser32DLL.NewProc("GetCursorPos")
	screenToClientProc         = captureUser32DLL.NewProc("ScreenToClient")
	invalidateRectProc         = captureUser32DLL.NewProc("InvalidateRect")
	beginPaintProc             = captureUser32DLL.NewProc("BeginPaint")
	endPaintProc               = captureUser32DLL.NewProc("EndPaint")
	getMessageProc             = captureUser32DLL.NewProc("GetMessageW")
	translateMessageProc       = captureUser32DLL.NewProc("TranslateMessage")
	dispatchMessageProc        = captureUser32DLL.NewProc("DispatchMessageW")
	postQuitMessageProc        = captureUser32DLL.NewProc("PostQuitMessage")
	postMessageProc            = captureUser32DLL.NewProc("PostMessageW")
	setLayeredAttributesProc   = captureUser32DLL.NewProc("SetLayeredWindowAttributes")
	loadCursorProc             = captureUser32DLL.NewProc("LoadCursorW")
	getStockObjectProc         = captureGDI32DLL.NewProc("GetStockObject")
	createPenProc              = captureGDI32DLL.NewProc("CreatePen")
	rectangleProc              = captureGDI32DLL.NewProc("Rectangle")
	getModuleHandleProc        = captureKernel32DLL.NewProc("GetModuleHandleW")
)

type windowsCapturer struct{}

var regionSelectionMu sync.Mutex

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

func New() Capturer                     { return windowsCapturer{} }
func (windowsCapturer) Supported() bool { return true }

func (windowsCapturer) WaitUntilReady(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// DwmFlush waits until the compositor has processed pending window state
	// changes, which is a stronger readiness boundary than a fixed delay.
	result, _, _ := dwmFlushProc.Call()
	if int32(result) < 0 {
		return fmt.Errorf("wait for desktop compositor: HRESULT 0x%x", uint32(result))
	}
	return ctx.Err()
}

func (windowsCapturer) Capture(ctx context.Context, request Request) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	previousDPIContext, _, _ := setThreadDPIAwarenessProc.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
	if previousDPIContext != 0 {
		defer setThreadDPIAwarenessProc.Call(previousDPIContext)
	}

	var bounds winRect
	virtualBounds := getVirtualScreenBounds()
	kind := request.Mode
	switch request.Mode {
	case ModeScreen:
		bounds = virtualBounds
	case ModeRegion:
		var selected Rectangle
		var err error
		if request.Region != nil {
			selected = *request.Region
		} else if request.Interactive {
			selected, err = selectRegion(ctx, virtualBounds)
			if err != nil {
				return Result{}, err
			}
		} else {
			return Result{}, &captureValidationError{message: "region capture requires a selected rectangle"}
		}
		if err := validateCaptureRectangle(selected, virtualBounds); err != nil {
			return Result{}, err
		}
		// The selection overlay has just been destroyed. Wait for DWM to
		// present that state before reading desktop pixels so the overlay is
		// never included in the saved region.
		if result, _, _ := dwmFlushProc.Call(); int32(result) < 0 {
			return Result{}, fmt.Errorf("hide region selector: HRESULT 0x%x", uint32(result))
		}
		bounds = winRect{Left: int32(selected.X), Top: int32(selected.Y), Right: int32(selected.X + selected.Width), Bottom: int32(selected.Y + selected.Height)}
	case ModeWindow:
		window, _, _ := getForegroundWindowProc.Call()
		if window == 0 {
			return Result{}, fmt.Errorf("no foreground window is available")
		}
		result, _, callErr := getWindowRectProc.Call(window, uintptr(unsafe.Pointer(&bounds)))
		if result == 0 {
			return Result{}, fmt.Errorf("read foreground window bounds: %w", callErr)
		}
		bounds.Left = maximumInt32(bounds.Left, virtualBounds.Left)
		bounds.Top = maximumInt32(bounds.Top, virtualBounds.Top)
		bounds.Right = minimumInt32(bounds.Right, virtualBounds.Right)
		bounds.Bottom = minimumInt32(bounds.Bottom, virtualBounds.Bottom)
	default:
		return Result{}, fmt.Errorf("unsupported capture mode %q", request.Mode)
	}
	width := int(bounds.Right - bounds.Left)
	height := int(bounds.Bottom - bounds.Top)
	rectangle := Rectangle{X: int(bounds.Left), Y: int(bounds.Top), Width: width, Height: height}
	if err := validateCaptureRectangle(rectangle, virtualBounds); err != nil {
		return Result{}, err
	}
	captured, err := captureRectangle(rectangle.X, rectangle.Y, rectangle.Width, rectangle.Height)
	if err != nil {
		return Result{}, err
	}
	return Result{Image: captured, Kind: kind, Bounds: rectangle, RegionSelected: kind == ModeRegion}, nil
}

type captureValidationError struct{ message string }

func (e *captureValidationError) Error() string { return e.message }

func getVirtualScreenBounds() winRect {
	x, _, _ := getSystemMetricsProc.Call(systemMetricXVirtualScreen)
	y, _, _ := getSystemMetricsProc.Call(systemMetricYVirtualScreen)
	width, _, _ := getSystemMetricsProc.Call(systemMetricCXVirtualScreen)
	height, _, _ := getSystemMetricsProc.Call(systemMetricCYVirtualScreen)
	left, top := int32(x), int32(y)
	return winRect{Left: left, Top: top, Right: left + int32(width), Bottom: top + int32(height)}
}

func validateCaptureRectangle(rectangle Rectangle, virtual winRect) error {
	if rectangle.Width <= 0 || rectangle.Height <= 0 {
		return &captureValidationError{message: "capture width and height must be positive"}
	}
	if rectangle.Width > maximumCaptureDimension || rectangle.Height > maximumCaptureDimension ||
		rectangle.Width > maximumCapturePixels/rectangle.Height {
		return &captureValidationError{message: "capture rectangle exceeds the supported bounds"}
	}
	right := int64(rectangle.X) + int64(rectangle.Width)
	bottom := int64(rectangle.Y) + int64(rectangle.Height)
	if int64(rectangle.X) < int64(virtual.Left) || int64(rectangle.Y) < int64(virtual.Top) ||
		right > int64(virtual.Right) || bottom > int64(virtual.Bottom) {
		return &captureValidationError{message: "capture rectangle is outside the virtual desktop"}
	}
	return nil
}

type regionPoint struct{ X, Y int }

type regionSelectorState struct {
	hwnd      uintptr
	virtual   winRect
	anchor    regionPoint
	current   regionPoint
	dragging  bool
	selected  bool
	cancelled bool
}

type windowClassEx struct {
	Size, Style                        uint32
	WindowProc                         uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	SmallIcon                          uintptr
}

type windowMessage struct {
	Window, Message, WParam, LParam uintptr
	Time                            uint32
	Point                           struct{ X, Y int32 }
	Private                         uint32
}

type paintStruct struct {
	DeviceContext        uintptr
	Erase                int32
	Paint                winRect
	Restore, Incremental int32
	Reserved             [32]byte
}

var activeRegionSelector *regionSelectorState
var regionSelectorWindowProc = syscall.NewCallback(regionSelectorProc)

func selectRegion(ctx context.Context, virtual winRect) (Rectangle, error) {
	regionSelectionMu.Lock()
	defer regionSelectionMu.Unlock()
	if err := ctx.Err(); err != nil {
		return Rectangle{}, err
	}
	className, err := windows.UTF16PtrFromString("SomethingRegionSelector")
	if err != nil {
		return Rectangle{}, fmt.Errorf("prepare region selector: %w", err)
	}
	instance, _, callErr := getModuleHandleProc.Call(0)
	if instance == 0 {
		return Rectangle{}, fmt.Errorf("load process module for region selector: %w", callErr)
	}
	cursor, _, _ := loadCursorProc.Call(0, crossCursorID)
	background, _, _ := getStockObjectProc.Call(stockBlackBrush)
	class := windowClassEx{
		Size: uint32(unsafe.Sizeof(windowClassEx{})), WindowProc: regionSelectorWindowProc,
		Instance: instance, Cursor: cursor, Background: background, ClassName: className,
	}
	if result, _, registerErr := registerClassExProc.Call(uintptr(unsafe.Pointer(&class))); result == 0 && registerErr != syscall.Errno(1410) {
		return Rectangle{}, fmt.Errorf("register region selector: %w", registerErr)
	}
	state := &regionSelectorState{virtual: virtual, cancelled: true}
	activeRegionSelector = state
	defer func() { activeRegionSelector = nil }()
	width, height := int(virtual.Right-virtual.Left), int(virtual.Bottom-virtual.Top)
	hwnd, _, callErr := createWindowExProc.Call(
		wsExTopmost|wsExToolWindow|wsExLayered,
		uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(className)), wsPopup,
		uintptr(int64(virtual.Left)), uintptr(int64(virtual.Top)), uintptr(width), uintptr(height),
		0, 0, instance, 0,
	)
	if hwnd == 0 {
		return Rectangle{}, fmt.Errorf("open region selector: %w", callErr)
	}
	state.hwnd = hwnd
	setLayeredAttributesProc.Call(hwnd, 0, 150, layeredAlpha)
	showWindowProc.Call(hwnd, showWindow)
	updateWindowProc.Call(hwnd)
	setForegroundWindowProc.Call(hwnd)
	setFocusProc.Call(hwnd)

	selectionDone := make(chan struct{})
	defer close(selectionDone)
	go func() {
		select {
		case <-ctx.Done():
			postMessageProc.Call(hwnd, wmClose, 0, 0)
		case <-selectionDone:
		}
	}()
	var message windowMessage
	for {
		result, _, messageErr := getMessageProc.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) == -1 {
			return Rectangle{}, fmt.Errorf("read region selector input: %w", messageErr)
		}
		if result == 0 {
			break
		}
		translateMessageProc.Call(uintptr(unsafe.Pointer(&message)))
		dispatchMessageProc.Call(uintptr(unsafe.Pointer(&message)))
	}
	if err := ctx.Err(); err != nil {
		return Rectangle{}, err
	}
	if state.cancelled || !state.selected {
		return Rectangle{}, ErrCancelled
	}
	left := minimumInt(state.anchor.X, state.current.X) + int(virtual.Left)
	top := minimumInt(state.anchor.Y, state.current.Y) + int(virtual.Top)
	return Rectangle{
		X: left, Y: top,
		Width:  absoluteInt(state.current.X - state.anchor.X),
		Height: absoluteInt(state.current.Y - state.anchor.Y),
	}, nil
}

func regionSelectorProc(hwnd, message, wParam, lParam uintptr) uintptr {
	state := activeRegionSelector
	if state == nil {
		result, _, _ := defWindowProc.Call(hwnd, message, wParam, lParam)
		return result
	}
	point := cursorClientPoint(hwnd, lParam)
	switch message {
	case wmLButtonDown:
		state.anchor, state.current, state.dragging = point, point, true
		state.cancelled = false
		setCaptureProc.Call(hwnd)
		invalidateRectProc.Call(hwnd, 0, 1)
		return 0
	case wmMouseMove:
		if state.dragging {
			state.current = point
			invalidateRectProc.Call(hwnd, 0, 1)
		}
		return 0
	case wmLButtonUp:
		if state.dragging {
			state.current = point
			state.dragging = false
			releaseCaptureProc.Call()
			state.selected = absoluteInt(state.current.X-state.anchor.X) > 0 && absoluteInt(state.current.Y-state.anchor.Y) > 0
			state.cancelled = !state.selected
			destroyWindowProc.Call(hwnd)
		}
		return 0
	case wmRButtonDown, wmClose:
		state.cancelled = true
		destroyWindowProc.Call(hwnd)
		return 0
	case wmKeyDown:
		if wParam == vkEscape {
			state.cancelled = true
			destroyWindowProc.Call(hwnd)
			return 0
		}
	case wmPaint:
		var paint paintStruct
		dc, _, _ := beginPaintProc.Call(hwnd, uintptr(unsafe.Pointer(&paint)))
		if dc != 0 && (state.dragging || state.selected) {
			pen, _, _ := createPenProc.Call(penSolid, 3, 0x00FFFF00) // cyan in COLORREF order
			hollow, _, _ := getStockObjectProc.Call(stockHollowBrush)
			previousPen, _, _ := selectObjectProc.Call(dc, pen)
			previousBrush, _, _ := selectObjectProc.Call(dc, hollow)
			left, top := minimumInt(state.anchor.X, state.current.X), minimumInt(state.anchor.Y, state.current.Y)
			right, bottom := maximumInt(state.anchor.X, state.current.X), maximumInt(state.anchor.Y, state.current.Y)
			rectangleProc.Call(dc, uintptr(left), uintptr(top), uintptr(right), uintptr(bottom))
			selectObjectProc.Call(dc, previousPen)
			selectObjectProc.Call(dc, previousBrush)
			deleteObjectProc.Call(pen)
		}
		endPaintProc.Call(hwnd, uintptr(unsafe.Pointer(&paint)))
		return 0
	case wmDestroy:
		postQuitMessageProc.Call(0)
		return 0
	}
	result, _, _ := defWindowProc.Call(hwnd, message, wParam, lParam)
	return result
}

// Mouse-message coordinates are only 16-bit. Reading the cursor's full
// screen coordinate and converting it to the overlay client area keeps
// selection correct on wide multi-monitor virtual desktops.
func cursorClientPoint(hwnd, lParam uintptr) regionPoint {
	point := struct{ X, Y int32 }{}
	if result, _, _ := getCursorPosProc.Call(uintptr(unsafe.Pointer(&point))); result != 0 {
		if result, _, _ := screenToClientProc.Call(hwnd, uintptr(unsafe.Pointer(&point))); result != 0 {
			return regionPoint{X: int(point.X), Y: int(point.Y)}
		}
	}
	return regionPoint{X: int(int16(lParam & 0xffff)), Y: int(int16((lParam >> 16) & 0xffff))}
}

func minimumInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maximumInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func minimumInt32(left, right int32) int32 {
	if left < right {
		return left
	}
	return right
}

func maximumInt32(left, right int32) int32 {
	if left > right {
		return left
	}
	return right
}

func absoluteInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func captureRectangle(x int, y int, width int, height int) (image.Image, error) {
	screenDC, _, callErr := getDCProc.Call(0)
	if screenDC == 0 {
		return nil, fmt.Errorf("open screen device context: %w", callErr)
	}
	defer releaseDCProc.Call(0, screenDC)
	memoryDC, _, callErr := createCompatibleDCProc.Call(screenDC)
	if memoryDC == 0 {
		return nil, fmt.Errorf("create capture device context: %w", callErr)
	}
	defer deleteDCProc.Call(memoryDC)
	bitmap, _, callErr := createCompatibleBitmapProc.Call(screenDC, uintptr(width), uintptr(height))
	if bitmap == 0 {
		return nil, fmt.Errorf("create capture bitmap: %w", callErr)
	}
	defer deleteObjectProc.Call(bitmap)
	previous, _, _ := selectObjectProc.Call(memoryDC, bitmap)
	if previous == 0 {
		return nil, fmt.Errorf("select capture bitmap")
	}
	defer selectObjectProc.Call(memoryDC, previous)
	result, _, callErr := bitBltProc.Call(
		memoryDC, 0, 0, uintptr(width), uintptr(height), screenDC,
		uintptr(int64(x)), uintptr(int64(y)), sourceCopy|captureLayeredWindows,
	)
	if result == 0 {
		return nil, fmt.Errorf("copy screen pixels: %w", callErr)
	}

	buffer := make([]byte, width*height*4)
	info := bitmapInfo{Header: bitmapInfoHeader{
		Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: int32(width), Height: -int32(height),
		Planes: 1, BitCount: 32,
	}}
	result, _, callErr = getDIBitsProc.Call(
		memoryDC, bitmap, 0, uintptr(height), uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&info)), dibRGBColors,
	)
	if result == 0 {
		return nil, fmt.Errorf("read captured pixels: %w", callErr)
	}
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	for index := 0; index < len(buffer); index += 4 {
		rgba.Pix[index] = buffer[index+2]
		rgba.Pix[index+1] = buffer[index+1]
		rgba.Pix[index+2] = buffer[index]
		rgba.Pix[index+3] = 255
	}
	runtime.KeepAlive(buffer)
	return rgba, nil
}
