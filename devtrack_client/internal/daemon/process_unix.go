//go:build !windows

package daemon

import (
	"errors"
	"os"
	"syscall"
)

func CheckProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

// KillProcessTree terminates pid. On POSIX, "uv run ... python ..." execs
// into the python process in place rather than spawning a separate child
// (unlike Windows), so a single Kill() already reaches the real server.
func KillProcessTree(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}
