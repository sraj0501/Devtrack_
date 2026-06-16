package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/config"
)

type workspacesDataMsg []config.WorkspaceConfig

type workspacesModel struct {
	items   []config.WorkspaceConfig
	spinner spinner.Model
	loading bool
	width   int
	height  int
}

func newWorkspacesModel() workspacesModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorAccent)
	return workspacesModel{spinner: sp, loading: true}
}

func (m workspacesModel) load() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.LoadWorkspacesConfig()
		if err != nil || cfg == nil {
			return workspacesDataMsg(nil)
		}
		return workspacesDataMsg(cfg.Workspaces)
	}
}

func (m workspacesModel) Update(msg tea.Msg) (workspacesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case workspacesDataMsg:
		m.items = []config.WorkspaceConfig(msg)
		m.loading = false
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m workspacesModel) View() string {
	if m.loading {
		return "\n  " + m.spinner.View() + " Loading workspaces…"
	}
	if len(m.items) == 0 {
		return "\n  No workspaces configured. Add entries to workspaces.yaml."
	}

	cardW := m.width - 4
	if cardW < 20 {
		cardW = 20
	}
	// cardInnerWidth accounts for the card border+padding (2 chars each side = 4 total).
	cardInnerWidth := cardW - 4

	var sb strings.Builder
	sb.WriteString("\n")
	for _, ws := range m.items {
		// Platform badge.
		platform := ws.PMPlatform
		if platform == "" {
			platform = "none"
		}
		platBadge := StyleBadge(ColorInfo).Render(" " + platform + " ")

		// Status badge.
		var statusBadge string
		if ws.Enabled {
			statusBadge = StyleBadge(ColorSuccess).Render("● enabled")
		} else {
			statusBadge = StyleBadge(ColorDanger).Render("○ disabled")
		}

		// Header line: workspace name left + platform badge right.
		nameStr := lipgloss.NewStyle().Bold(true).Render(ws.Name)
		// Calculate spacer between name and badge.
		nameW := lipgloss.Width(nameStr)
		badgeW := lipgloss.Width(platBadge)
		spacer := cardInnerWidth - nameW - badgeW
		if spacer < 1 {
			spacer = 1
		}
		headerLine := nameStr + strings.Repeat(" ", spacer) + platBadge

		// Path line (truncated).
		path := ws.Path
		if len(path) > cardInnerWidth-2 {
			path = "…" + path[len(path)-(cardInnerWidth-3):]
		}
		pathLine := StyleMuted.Render(path)

		// Status line.
		statusLine := statusBadge

		cardContent := headerLine + "\n" + pathLine + "\n" + statusLine
		card := StyleCard.Width(cardW).Render(cardContent)
		sb.WriteString(card + "\n")
	}
	return sb.String()
}
