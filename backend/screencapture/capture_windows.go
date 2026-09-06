//go:build windows

package screencapture

import (
	"fmt"
	"image"
	"runtime"
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
)

var (
	captureUser32DLL           = windows.NewLazySystemDLL("user32.dll")
	captureGDI32DLL            = windows.NewLazySystemDLL("gdi32.dll")
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
)

type windowsCapturer struct{}

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

func (windowsCapturer) Capture(mode string) (image.Image, error) {
	var bounds winRect
	switch mode {
	case "screen", "region":
		x, _, _ := getSystemMetricsProc.Call(systemMetricXVirtualScreen)
		y, _, _ := getSystemMetricsProc.Call(systemMetricYVirtualScreen)
		width, _, _ := getSystemMetricsProc.Call(systemMetricCXVirtualScreen)
		height, _, _ := getSystemMetricsProc.Call(systemMetricCYVirtualScreen)
		bounds = winRect{Left: int32(x), Top: int32(y), Right: int32(x) + int32(width), Bottom: int32(y) + int32(height)}
	case "window":
		window, _, _ := getForegroundWindowProc.Call()
		if window == 0 {
			return nil, fmt.Errorf("no foreground window is available")
		}
		result, _, callErr := getWindowRectProc.Call(window, uintptr(unsafe.Pointer(&bounds)))
		if result == 0 {
			return nil, fmt.Errorf("read foreground window bounds: %w", callErr)
		}
	default:
		return nil, fmt.Errorf("unsupported capture mode %q", mode)
	}
	width := int(bounds.Right - bounds.Left)
	height := int(bounds.Bottom - bounds.Top)
	if width <= 0 || height <= 0 || width > 32768 || height > 32768 {
		return nil, fmt.Errorf("invalid capture bounds %dx%d", width, height)
	}
	return captureRectangle(int(bounds.Left), int(bounds.Top), width, height)
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
