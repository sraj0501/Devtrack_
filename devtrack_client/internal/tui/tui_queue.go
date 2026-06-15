package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

// queueDataMsg carries the latest snapshot from the DB.
type queueDataMsg []db.PendingAction

// queueModel holds the state for the Queue tab.
type queueModel struct {
	db       *db.Database
	items    []db.PendingAction
	cursor   int
	width    int
	height   int
	lastLoad time.Time
}

func newQueueModel(database *db.Database) queueModel {
	return queueModel{db: database}
}

// load fetches the last 24 hours of pending actions from the DB in a goroutine.
func (m queueModel) load() tea.Cmd {
	return func() tea.Msg {
		if m.db == nil {
			return queueDataMsg(nil)
		}
		items, _ := m.db.ListPendingActionsRecent(24)
		return queueDataMsg(items)
	}
}

// Update handles messages for the Queue tab.
func (m queueModel) Update(msg tea.Msg) (queueModel, tea.Cmd) {
	switch msg := msg.(type) {
	case queueDataMsg:
		m.items = []db.PendingAction(msg)
		m.lastLoad = time.Now()
		// Clamp cursor to valid range after reload.
		if m.cursor >= len(m.items) && len(m.items) > 0 {
			m.cursor = len(m.items) - 1
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "a":
			if len(m.items) > 0 && m.cursor < len(m.items) {
				item := m.items[m.cursor]
				if item.Status == "pending" {
					_ = m.db.UpdatePendingActionStatus(item.ID, "approved", "tui")
					return m, m.load()
				}
			}
		case "r":
			if len(m.items) > 0 && m.cursor < len(m.items) {
				item := m.items[m.cursor]
				if item.Status == "pending" {
					_ = m.db.UpdatePendingActionStatus(item.ID, "rejected", "tui")
					return m, m.load()
				}
			}
		case "e":
			// Edit not implemented yet — placeholder for Phase 1 completion.
			return m, nil
		}

	case tuiTickMsg:
		return m, m.load()
	}

	return m, nil
}

// confidence_bar renders a 5-character block bar for a confidence score in [0.0, 1.0].
// Example: 0.80 → "████░"
func confidenceBar(c float64) string {
	if c < 0 {
		c = 0
	}
	if c > 1 {
		c = 1
	}
	filled := int(c*5 + 0.5)
	return strings.Repeat("█", filled) + strings.Repeat("░", 5-filled)
}

// expiresCountdown returns a human-readable countdown for the ExpiresAt field.
func expiresCountdown(t time.Time) string {
	remaining := time.Until(t)
	if remaining <= 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("expired")
	}
	remaining = remaining.Round(time.Second)
	if remaining >= time.Minute {
		return fmt.Sprintf("in %dm", int(remaining.Minutes()))
	}
	return fmt.Sprintf("in %ds", int(remaining.Seconds()))
}

// View renders the Queue tab.
func (m queueModel) View() string {
	if len(m.items) == 0 {
		footer := queueFooter(m.lastLoad)
		return "\n  No pending actions in the last 24 hours.\n\n" + footer
	}

	// Status badge styles.
	pendingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)  // yellow
	approvedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)  // green
	rejectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))            // red
	postedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))              // dim
	failedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)   // bright red
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle := lipgloss.NewStyle().Reverse(true)

	var sb strings.Builder
	sb.WriteString("\n")

	header := fmt.Sprintf("  %-7s  %-5s  %-10s  %-18s  %-20s\n",
		"STATUS", "CONF", "EXPIRES", "TYPE", "TARGET")
	sb.WriteString(muted.Render(header))

	for i, item := range m.items {
		// Status badge.
		var badge string
		switch item.Status {
		case "pending":
			badge = pendingStyle.Render(fmt.Sprintf("%-8s", "pending"))
		case "approved":
			badge = approvedStyle.Render(fmt.Sprintf("%-8s", "approved"))
		case "rejected":
			badge = rejectedStyle.Render(fmt.Sprintf("%-8s", "rejected"))
		case "posted":
			badge = postedStyle.Render(fmt.Sprintf("%-8s", "posted"))
		case "failed":
			badge = failedStyle.Render(fmt.Sprintf("%-8s", "failed"))
		default:
			badge = fmt.Sprintf("%-8s", item.Status)
		}

		// Confidence bar.
		bar := confidenceBar(item.Confidence)

		// Expiry countdown.
		expiry := expiresCountdown(item.ExpiresAt)

		// Truncate long strings.
		actionType := item.ActionType
		if len(actionType) > 18 {
			actionType = actionType[:15] + "..."
		}
		target := item.Target
		if len(target) > 20 {
			target = target[:17] + "..."
		}

		row := fmt.Sprintf("  %s  %s  %-10s  %-18s  %-20s",
			badge, bar, expiry, actionType, target)

		if i == m.cursor {
			sb.WriteString(cursorStyle.Render(row) + "\n")
		} else {
			sb.WriteString(row + "\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(queueFooter(m.lastLoad))
	return sb.String()
}

func queueFooter(lastLoad time.Time) string {
	ts := "--:--:--"
	if !lastLoad.IsZero() {
		ts = lastLoad.Format("15:04:05")
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(fmt.Sprintf("  [a]pprove  [r]eject  [e]dit  (auto-refresh 10s)  last: %s", ts))
}
