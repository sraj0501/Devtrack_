package tui

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
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
	db      *db.Database
	data    overviewDataMsg
	spinner spinner.Model
	loading bool
	width   int
	height  int
}

func newOverviewModel(database *db.Database) overviewModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorAccent)
	return overviewModel{db: database, spinner: sp, loading: true}
}

func (m overviewModel) load() tea.Cmd {
	return func() tea.Msg {
		msg := overviewDataMsg{
			serverURL: config.GetServerURL(),
			mode:      string(config.GetServerMode()),
		}

		// Daemon status — stat the PID file.
		if fi, err := os.Stat(config.GetPIDFilePath()); err == nil {
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

		// Server health ping.
		client := trigger.NewHTTPTriggerClient()
		start := time.Now()
		msg.serverUp = client.Ping()
		msg.serverLatency = time.Since(start).Milliseconds()

		// Trigger counts today.
		if m.db != nil {
			msg.commits, msg.timers = m.db.CountTriggersToday()
		}

		// Workspace count.
		if cfg, err := config.LoadWorkspacesConfig(); err == nil && cfg != nil {
			msg.workspaces = len(cfg.GetEnabledWorkspaces())
		}

		return msg
	}
}

func (m overviewModel) Update(msg tea.Msg) (overviewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case overviewDataMsg:
		m.data = msg
		m.loading = false
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m overviewModel) View() string {
	if m.loading {
		return "\n  " + m.spinner.View() + " Loading…"
	}

	d := m.data
	cardW := (m.width - 6) / 2

	// Daemon card.
	var daemonStatus string
	if d.daemonRunning {
		dot := lipgloss.NewStyle().Foreground(ColorSuccess).Render("●")
		uptime := ""
		if d.daemonUptime != "" {
			uptime = StyleMuted.Render("  uptime: " + d.daemonUptime)
		}
		daemonStatus = dot + lipgloss.NewStyle().Foreground(ColorSuccess).Render(" running") + uptime
	} else {
		daemonStatus = lipgloss.NewStyle().Foreground(ColorDanger).Render("● stopped")
	}
	daemonCard := StyleCard.Width(cardW).Render(
		StyleSection.Render("DAEMON") + "\n" + daemonStatus,
	)

	// Server card.
	var serverStatus string
	if d.serverUp {
		check := lipgloss.NewStyle().Foreground(ColorSuccess).Render("✓")
		serverStatus = check + lipgloss.NewStyle().Foreground(ColorSuccess).Render(" up") +
			StyleMuted.Render(fmt.Sprintf("  %dms  %s", d.serverLatency, d.mode)) +
			"\n" + StyleMuted.Render(d.serverURL)
	} else {
		serverStatus = lipgloss.NewStyle().Foreground(ColorDanger).Render("✗ unreachable")
	}
	serverCard := StyleCard.Width(cardW).Render(
		StyleSection.Render("AI SERVER") + "\n" + serverStatus,
	)

	// Side-by-side layout.
	cards := lipgloss.JoinHorizontal(lipgloss.Top, daemonCard, " ", serverCard)

	// Metrics strip — full-width card.
	metricsContent := fmt.Sprintf("  %d commits   │   %d timers   │   %d workspaces",
		d.commits, d.timers, d.workspaces)
	metricsCard := StyleCard.Width(m.width - 4).Render(metricsContent)

	return "\n" + cards + "\n\n" + metricsCard
}
