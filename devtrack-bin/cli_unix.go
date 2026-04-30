//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// setSetsid detaches cmd from the parent process group by creating a new session.
// This is the Unix equivalent of Windows CREATE_NEW_PROCESS_GROUP.
func setSetsid(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}

// sendForceTriggerSignal sends SIGUSR2 to the daemon process to request an
// immediate trigger. On Unix this is a real-time signal; on Windows the stub
// uses the HTTP trigger endpoint instead.
func sendForceTriggerSignal(process *os.Process) error {
	if err := process.Signal(syscall.SIGUSR2); err != nil {
		return fmt.Errorf("could not send SIGUSR2 to daemon: %w", err)
	}
	return nil
}
