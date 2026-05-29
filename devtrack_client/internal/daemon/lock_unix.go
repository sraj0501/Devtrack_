//go:build !windows

package daemon

import (
	"fmt"
	"os"
	"syscall"
)

// acquireDaemonLock acquires an exclusive advisory lock on path using flock(2).
// Returns the open file (must stay open for the lock to be held) or an error if
// another process already holds the lock. The OS releases the lock automatically
// when the process exits for any reason.
func acquireDaemonLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another devtrack instance is already running")
	}
	return f, nil
}

func releaseDaemonLock(f *os.File) {
	if f != nil {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck
		f.Close()
	}
}
