//go:build windows

package daemon

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// sendStopSignal terminates the target process on Windows.
// SIGTERM is not sendable to other processes on Windows; Kill() (TerminateProcess) is equivalent.
func sendStopSignal(process *os.Process) error {
	return process.Kill()
}

// SendReloadSignal is a no-op on Windows — SIGHUP is not supported.
// Use `devtrack restart` to reload configuration on Windows.
func SendReloadSignal(_ *os.Process) error {
	return fmt.Errorf("workspace reload via signal is not supported on Windows; run: devtrack restart")
}

// setupSignalHandlers sets up handlers for graceful shutdown.
// Windows: SIGUSR2 and SIGHUP are not available; force-trigger uses HTTP instead.
// Only os.Interrupt and SIGTERM are handled here.
func (d *Daemon) setupSignalHandlers() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		for sig := range sigChan {
			log.Printf("Received signal: %v", sig)

			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				// Graceful shutdown
				log.Println("Initiating graceful shutdown...")
				d.cancel()
				return
			}
		}
	}()
}
