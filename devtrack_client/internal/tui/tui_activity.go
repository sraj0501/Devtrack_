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

type activityDataMsg []db.TriggerRecord

type activityModel struct {
	db      *db.Database
	vp      viewport.Model
	items   []db.TriggerRecord
	spinner spinner.Model
	loading bool
	width   int
	height  int
}

func newActivityModel(database *db.Database) activityModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorAccent)
	return activityModel{db: database, spinner: sp, loading: true}
}

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
	switch msg := msg.(type) {
	case activityDataMsg:
		m.items = []db.TriggerRecord(msg)
		m.loading = false
		// Build viewport content.
		content := m.buildContent()
		m.vp.SetContent(content)
		return m, nil

	case tea.WindowSizeMsg:
		contentH := msg.Height - 5 // leave room for header, tabs, sep, footer
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
		// Forward other messages (scroll keys) to the viewport.
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
}

func (m activityModel) buildContent() string {
	if len(m.items) == 0 {
		return "\n  No recent activity logged yet."
	}

	commitBadge := StyleBadge(ColorInfo).Render(" commit ")
	timerBadge := StyleBadge(ColorWarning).Render(" timer  ")

	var sb strings.Builder
	sb.WriteString("\n")
	for _, r := range m.items {
		badge := commitBadge
		if r.TriggerType == "timer" {
			badge = timerBadge
		}
		ts := StyleMuted.Render(r.Timestamp.Format("01/02 15:04"))
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
			hash = StyleMuted.Render(" " + h)
		}
		sb.WriteString(fmt.Sprintf("  %s  %s  %s%s\n", ts, badge, msg, hash))
	}
	return sb.String()
}

func (m activityModel) View() string {
	if m.loading {
		return "\n  " + m.spinner.View() + " Loading activity…"
	}
	return m.vp.View()
}
