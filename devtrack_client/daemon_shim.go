package main

// daemon_shim.go — forwarding aliases from package main to internal/daemon.
// The daemon lifecycle, internal HTTP API, PID lock, process checks, message
// queue, and telemetry ping live in internal/daemon; the CLI reaches them
// through these aliases.

import (
	"os"

	dmn "github.com/sraj0501/Devtrack_/devtrack_client/internal/daemon"
)

// ── Type aliases ─────────────────────────────────────────────────────────────

type Daemon = dmn.Daemon
type DaemonStatus = dmn.DaemonStatus

// ── Const forward ────────────────────────────────────────────────────────────

const RouteInternalReloadConfig = dmn.RouteInternalReloadConfig

// ── Function forwards ─────────────────────────────────────────────────────────

func NewDaemon(repoPath string) (*Daemon, error) {
	d, err := dmn.NewDaemon(repoPath)
	if err != nil {
		return nil, err
	}
	// Wire ticket-sync callbacks: package main owns the connector code,
	// so we set the functions here where both sides are visible.
	db, dbErr := NewDatabase()
	if dbErr == nil {
		d.PushCachedFn = func() { PushCachedTickets(db) }
		d.FullSyncFn = func() { SyncAllTickets(db, false) }
	}
	return d, nil
}
func KillDaemon(pidFile string) error              { return dmn.KillDaemon(pidFile) }
func SendActivePingIfDue()                         { dmn.SendActivePingIfDue() }
func SetTelemetryEnabled(on bool) error            { return dmn.SetTelemetryEnabled(on) }
func TelemetryEnabled() bool                       { return dmn.TelemetryEnabled() }
func CheckProcessAlive(pid int) bool               { return dmn.CheckProcessAlive(pid) }
func SendReloadSignal(proc *os.Process) error      { return dmn.SendReloadSignal(proc) }
