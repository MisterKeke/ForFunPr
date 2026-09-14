//go:build windows

package gitworkspace

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func configureBackgroundCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
}

func containBackgroundProcess(process *os.Process) (func(), error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create Windows job object: %w", err)
	}
	closeJob := func() {
		_ = windows.CloseHandle(job)
	}

	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	result, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		uint32(unsafe.Sizeof(limits)),
	)
	if err != nil || result == 0 {
		closeJob()
		if err == nil {
			err = errors.New("Windows rejected the job object limits")
		}
		return nil, fmt.Errorf("configure Windows job object: %w", err)
	}

	var assignErr error
	handleErr := process.WithHandle(func(handle uintptr) {
		assignErr = windows.AssignProcessToJobObject(job, windows.Handle(handle))
	})
	if handleErr != nil || assignErr != nil {
		closeJob()
		if handleErr != nil {
			return nil, fmt.Errorf("access Git process handle: %w", handleErr)
		}
		return nil, fmt.Errorf("assign Git process to job object: %w", assignErr)
	}
	return closeJob, nil
}
