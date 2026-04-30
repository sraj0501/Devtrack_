//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// setSetsid is a Windows stub. On Unix, Setsid creates a new session to detach
// the daemon from the parent terminal. On Windows the equivalent is
// CREATE_NEW_PROCESS_GROUP, which prevents the child from receiving Ctrl+C
// events sent to the parent console.
func setSetsid(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// sendForceTriggerSignal triggers an immediate scheduler run on Windows.
// SIGUSR2 is not available on Windows, so we use the HTTP trigger endpoint
// that the daemon exposes for exactly this purpose.
func sendForceTriggerSignal(process *os.Process) error {
	// Verify the process is still alive before attempting the HTTP call.
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("daemon process is not running: %w", err)
	}
	client := NewHTTPTriggerClient()
	data := TimerTriggerData{
		Timestamp:    "force",
		IntervalMins: 0,
		TriggerCount: 0,
	}
	if err := client.SendTimerTrigger(data); err != nil {
		return fmt.Errorf("could not send force-trigger via HTTP: %w", err)
	}
	fmt.Println("(Windows: force-trigger sent via HTTP endpoint)")
	return nil
}
