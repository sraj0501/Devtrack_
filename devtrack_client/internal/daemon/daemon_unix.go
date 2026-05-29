//go:build !windows

package daemon

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// sendStopSignal sends SIGTERM to the target process, allowing graceful shutdown.
func sendStopSignal(process *os.Process) error {
	return process.Signal(syscall.SIGTERM)
}

// SendReloadSignal sends SIGHUP to the target process, triggering config reload.
func SendReloadSignal(proc *os.Process) error {
	return proc.Signal(syscall.SIGHUP)
}

// setupSignalHandlers sets up handlers for graceful shutdown and force-trigger.
// Unix: listens for SIGUSR2 (force-trigger), SIGHUP (config reload), SIGTERM/Interrupt (shutdown).
func (d *Daemon) setupSignalHandlers() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR2)

	go func() {
		for sig := range sigChan {
			log.Printf("Received signal: %v", sig)

			switch sig {
			case syscall.SIGUSR2:
				// Force immediate trigger (from devtrack force-trigger)
				if d.monitor != nil && d.monitor.Scheduler() != nil {
					log.Println("Force trigger requested via signal")
					d.monitor.Scheduler().ForceImmediate()
				}

			case syscall.SIGHUP:
				// Reload configuration and workspaces
				log.Println("Reloading configuration...")
				if cfg, err := config.LoadConfig(); err == nil {
					d.config = cfg
					log.Println("✓ Configuration reloaded")
				} else {
					log.Printf("Error reloading config: %v", err)
				}
				if d.monitor != nil {
					d.monitor.ReloadWorkspaces()
					// Notify Python to reload its workspace router
					go func() {
						if err := trigger.NewHTTPTriggerClient().SendWorkspaceReload(); err != nil {
							log.Printf("Could not notify Python of workspace reload: %v", err)
						}
					}()
				}

			case os.Interrupt, syscall.SIGTERM:
				// Graceful shutdown
				log.Println("Initiating graceful shutdown...")
				d.cancel()
				return
			}
		}
	}()
}
