package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type activityDataMsg []TriggerRecord

type activityModel struct {
	db     *Database
	items  []TriggerRecord
	width  int
	height int
}

func newActivityModel(db *Database) activityModel { return activityModel{db: db} }

func (m activityModel) load() tea.Cmd {
	return func() tea.Msg {
		if m.db == nil {
			return activityDataMsg(nil)
		}
		records, _ := m.db.GetRecentTriggers(30)
		return activityDataMsg(records)
	}
}

func (m activityModel) Update(msg tea.Msg) (activityModel, tea.Cmd) {
	if d, ok := msg.(activityDataMsg); ok {
		m.items = []TriggerRecord(d)
	}
	return m, nil
}

func (m activityModel) View() string {
	if len(m.items) == 0 {
		return "\n  No recent activity logged yet."
	}

	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	commitStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	timerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))

	var sb strings.Builder
	sb.WriteString("\n")
	for _, r := range m.items {
		icon := commitStyle.Render("⬡ commit")
		if r.TriggerType == "timer" {
			icon = timerStyle.Render("⏱ timer ")
		}
		ts := r.Timestamp.Format("01/02 15:04")
		msg := r.CommitMessage
		if len(msg) > 60 {
			msg = msg[:57] + "…"
		}
		if r.TriggerType == "timer" || msg == "" {
			msg = fmt.Sprintf("trigger from %s", r.Source)
		}
		hash := ""
		if r.CommitHash != "" {
			h := r.CommitHash
			if len(h) > 7 {
				h = h[:7]
			}
			hash = muted.Render(" " + h)
		}
		sb.WriteString(fmt.Sprintf("  %s  %s  %s%s\n",
			muted.Render(ts), icon, msg, hash))
	}
	return sb.String()
}
