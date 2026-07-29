//go:build windows

package backend

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	openAsExecute                     = 0x00000004
	classContextInProcServer          = 0x1
	fileOperationNoConfirmation       = 0x0010
	fileOperationSilent               = 0x0004
	fileOperationNoErrorUI            = 0x0400
	fileOperationRecycleDelete        = 0x00080000
	fileOperationAddUndoRecord        = 0x20000000
	windowsErrorCancelled             = 1223
	shellExecuteAssociationIncomplete = 27
	shellExecuteNoAssociation         = 31
)

var (
	shell32DLL                      = windows.NewLazySystemDLL("shell32.dll")
	ole32DLL                        = windows.NewLazySystemDLL("ole32.dll")
	procShellExecuteW               = shell32DLL.NewProc("ShellExecuteW")
	procSHOpenWithDialog            = shell32DLL.NewProc("SHOpenWithDialog")
	procSHCreateItemFromParsingName = shell32DLL.NewProc("SHCreateItemFromParsingName")
	procCoCreateInstance            = ole32DLL.NewProc("CoCreateInstance")

	classIDFileOperation = windows.GUID{
		Data1: 0x3ad05575,
		Data2: 0x8857,
		Data3: 0x4850,
		Data4: [8]byte{0x92, 0x77, 0x11, 0xb8, 0x5b, 0xdb, 0x8e, 0x09},
	}
	interfaceIDFileOperation = windows.GUID{
		Data1: 0x947aab5f,
		Data2: 0x0a5c,
		Data3: 0x4c13,
		Data4: [8]byte{0xb4, 0xd6, 0x4b, 0xf7, 0x83, 0x6f, 0xc9, 0xf8},
	}
	interfaceIDShellItem = windows.GUID{
		Data1: 0x43826d1e,
		Data2: 0xe718,
		Data3: 0x42ee,
		Data4: [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe},
	}
)

type windowsFileExplorerShell struct{}

type openAsInfo struct {
	file  *uint16
	class *uint16
	flags uint32
}

type iUnknownVTable struct {
	queryInterface uintptr
	addRef         uintptr
	release        uintptr
}

type shellItem struct {
	vtable *iUnknownVTable
}

type fileOperation struct {
	vtable *fileOperationVTable
}

type fileOperationVTable struct {
	iUnknownVTable
	advise                  uintptr
	unadvise                uintptr
	setOperationFlags       uintptr
	setProgressMessage      uintptr
	setProgressDialog       uintptr
	setProperties           uintptr
	setOwnerWindow          uintptr
	applyPropertiesToItem   uintptr
	applyPropertiesToItems  uintptr
	renameItem              uintptr
	renameItems             uintptr
	moveItem                uintptr
	moveItems               uintptr
	copyItem                uintptr
	copyItems               uintptr
	deleteItem              uintptr
	deleteItems             uintptr
	newItem                 uintptr
	performOperations       uintptr
	getAnyOperationsAborted uintptr
}

func newFileExplorerShell() fileExplorerShell {
	return windowsFileExplorerShell{}
}

func (windowsFileExplorerShell) OpenFile(path string) error {
	return runExplorerShellSTA(func() error {
		pathPointer, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		verbPointer, err := windows.UTF16PtrFromString("open")
		if err != nil {
			return err
		}
		result, _, _ := procShellExecuteW.Call(
			0,
			uintptr(unsafe.Pointer(verbPointer)),
			uintptr(unsafe.Pointer(pathPointer)),
			0,
			0,
			uintptr(windows.SW_SHOWNORMAL),
		)
		runtime.KeepAlive(verbPointer)
		runtime.KeepAlive(pathPointer)
		if result > 32 {
			return nil
		}
		if result == shellExecuteAssociationIncomplete || result == shellExecuteNoAssociation {
			return showWindowsOpenWith(pathPointer)
		}
		switch result {
		case 2, 3:
			return osPathError("open", path, syscall.ENOENT)
		case 5:
			return osPathError("open", path, syscall.EACCES)
		default:
			return fmt.Errorf("Windows could not open the file (ShellExecute code %d)", result)
		}
	})
}

func (windowsFileExplorerShell) RecycleFile(path string) error {
	return runExplorerShellSTA(func() error {
		pathPointer, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		item, err := createShellItem(pathPointer)
		if err != nil {
			return err
		}
		defer item.release()

		operation, err := createFileOperation()
		if err != nil {
			return err
		}
		defer operation.release()

		flags := uintptr(fileOperationNoConfirmation |
			fileOperationSilent |
			fileOperationNoErrorUI |
			fileOperationRecycleDelete |
			fileOperationAddUndoRecord)
		if err := callHRESULT(operation.vtable.setOperationFlags, uintptr(unsafe.Pointer(operation)), flags); err != nil {
			return err
		}
		if err := callHRESULT(
			operation.vtable.deleteItem,
			uintptr(unsafe.Pointer(operation)),
			uintptr(unsafe.Pointer(item)),
			0,
		); err != nil {
			return err
		}
		if err := callHRESULT(operation.vtable.performOperations, uintptr(unsafe.Pointer(operation))); err != nil {
			return err
		}
		var aborted int32
		if err := callHRESULT(
			operation.vtable.getAnyOperationsAborted,
			uintptr(unsafe.Pointer(operation)),
			uintptr(unsafe.Pointer(&aborted)),
		); err != nil {
			return err
		}
		if aborted != 0 {
			return errors.New("the Recycle Bin operation was cancelled")
		}
		runtime.KeepAlive(pathPointer)
		return nil
	})
}

func showWindowsOpenWith(pathPointer *uint16) error {
	info := openAsInfo{file: pathPointer, flags: openAsExecute}
	result, _, _ := procSHOpenWithDialog.Call(0, uintptr(unsafe.Pointer(&info)))
	if hresultSucceeded(result) || hresultCode(result) == hresultFromWin32(windowsErrorCancelled) {
		return nil
	}
	return fmt.Errorf("Windows could not show application suggestions (HRESULT 0x%08X)", hresultCode(result))
}

func createShellItem(pathPointer *uint16) (*shellItem, error) {
	var item *shellItem
	result, _, _ := procSHCreateItemFromParsingName.Call(
		uintptr(unsafe.Pointer(pathPointer)),
		0,
		uintptr(unsafe.Pointer(&interfaceIDShellItem)),
		uintptr(unsafe.Pointer(&item)),
	)
	if !hresultSucceeded(result) {
		return nil, fmt.Errorf("create Windows shell item (HRESULT 0x%08X)", hresultCode(result))
	}
	if item == nil {
		return nil, errors.New("Windows did not create a shell item")
	}
	return item, nil
}

func createFileOperation() (*fileOperation, error) {
	var operation *fileOperation
	result, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&classIDFileOperation)),
		0,
		classContextInProcServer,
		uintptr(unsafe.Pointer(&interfaceIDFileOperation)),
		uintptr(unsafe.Pointer(&operation)),
	)
	if !hresultSucceeded(result) {
		return nil, fmt.Errorf("create Windows file operation (HRESULT 0x%08X)", hresultCode(result))
	}
	if operation == nil {
		return nil, errors.New("Windows did not create a file operation")
	}
	return operation, nil
}

func runExplorerShellSTA(operation func() error) error {
	result := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		initializeError := windows.CoInitializeEx(
			0,
			windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE,
		)
		initialized := initializeError == nil || errors.Is(initializeError, syscall.Errno(1))
		if !initialized {
			result <- fmt.Errorf("initialize Windows shell services: %w", initializeError)
			return
		}
		defer windows.CoUninitialize()
		result <- operation()
	}()
	return <-result
}

func callHRESULT(method uintptr, arguments ...uintptr) error {
	result, _, _ := syscall.SyscallN(method, arguments...)
	if hresultSucceeded(result) {
		return nil
	}
	return fmt.Errorf("Windows file operation failed (HRESULT 0x%08X)", hresultCode(result))
}

func (item *shellItem) release() {
	if item != nil && item.vtable != nil {
		syscall.SyscallN(item.vtable.release, uintptr(unsafe.Pointer(item)))
	}
}

func (operation *fileOperation) release() {
	if operation != nil && operation.vtable != nil {
		syscall.SyscallN(operation.vtable.release, uintptr(unsafe.Pointer(operation)))
	}
}

func hresultCode(result uintptr) uint32 {
	return uint32(result)
}

func hresultSucceeded(result uintptr) bool {
	return int32(hresultCode(result)) >= 0
}

func hresultFromWin32(code uint32) uint32 {
	if code == 0 {
		return 0
	}
	return 0x80070000 | code
}

func osPathError(operation string, path string, err error) error {
	return &os.PathError{Op: operation, Path: path, Err: err}
}
