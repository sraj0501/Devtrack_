//go:build windows

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

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
