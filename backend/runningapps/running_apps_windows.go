//go:build windows

package runningapps

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	gaRootOwner                    = 3
	gwlExStyle                     = -20
	gclpHIcon                      = -14
	gclpHIconSmall                 = -34
	dwmwaCloaked                   = 14
	wmGetIcon                      = 0x007f
	iconSmall                      = 0
	iconBig                        = 1
	iconSmall2                     = 2
	sendMessageTimeoutAbortIfHung  = 0x0002
	sendMessageTimeoutMilliseconds = 120
	processQueryLimitedInformation = 0x1000
	swRestore                      = 9
	shgfiIcon                      = 0x000000100
	shgfiSmallIcon                 = 0x000000001
	dibRGBColors                   = 0
	iconRenderSize                 = 32
	iconPixelBytes                 = iconRenderSize * iconRenderSize * 4
	maxIconPNGBytes                = 256 * 1024
	diNormal                       = 0x0003
)

var (
	runningUser32DLL                 = windows.NewLazySystemDLL("user32.dll")
	runningKernel32DLL               = windows.NewLazySystemDLL("kernel32.dll")
	runningGDI32DLL                  = windows.NewLazySystemDLL("gdi32.dll")
	runningDWMAPIDLL                 = windows.NewLazySystemDLL("dwmapi.dll")
	runningShell32DLL                = windows.NewLazySystemDLL("shell32.dll")
	runningEnumWindowsProc           = runningUser32DLL.NewProc("EnumWindows")
	runningIsWindowProc              = runningUser32DLL.NewProc("IsWindow")
	runningIsWindowVisibleProc       = runningUser32DLL.NewProc("IsWindowVisible")
	runningIsIconicProc              = runningUser32DLL.NewProc("IsIconic")
	runningGetWindowTextLengthProc   = runningUser32DLL.NewProc("GetWindowTextLengthW")
	runningGetWindowTextProc         = runningUser32DLL.NewProc("GetWindowTextW")
	runningGetWindowThreadProcessID  = runningUser32DLL.NewProc("GetWindowThreadProcessId")
	runningGetForegroundWindowProc   = runningUser32DLL.NewProc("GetForegroundWindow")
	runningGetWindowLongPtrProc      = runningUser32DLL.NewProc("GetWindowLongPtrW")
	runningGetAncestorProc           = runningUser32DLL.NewProc("GetAncestor")
	runningGetLastActivePopupProc    = runningUser32DLL.NewProc("GetLastActivePopup")
	runningGetClassNameProc          = runningUser32DLL.NewProc("GetClassNameW")
	runningSendMessageTimeoutProc    = runningUser32DLL.NewProc("SendMessageTimeoutW")
	runningGetClassLongPtrProc       = runningUser32DLL.NewProc("GetClassLongPtrW")
	runningCopyIconProc              = runningUser32DLL.NewProc("CopyIcon")
	runningDestroyIconProc           = runningUser32DLL.NewProc("DestroyIcon")
	runningGetDCProc                 = runningUser32DLL.NewProc("GetDC")
	runningReleaseDCProc             = runningUser32DLL.NewProc("ReleaseDC")
	runningDrawIconExProc            = runningUser32DLL.NewProc("DrawIconEx")
	runningShowWindowAsyncProc       = runningUser32DLL.NewProc("ShowWindowAsync")
	runningSetForegroundWindowProc   = runningUser32DLL.NewProc("SetForegroundWindow")
	runningBringWindowToTopProc      = runningUser32DLL.NewProc("BringWindowToTop")
	runningQueryFullProcessImageName = runningKernel32DLL.NewProc("QueryFullProcessImageNameW")
	runningCreateCompatibleDCProc    = runningGDI32DLL.NewProc("CreateCompatibleDC")
	runningDeleteDCProc              = runningGDI32DLL.NewProc("DeleteDC")
	runningCreateDIBSectionProc      = runningGDI32DLL.NewProc("CreateDIBSection")
	runningSelectObjectProc          = runningGDI32DLL.NewProc("SelectObject")
	runningDeleteObjectProc          = runningGDI32DLL.NewProc("DeleteObject")
	runningDwmGetWindowAttributeProc = runningDWMAPIDLL.NewProc("DwmGetWindowAttribute")
	runningSHGetFileInfoProc         = runningShell32DLL.NewProc("SHGetFileInfoW")
)

type windowsProvider struct {
	mu    sync.Mutex
	icons map[iconCacheKey]string
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

type shFileInfo struct {
	Icon        uintptr
	IconIndex   int32
	Attributes  uint32
	DisplayName [260]uint16
	TypeName    [80]uint16
}

func New() Provider {
	return &windowsProvider{icons: make(map[iconCacheKey]string)}
}

func (*windowsProvider) Supported() bool {
	return true
}

func (p *windowsProvider) List(ctx context.Context) ([]App, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	currentProcessID := uint32(os.Getpid())
	snapshots, err := enumerateWindowSnapshots(ctx, currentProcessID)
	if err != nil {
		return nil, err
	}
	apps, presentIcons, err := buildAppList(ctx, snapshots, currentProcessID, p.cachedIconDataURL)
	if err != nil {
		return nil, err
	}
	p.pruneIconCache(presentIcons)
	return apps, nil
}

func (p *windowsProvider) Activate(ctx context.Context, windowID string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	handle, err := parseWindowID(windowID)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !windowExists(handle) {
		return ErrWindowUnavailable
	}

	currentProcessID := uint32(os.Getpid())
	snapshots, err := enumerateWindowSnapshots(ctx, currentProcessID)
	if err != nil {
		return err
	}
	snapshot, err := resolveActivatableSnapshot(ctx, windowID, snapshots, currentProcessID)
	if err != nil {
		return err
	}
	if !windowExists(handle) {
		return ErrWindowUnavailable
	}
	if snapshot.Minimized {
		runningShowWindowAsyncProc.Call(handle, swRestore)
	}
	runningBringWindowToTopProc.Call(handle)
	result, _, _ := runningSetForegroundWindowProc.Call(handle)
	if result == 0 {
		return ErrActivationRefused
	}
	return ctx.Err()
}

func enumerateWindowSnapshots(ctx context.Context, currentProcessID uint32) ([]windowSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	foreground := foregroundWindow()
	snapshots := make([]windowSnapshot, 0, 32)
	var callbackErr error
	callback := syscall.NewCallback(func(handle uintptr, lparam uintptr) uintptr {
		if err := ctx.Err(); err != nil {
			callbackErr = err
			return 0
		}
		snapshot := readWindowSnapshot(handle, foreground)
		if shouldReadProcessMetadata(snapshot, currentProcessID) {
			snapshot.ProcessName, snapshot.ProcessPath = readProcessMetadata(snapshot.ProcessID)
		}
		snapshots = append(snapshots, snapshot)
		return 1
	})
	result, _, callErr := runningEnumWindowsProc.Call(callback, 0)
	runtime.KeepAlive(callback)
	if callbackErr != nil {
		return nil, callbackErr
	}
	if result == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno != 0 {
			return nil, fmt.Errorf("enumerate Windows taskbar applications: %w", callErr)
		}
		return nil, fmt.Errorf("Windows could not enumerate taskbar applications")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return snapshots, nil
}

func readWindowSnapshot(handle uintptr, foreground uintptr) windowSnapshot {
	snapshot := windowSnapshot{Handle: handle}
	if !windowExists(handle) {
		return snapshot
	}
	snapshot.Visible = windowVisible(handle)
	snapshot.Cloaked = windowCloaked(handle)
	snapshot.ExStyle = windowExStyle(handle)
	snapshot.ProcessID = windowProcessID(handle)
	snapshot.ClassName = windowClassName(handle)
	snapshot.Title = windowTitle(handle)
	snapshot.Minimized = windowMinimized(handle)
	snapshot.Active = handle != 0 && handle == foreground
	snapshot.TaskbarRepresentative = windowIsTaskbarRepresentative(handle)
	return snapshot
}

func shouldReadProcessMetadata(snapshot windowSnapshot, currentProcessID uint32) bool {
	if snapshot.Handle == 0 || !snapshot.Visible || snapshot.Cloaked {
		return false
	}
	if currentProcessID != 0 && snapshot.ProcessID == currentProcessID {
		return false
	}
	if snapshot.ProcessID == 0 || isShellSurfaceClass(snapshot.ClassName) {
		return false
	}
	hasAppWindow := snapshot.ExStyle&wsExAppWindow != 0
	hasToolWindow := snapshot.ExStyle&wsExToolWindow != 0
	if hasToolWindow && !hasAppWindow {
		return false
	}
	if !hasAppWindow && !snapshot.TaskbarRepresentative {
		return false
	}
	return true
}

func windowExists(handle uintptr) bool {
	result, _, _ := runningIsWindowProc.Call(handle)
	return result != 0
}

func windowVisible(handle uintptr) bool {
	result, _, _ := runningIsWindowVisibleProc.Call(handle)
	return result != 0
}

func windowMinimized(handle uintptr) bool {
	result, _, _ := runningIsIconicProc.Call(handle)
	return result != 0
}

func foregroundWindow() uintptr {
	handle, _, _ := runningGetForegroundWindowProc.Call()
	return handle
}

func windowProcessID(handle uintptr) uint32 {
	var processID uint32
	runningGetWindowThreadProcessID.Call(handle, uintptr(unsafe.Pointer(&processID)))
	return processID
}

func windowExStyle(handle uintptr) uintptr {
	result, _, _ := runningGetWindowLongPtrProc.Call(handle, signedIndex(gwlExStyle))
	return result
}

func windowClassName(handle uintptr) string {
	buffer := make([]uint16, 256)
	copied, _, _ := runningGetClassNameProc.Call(
		handle,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
	)
	if copied == 0 {
		return ""
	}
	return windows.UTF16ToString(buffer[:copied])
}

func windowTitle(handle uintptr) string {
	lengthResult, _, _ := runningGetWindowTextLengthProc.Call(handle)
	length := int(lengthResult)
	if length <= 0 {
		return ""
	}
	if length > 4096 {
		length = 4096
	}
	buffer := make([]uint16, length+1)
	copied, _, _ := runningGetWindowTextProc.Call(
		handle,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
	)
	if copied == 0 {
		return ""
	}
	return windows.UTF16ToString(buffer[:copied])
}

func windowCloaked(handle uintptr) bool {
	if err := runningDwmGetWindowAttributeProc.Find(); err != nil {
		return false
	}
	var cloaked uint32
	result, _, _ := runningDwmGetWindowAttributeProc.Call(
		handle,
		dwmwaCloaked,
		uintptr(unsafe.Pointer(&cloaked)),
		unsafe.Sizeof(cloaked),
	)
	if int32(uint32(result)) < 0 {
		return false
	}
	return cloaked != 0
}

func windowIsTaskbarRepresentative(handle uintptr) bool {
	rootOwner, _, _ := runningGetAncestorProc.Call(handle, gaRootOwner)
	if rootOwner == 0 {
		rootOwner = handle
	}
	return lastVisibleActivePopup(rootOwner, getLastActivePopup, windowVisible) == handle
}

func getLastActivePopup(handle uintptr) uintptr {
	result, _, _ := runningGetLastActivePopupProc.Call(handle)
	return result
}

func signedIndex(value int32) uintptr {
	return uintptr(int(value))
}

func readProcessMetadata(processID uint32) (string, string) {
	handle, err := windows.OpenProcess(processQueryLimitedInformation, false, processID)
	if err != nil {
		return "", ""
	}
	defer windows.CloseHandle(handle)

	path := queryFullProcessImageName(handle)
	if path == "" {
		return "", ""
	}
	name := filepath.Base(path)
	if name == "." || name == string(filepath.Separator) {
		return "", path
	}
	return name, path
}

func queryFullProcessImageName(handle windows.Handle) string {
	for _, bufferSize := range []int{260, 32768} {
		buffer := make([]uint16, bufferSize)
		length := uint32(len(buffer))
		result, _, _ := runningQueryFullProcessImageName.Call(
			uintptr(handle),
			0,
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(unsafe.Pointer(&length)),
		)
		if result != 0 && length > 0 && int(length) <= len(buffer) {
			return windows.UTF16ToString(buffer[:length])
		}
	}
	return ""
}

func (p *windowsProvider) cachedIconDataURL(ctx context.Context, snapshot windowSnapshot) string {
	if err := ctx.Err(); err != nil {
		return ""
	}
	key := iconCacheKey{WindowID: snapshot.Handle, ProcessID: snapshot.ProcessID}
	p.mu.Lock()
	cached, ok := p.icons[key]
	p.mu.Unlock()
	if ok {
		return cached
	}

	icon := windowIconDataURL(snapshot.Handle, snapshot.ProcessPath)
	p.mu.Lock()
	if p.icons == nil {
		p.icons = make(map[iconCacheKey]string)
	}
	p.icons[key] = icon
	p.mu.Unlock()
	return icon
}

func (p *windowsProvider) pruneIconCache(present map[iconCacheKey]struct{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key := range p.icons {
		if _, ok := present[key]; !ok {
			delete(p.icons, key)
		}
	}
}

func windowIconDataURL(handle uintptr, processPath string) string {
	if icon := iconFromWindowMessage(handle); icon != "" {
		return icon
	}
	if icon := iconFromClass(handle); icon != "" {
		return icon
	}
	if processPath != "" {
		return iconFromFile(processPath)
	}
	return ""
}

func iconFromWindowMessage(handle uintptr) string {
	for _, iconKind := range []uintptr{iconSmall2, iconSmall, iconBig} {
		var iconHandle uintptr
		result, _, _ := runningSendMessageTimeoutProc.Call(
			handle,
			wmGetIcon,
			iconKind,
			0,
			sendMessageTimeoutAbortIfHung,
			sendMessageTimeoutMilliseconds,
			uintptr(unsafe.Pointer(&iconHandle)),
		)
		if result == 0 || iconHandle == 0 {
			continue
		}
		if dataURL := renderBorrowedIconDataURL(iconHandle); dataURL != "" {
			return dataURL
		}
	}
	return ""
}

func iconFromClass(handle uintptr) string {
	for _, classIndex := range []int32{gclpHIconSmall, gclpHIcon} {
		iconHandle, _, _ := runningGetClassLongPtrProc.Call(handle, signedIndex(classIndex))
		if iconHandle == 0 {
			continue
		}
		if dataURL := renderBorrowedIconDataURL(iconHandle); dataURL != "" {
			return dataURL
		}
	}
	return ""
}

func iconFromFile(path string) string {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	var info shFileInfo
	result, _, _ := runningSHGetFileInfoProc.Call(
		uintptr(unsafe.Pointer(pathPointer)),
		0,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
		shgfiIcon|shgfiSmallIcon,
	)
	runtime.KeepAlive(pathPointer)
	if result == 0 || info.Icon == 0 {
		return ""
	}
	defer runningDestroyIconProc.Call(info.Icon)
	return renderIconDataURL(info.Icon)
}

func renderBorrowedIconDataURL(iconHandle uintptr) string {
	owned, _, _ := runningCopyIconProc.Call(iconHandle)
	if owned == 0 {
		return ""
	}
	defer runningDestroyIconProc.Call(owned)
	return renderIconDataURL(owned)
}

func renderIconDataURL(iconHandle uintptr) string {
	screenDC, _, _ := runningGetDCProc.Call(0)
	if screenDC == 0 {
		return ""
	}
	defer runningReleaseDCProc.Call(0, screenDC)

	memoryDC, _, _ := runningCreateCompatibleDCProc.Call(screenDC)
	if memoryDC == 0 {
		return ""
	}
	defer runningDeleteDCProc.Call(memoryDC)

	info := bitmapInfo{Header: bitmapInfoHeader{
		Size:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:    iconRenderSize,
		Height:   -iconRenderSize,
		Planes:   1,
		BitCount: 32,
	}}
	var bits unsafe.Pointer
	bitmap, _, _ := runningCreateDIBSectionProc.Call(
		memoryDC,
		uintptr(unsafe.Pointer(&info)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if bitmap == 0 || bits == nil {
		return ""
	}
	defer runningDeleteObjectProc.Call(bitmap)

	previous, _, _ := runningSelectObjectProc.Call(memoryDC, bitmap)
	if previous == 0 {
		return ""
	}
	defer runningSelectObjectProc.Call(memoryDC, previous)

	result, _, _ := runningDrawIconExProc.Call(
		memoryDC,
		0,
		0,
		iconHandle,
		iconRenderSize,
		iconRenderSize,
		0,
		0,
		diNormal,
	)
	if result == 0 {
		return ""
	}

	rawPixels := unsafe.Slice((*byte)(bits), iconPixelBytes)
	rgba := image.NewRGBA(image.Rect(0, 0, iconRenderSize, iconRenderSize))
	hasAlpha := false
	for index := 3; index < len(rawPixels); index += 4 {
		if rawPixels[index] != 0 {
			hasAlpha = true
			break
		}
	}
	for index := 0; index < len(rawPixels); index += 4 {
		blue := rawPixels[index]
		green := rawPixels[index+1]
		red := rawPixels[index+2]
		alpha := rawPixels[index+3]
		if !hasAlpha && (red != 0 || green != 0 || blue != 0) {
			alpha = 255
		}
		rgba.Pix[index] = red
		rgba.Pix[index+1] = green
		rgba.Pix[index+2] = blue
		rgba.Pix[index+3] = alpha
	}

	var output bytes.Buffer
	if err := png.Encode(&output, rgba); err != nil || output.Len() > maxIconPNGBytes {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(output.Bytes())
}
