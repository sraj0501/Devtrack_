package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type alertsDataMsg []NotificationRecord

type alertsModel struct {
	db     *Database
	items  []NotificationRecord
	width  int
	height int
}

func newAlertsModel(db *Database) alertsModel { return alertsModel{db: db} }

func (m alertsModel) load() tea.Cmd {
	return func() tea.Msg {
		if m.db == nil {
			return alertsDataMsg(nil)
		}
		records, _ := m.db.GetAllNotifications(30)
		return alertsDataMsg(records)
	}
}

func (m alertsModel) Update(msg tea.Msg) (alertsModel, tea.Cmd) {
	if d, ok := msg.(alertsDataMsg); ok {
		m.items = []NotificationRecord(d)
	}
	return m, nil
}

func (m alertsModel) View() string {
	if len(m.items) == 0 {
		return "\n  No notifications yet.\n  The ticket alerter will populate this as alerts arrive."
	}

	githubStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	azureStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	jiraStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	unread := lipgloss.NewStyle().Bold(true)

	var sb strings.Builder
	sb.WriteString("\n")
	for _, r := range m.items {
		srcStyle := muted
		switch r.Source {
		case "github":
			srcStyle = githubStyle
		case "azure":
			srcStyle = azureStyle
		case "jira":
			srcStyle = jiraStyle
		}

		dot := muted.Render("○")
		titleStyle := muted
		if !r.Read {
			dot = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("●")
			titleStyle = unread
		}

		ts := r.CreatedAt.Format("01/02 15:04")
		title := r.Title
		if len(title) > 55 {
			title = title[:52] + "…"
		}

		sb.WriteString(fmt.Sprintf("  %s %s  %-8s %-18s %s\n",
			dot,
			muted.Render(ts),
			srcStyle.Render(r.Source),
			muted.Render(r.EventType),
			titleStyle.Render(title),
		))
	}
	return sb.String()
}
