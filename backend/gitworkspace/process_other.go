//go:build !windows

package gitworkspace

import (
	"os"
	"os/exec"
)

func configureBackgroundCommand(*exec.Cmd) {}

func containBackgroundProcess(*os.Process) (func(), error) {
	return func() {}, nil
}
