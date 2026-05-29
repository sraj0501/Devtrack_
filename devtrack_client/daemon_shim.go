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

func NewDaemon(repoPath string) (*Daemon, error)  { return dmn.NewDaemon(repoPath) }
func KillDaemon(pidFile string) error              { return dmn.KillDaemon(pidFile) }
func SendActivePingIfDue()                         { dmn.SendActivePingIfDue() }
func CheckProcessAlive(pid int) bool               { return dmn.CheckProcessAlive(pid) }
func SendReloadSignal(proc *os.Process) error      { return dmn.SendReloadSignal(proc) }
