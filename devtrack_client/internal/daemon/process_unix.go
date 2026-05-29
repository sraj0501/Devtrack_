//go:build !windows

package daemon

import (
	"os"
	"syscall"
)

func CheckProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
