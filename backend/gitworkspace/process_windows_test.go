//go:build windows

package gitworkspace

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

func TestConfigureBackgroundCommandHidesConsoleWindow(t *testing.T) {
	command := exec.Command("git", "--version")
	configureBackgroundCommand(command)
	if command.SysProcAttr == nil || !command.SysProcAttr.HideWindow {
		t.Fatal("background Git command does not hide its Windows console")
	}
	if command.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatal("background Git command does not use CREATE_NO_WINDOW")
	}
}
