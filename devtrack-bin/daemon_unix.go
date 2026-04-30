//go:build !windows

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

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
				if d.monitor != nil && d.monitor.scheduler != nil {
					log.Println("Force trigger requested via signal")
					d.monitor.scheduler.ForceImmediate()
				}

			case syscall.SIGHUP:
				// Reload configuration and workspaces
				log.Println("Reloading configuration...")
				if config, err := LoadConfig(); err == nil {
					d.config = config
					log.Println("✓ Configuration reloaded")
				} else {
					log.Printf("Error reloading config: %v", err)
				}
				if d.monitor != nil {
					d.monitor.ReloadWorkspaces()
					// Notify Python to reload its workspace router
					go func() {
						if err := NewHTTPTriggerClient().SendWorkspaceReload(); err != nil {
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
