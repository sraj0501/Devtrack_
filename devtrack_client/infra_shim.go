package main

// infra_shim.go — forwarding aliases from package main to internal/infra.
// Lets the CLI files keep using bare names while the git monitor, scheduler,
// and Docker-infra provisioning live in internal/infra. Only the symbols the
// CLI (package main) actually references are forwarded here; the daemon and TUI
// packages import internal/infra directly.

import (
	infra "github.com/sraj0501/Devtrack_/devtrack_client/internal/infra"
)

// ── Type aliases ─────────────────────────────────────────────────────────────

type Scheduler = infra.Scheduler
type GitMonitor = infra.GitMonitor
type CommitInfo = infra.CommitInfo
type TriggerEvent = infra.TriggerEvent

// ── Function forwards ─────────────────────────────────────────────────────────

func NewScheduler(cfg *Config, onTrigger func(TriggerEvent)) *Scheduler {
	return infra.NewScheduler(cfg, onTrigger)
}
func NewGitMonitor(repoPath string) (*GitMonitor, error) { return infra.NewGitMonitor(repoPath) }
func EnsureLocalInfra() error                            { return infra.EnsureLocalInfra() }
func IsGitRepository(path string) bool                   { return infra.IsGitRepository(path) }
func InstallPostCommitHook(repoPath string) error        { return infra.InstallPostCommitHook(repoPath) }
func InstallPrePushHook(repoPath string) error           { return infra.InstallPrePushHook(repoPath) }
func TestIntegrated()                                    { infra.TestIntegrated() }
