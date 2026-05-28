package main

// health_shim.go — forwarding aliases from package main to internal/health.

import (
	ihealth "github.com/sraj0501/Devtrack_/devtrack_client/internal/health"
)

// ── Type alias ────────────────────────────────────────────────────────────────

type HealthMonitor = ihealth.HealthMonitor

// ── Function forwards ─────────────────────────────────────────────────────────

func NewHealthMonitor(db *Database) *HealthMonitor {
	hm := ihealth.NewHealthMonitor(db)
	// Wire platform-specific process check (defined in process_unix.go / process_windows.go)
	hm.PingServerFn = func() bool {
		c := NewHTTPTriggerClient()
		return c.Ping()
	}
	hm.GetServerURL = GetServerURL
	return hm
}

func init() {
	// Wire platform-specific process-alive check into internal/health.
	// checkProcessAlive is defined in process_unix.go / process_windows.go.
	ihealth.IsProcessAlive = checkProcessAlive
}

// isProcessAlive is a package main convenience wrapper around the platform-specific
// checkProcessAlive function. Used by upgrade.go and other callers that were
// previously calling the function defined in health.go.
func isProcessAlive(pid int) bool {
	return checkProcessAlive(pid)
}
