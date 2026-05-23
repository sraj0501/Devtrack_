package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type workspacesDataMsg []WorkspaceConfig

type workspacesModel struct {
	items  []WorkspaceConfig
	width  int
	height int
}

func newWorkspacesModel() workspacesModel { return workspacesModel{} }

func (m workspacesModel) load() tea.Cmd {
	return func() tea.Msg {
		cfg, err := LoadWorkspacesConfig()
		if err != nil || cfg == nil {
			return workspacesDataMsg(nil)
		}
		return workspacesDataMsg(cfg.Workspaces)
	}
}

func (m workspacesModel) Update(msg tea.Msg) (workspacesModel, tea.Cmd) {
	if d, ok := msg.(workspacesDataMsg); ok {
		m.items = []WorkspaceConfig(d)
	}
	return m, nil
}

func (m workspacesModel) View() string {
	if len(m.items) == 0 {
		return "\n  No workspaces configured. Add entries to workspaces.yaml."
	}

	green := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  %-24s %-12s %-12s %s\n",
		bold.Render("Name"),
		bold.Render("Platform"),
		bold.Render("Status"),
		bold.Render("Path"),
	))
	sb.WriteString("  " + strings.Repeat("─", 72) + "\n")

	for _, ws := range m.items {
		status := green.Render("● enabled ")
		if !ws.Enabled {
			status = red.Render("○ disabled")
		}
		platform := ws.PMPlatform
		if platform == "" {
			platform = muted.Render("none")
		}
		path := ws.Path
		if len(path) > 34 {
			path = "…" + path[len(path)-33:]
		}
		sb.WriteString(fmt.Sprintf("  %-24s %-12s %-12s %s\n",
			ws.Name, platform, status, muted.Render(path)))
	}
	return sb.String()
}
