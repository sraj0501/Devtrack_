package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type overviewDataMsg struct {
	daemonRunning bool
	daemonUptime  string
	serverUp      bool
	serverLatency int64
	serverURL     string
	mode          string
	commits       int
	timers        int
	workspaces    int
}

type overviewModel struct {
	db     *Database
	data   overviewDataMsg
	width  int
	height int
}

func newOverviewModel(db *Database) overviewModel { return overviewModel{db: db} }

func (m overviewModel) load() tea.Cmd {
	return func() tea.Msg {
		msg := overviewDataMsg{
			serverURL: GetServerURL(),
			mode:      string(GetServerMode()),
		}

		// Daemon status — stat the PID file
		if fi, err := os.Stat(GetPIDFilePath()); err == nil {
			msg.daemonRunning = true
			uptime := time.Since(fi.ModTime())
			switch {
			case uptime < time.Minute:
				msg.daemonUptime = fmt.Sprintf("%ds", int(uptime.Seconds()))
			case uptime < time.Hour:
				msg.daemonUptime = fmt.Sprintf("%dm", int(uptime.Minutes()))
			default:
				msg.daemonUptime = fmt.Sprintf("%.1fh", uptime.Hours())
			}
		}

		// Server health ping
		client := NewHTTPTriggerClient()
		start := time.Now()
		msg.serverUp = client.Ping()
		msg.serverLatency = time.Since(start).Milliseconds()

		// Trigger counts today
		if m.db != nil {
			msg.commits, msg.timers = m.db.CountTriggersToday()
		}

		// Workspace count
		if cfg, err := LoadWorkspacesConfig(); err == nil && cfg != nil {
			msg.workspaces = len(cfg.GetEnabledWorkspaces())
		}

		return msg
	}
}

func (m overviewModel) Update(msg tea.Msg) (overviewModel, tea.Cmd) {
	if d, ok := msg.(overviewDataMsg); ok {
		m.data = d
	}
	return m, nil
}

func (m overviewModel) View() string {
	d := m.data
	bold := lipgloss.NewStyle().Bold(true)
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	daemonStatus := red.Render("● stopped")
	if d.daemonRunning {
		extra := ""
		if d.daemonUptime != "" {
			extra = muted.Render(" (" + d.daemonUptime + ")")
		}
		daemonStatus = green.Render("● running") + extra
	}

	serverStatus := red.Render("✗ unreachable")
	if d.serverUp {
		serverStatus = green.Render(fmt.Sprintf("✓ up (%dms)", d.serverLatency))
	}

	out := "\n"
	out += "  " + bold.Render("DevTrack Dashboard") + "\n\n"
	out += fmt.Sprintf("  %-22s %s\n", "Go daemon:", daemonStatus)
	out += fmt.Sprintf("  %-22s %s\n", "Python server:", serverStatus)
	out += fmt.Sprintf("  %-22s %s\n", "Server URL:", muted.Render(d.serverURL))
	out += fmt.Sprintf("  %-22s %s\n", "Mode:", d.mode)
	out += "\n"
	out += fmt.Sprintf("  %-22s %d commits, %d timers\n", "Triggers today:", d.commits, d.timers)
	out += fmt.Sprintf("  %-22s %d active\n", "Workspaces:", d.workspaces)
	return out
}
