package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

type alertsDataMsg []db.NotificationRecord

type alertsModel struct {
	db      *db.Database
	vp      viewport.Model
	items   []db.NotificationRecord
	spinner spinner.Model
	loading bool
	width   int
	height  int
}

func newAlertsModel(database *db.Database) alertsModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorAccent)
	return alertsModel{db: database, spinner: sp, loading: true}
}

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
	switch msg := msg.(type) {
	case alertsDataMsg:
		m.items = []db.NotificationRecord(msg)
		m.loading = false
		m.vp.SetContent(m.buildContent())
		return m, nil

	case tea.WindowSizeMsg:
		contentH := msg.Height - 5
		if contentH < 1 {
			contentH = 1
		}
		m.vp = viewport.New(msg.Width, contentH)
		m.vp.SetContent(m.buildContent())
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	default:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
}

func (m alertsModel) sourceBadge(source string) string {
	switch source {
	case "github":
		return StyleBadge(ColorAccent).Render(" " + source + " ")
	case "azure":
		return StyleBadge(ColorInfo).Render(" " + source + " ")
	case "jira":
		return StyleBadge(ColorWarning).Render(" " + source + " ")
	default:
		return StyleBadge(ColorMuted).Render(" " + source + " ")
	}
}

func (m alertsModel) buildContent() string {
	if len(m.items) == 0 {
		return "\n  No notifications yet.\n  The ticket alerter will populate this as alerts arrive."
	}

	var sb strings.Builder
	sb.WriteString("\n")
	for _, r := range m.items {
		// Unread/read dot.
		var dot string
		var titleStyle lipgloss.Style
		if !r.Read {
			dot = lipgloss.NewStyle().Foreground(ColorSuccess).Render("●")
			titleStyle = lipgloss.NewStyle().Bold(true)
		} else {
			dot = StyleMuted.Render("○")
			titleStyle = StyleMuted
		}

		ts := StyleMuted.Render(r.CreatedAt.Format("01/02 15:04"))
		badge := m.sourceBadge(r.Source)
		eventType := StyleMuted.Render(fmt.Sprintf("%-14s", r.EventType))

		title := r.Title
		if len(title) > 55 {
			title = title[:52] + "…"
		}

		sb.WriteString(fmt.Sprintf("  %s  %s  %s  %s  %s\n",
			dot, ts, badge, eventType, titleStyle.Render(title)))
	}
	return sb.String()
}

func (m alertsModel) View() string {
	if m.loading {
		return "\n  " + m.spinner.View() + " Loading alerts…"
	}
	return m.vp.View()
}
