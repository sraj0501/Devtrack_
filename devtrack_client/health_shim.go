package main

// health_shim.go — package-main convenience wrapper around the platform-specific
// process-alive check, which now lives in internal/daemon (process_unix.go /
// process_windows.go) and is exported as CheckProcessAlive.
//
// The HealthMonitor type and its wiring (PingServerFn / GetServerURL /
// IsProcessAlive) now live entirely in internal/daemon and internal/health — the
// daemon package's init() wires internal/health.IsProcessAlive at startup.

// isProcessAlive reports whether a process with the given PID is running.
// Used by upgrade.go and other lifecycle callers in package main.
func isProcessAlive(pid int) bool {
	return CheckProcessAlive(pid)
}
