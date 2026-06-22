package tui

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
	"github.com/sraj0501/Devtrack_/devtrack_client/internal/trigger"
)

// queueDataMsg carries the latest snapshot from the DB.
type queueDataMsg []db.PendingAction

// queueModel holds the state for the Queue tab.
type queueModel struct {
	db            *db.Database
	triggerClient *trigger.HTTPTriggerClient
	items         []db.PendingAction
	cursor        int
	width         int
	height        int
	lastLoad      time.Time
	spinner       spinner.Model
	loading       bool
	pulseState    bool

	// Flagging mode — active when flaggingActionID != 0.
	flaggingActionID int64
	flagInput        textinput.Model
	flagErrMsg       string
}

func newQueueModel(database *db.Database, tc *trigger.HTTPTriggerClient) queueModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorAccent)
	return queueModel{db: database, triggerClient: tc, spinner: sp, loading: true}
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

// submitFlag inserts a correction for the most recent inference associated with
// the currently flagged action's type, halves the inference confidence, and
// resets flagging mode.
// Returns (updated model, cmd-to-reload) — the caller must return these from Update.
func (m queueModel) submitFlag() (queueModel, tea.Cmd) {
	// Find the action being flagged.
	var action *db.PendingAction
	for i := range m.items {
		if m.items[i].ID == m.flaggingActionID {
			action = &m.items[i]
			break
		}
	}
	if action == nil {
		m.flagErrMsg = "Action not found — it may have expired."
		return m, nil
	}

	// Look up the most recent inference for this action's type.
	inferences, err := m.db.ListInferences(action.ActionType, 1)
	if err != nil || len(inferences) == 0 {
		m.flagErrMsg = "No inference recorded for this action."
		return m, nil
	}
	inf := inferences[0]

	// Insert the correction.
	corrText := m.flagInput.Value()
	if strings.TrimSpace(corrText) == "" {
		m.flagErrMsg = "Correction text cannot be empty."
		return m, nil
	}

	_, insertErr := m.db.InsertCorrection(db.Correction{
		InferenceID: inf.ID,
		Correction:  corrText,
		FlaggedFrom: "tui",
		Weight:      2.0,
	})
	if insertErr != nil {
		log.Printf("tui: InsertCorrection failed for inference %d: %v", inf.ID, insertErr)
		m.flagErrMsg = fmt.Sprintf("Failed to save correction: %v", insertErr)
		return m, nil
	}

	// Halve the flagged inference's confidence immediately.
	newConf := inf.Confidence * 0.5
	if confErr := m.db.UpdateInferenceConfidence(inf.ID, newConf); confErr != nil {
		log.Printf("tui: UpdateInferenceConfidence failed for inference %d: %v", inf.ID, confErr)
	}

	// Reset flagging mode.
	m.flaggingActionID = 0
	m.flagInput.Reset()
	m.flagErrMsg = ""

	return m, m.load()
}

// Update handles messages for the Queue tab.
func (m queueModel) Update(msg tea.Msg) (queueModel, tea.Cmd) {
	// When in flagging mode, route all key events to the text input first.
	if m.flaggingActionID != 0 {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				return m.submitFlag()
			case "esc":
				// Cancel with no DB changes.
				m.flaggingActionID = 0
				m.flagInput.Reset()
				m.flagErrMsg = ""
				return m, nil
			default:
				var cmd tea.Cmd
				m.flagInput, cmd = m.flagInput.Update(msg)
				return m, cmd
			}
		}
		// Non-key messages still fall through to normal handling below.
	}

	switch msg := msg.(type) {
	case queueDataMsg:
		m.items = []db.PendingAction(msg)
		m.lastLoad = time.Now()
		m.loading = false
		// Clamp cursor to valid range after reload.
		if m.cursor >= len(m.items) && len(m.items) > 0 {
			m.cursor = len(m.items) - 1
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tuiTickMsg:
		// Toggle pulse state on every tick for urgency animation.
		m.pulseState = !m.pulseState
		return m, m.load()

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
					// Adaptive threshold signal — record approval for per-type threshold learning.
					if logErr := m.db.RecordApproval(item.ActionType, item.Workspace); logErr != nil {
						log.Printf("[threshold] RecordApproval: %v", logErr)
					}
					// Fire-and-forget: call /dialectic/infer for the approval interaction.
					// Non-blocking; errors logged only — never interrupts the TUI.
					if m.triggerClient != nil && m.db != nil {
						go func(action db.PendingAction, tc *trigger.HTTPTriggerClient, database *db.Database) {
							inferences, err := tc.PostDialecticInferApproval(action)
							if err != nil {
								log.Printf("dialectic: TUI approval infer failed for action %d: %v", action.ID, err)
								return
							}
							for _, inf := range inferences {
								_, storeErr := database.InsertInference(db.Inference{
									ContextType: inf.ContextType,
									Subject:     inf.Subject,
									Inference:   inf.InferenceText,
									Evidence:    fmt.Sprintf(`[%d]`, action.ID),
									Confidence:  inf.Confidence,
									Source:      "hermes3",
								})
								if storeErr != nil {
									log.Printf("dialectic: store approval inference failed: %v", storeErr)
								}
							}
						}(item, m.triggerClient, m.db)
					}
					return m, m.load()
				}
			}
		case "r":
			if len(m.items) > 0 && m.cursor < len(m.items) {
				item := m.items[m.cursor]
				if item.Status == "pending" {
					_ = m.db.UpdatePendingActionStatus(item.ID, "rejected", "tui")
					// Adaptive threshold signal — record rejection for per-type threshold learning.
					if logErr := m.db.RecordRejection(item.ActionType, item.Workspace); logErr != nil {
						log.Printf("[threshold] RecordRejection: %v", logErr)
					}
					// Fire-and-forget: call /dialectic/infer for the rejection interaction.
					if m.triggerClient != nil && m.db != nil {
						go func(action db.PendingAction, tc *trigger.HTTPTriggerClient, database *db.Database) {
							inferences, err := tc.PostDialecticInferRejection(action)
							if err != nil {
								log.Printf("dialectic: TUI rejection infer failed for action %d: %v", action.ID, err)
								return
							}
							for _, inf := range inferences {
								_, storeErr := database.InsertInference(db.Inference{
									ContextType: inf.ContextType,
									Subject:     inf.Subject,
									Inference:   inf.InferenceText,
									Evidence:    fmt.Sprintf(`[%d]`, action.ID),
									Confidence:  inf.Confidence,
									Source:      "hermes3",
								})
								if storeErr != nil {
									log.Printf("dialectic: store rejection inference failed: %v", storeErr)
								}
							}
						}(item, m.triggerClient, m.db)
					}
					return m, m.load()
				}
			}
		case "e":
			// Edit not implemented yet — placeholder for Phase 1 completion.
			return m, nil
		case "f":
			// Enter flagging mode for the currently selected pending action.
			if len(m.items) > 0 && m.cursor < len(m.items) {
				item := m.items[m.cursor]
				m.flaggingActionID = item.ID
				ti := textinput.New()
				ti.Placeholder = "Describe what was wrong…"
				ti.CharLimit = 200
				ti.Width = 60
				_ = ti.Focus()
				m.flagInput = ti
				m.flagErrMsg = ""
				return m, textinput.Blink
			}
		}
	}

	return m, nil
}

// queueStatusBadge returns a background-colored status badge.
func queueStatusBadge(status string) string {
	switch status {
	case "pending":
		return StyleBadge(ColorWarning).Render(" pending ")
	case "approved":
		return StyleBadge(ColorSuccess).Render(" approved")
	case "rejected":
		return StyleBadge(ColorDanger).Render(" rejected")
	case "posted":
		return StyleBadge(ColorMuted).Render(" posted  ")
	case "failed":
		return StyleBadge(ColorDanger).Bold(true).Render(" FAILED  ")
	default:
		return StyleBadge(ColorMuted).Render(" " + status + " ")
	}
}

// confidenceBar renders a 5-character block bar colored by threshold.
func confidenceBar(c float64) string {
	if c < 0 {
		c = 0
	}
	if c > 1 {
		c = 1
	}
	var barColor lipgloss.TerminalColor
	switch {
	case c > 0.90:
		barColor = ColorSuccess
	case c >= 0.70:
		barColor = ColorWarning
	default:
		barColor = ColorDanger
	}
	filled := int(c*5 + 0.5)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 5-filled)
	return lipgloss.NewStyle().Foreground(barColor).Render(bar)
}

// expiresCountdown returns a human-readable countdown, with pulsing color when < 30s.
func expiresCountdown(t time.Time, pulse bool) string {
	remaining := time.Until(t)
	if remaining <= 0 {
		return lipgloss.NewStyle().Foreground(ColorDanger).Render("expired")
	}
	remaining = remaining.Round(time.Second)
	var text string
	if remaining >= time.Minute {
		text = fmt.Sprintf("in %dm", int(remaining.Minutes()))
	} else {
		text = fmt.Sprintf("in %ds", int(remaining.Seconds()))
	}
	if remaining < 30*time.Second {
		color := lipgloss.TerminalColor(ColorDanger)
		if pulse {
			color = ColorWarning
		}
		return lipgloss.NewStyle().Foreground(color).Render(text)
	}
	return lipgloss.NewStyle().Foreground(ColorWarning).Render(text)
}

// queueFooter renders the key-hint bar with Accent-colored brackets.
func queueFooter(lastLoad time.Time) string {
	bracket := lipgloss.NewStyle().Foreground(ColorAccent)
	key := func(k, label string) string {
		return bracket.Render("[") + k + bracket.Render("]") + label
	}
	ts := "--:--:--"
	if !lastLoad.IsZero() {
		ts = lastLoad.Format("15:04:05")
	}
	return StyleMuted.Render(fmt.Sprintf("  %s  %s  %s  %s   last: %s",
		key("a", "pprove"), key("r", "eject"), key("e", "dit"),
		key("f", "lag-wrong-inference"), ts))
}

// flagOverlay renders the inline text-input box used when flagging an inference.
func (m queueModel) flagOverlay() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorAccent).
		Padding(0, 1).
		Width(68)

	title := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render("Flag inference as wrong")
	inputLine := "Correction: " + m.flagInput.View()
	hint := StyleMuted.Render("(Enter to submit · Esc to cancel)")

	content := title + "\n" + inputLine + "\n" + hint
	box := boxStyle.Render(content)

	if m.flagErrMsg != "" {
		errLine := lipgloss.NewStyle().Foreground(ColorDanger).Render("  " + m.flagErrMsg)
		return box + "\n" + errLine
	}
	return box
}

// View renders the Queue tab.
func (m queueModel) View() string {
	if m.loading {
		return "\n  " + m.spinner.View() + " Loading queue…"
	}

	if len(m.items) == 0 {
		footer := queueFooter(m.lastLoad)
		empty := "\n  No pending actions in the last 24 hours.\n\n"
		if m.flaggingActionID != 0 {
			return empty + "\n" + m.flagOverlay() + "\n"
		}
		return empty + footer
	}

	muted := StyleMuted

	var sb strings.Builder
	sb.WriteString("\n")

	header := fmt.Sprintf("  %-9s  %-5s  %-10s  %-18s  %-20s\n",
		"STATUS", "CONF", "EXPIRES", "TYPE", "TARGET")
	sb.WriteString(muted.Render(header))

	for i, item := range m.items {
		badge := queueStatusBadge(item.Status)
		bar := confidenceBar(item.Confidence)
		expiry := expiresCountdown(item.ExpiresAt, m.pulseState)

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
			row = lipgloss.NewStyle().
				Background(ColorAccent).
				Foreground(lipgloss.Color("#FFFFFF")).
				Render(row)
		}
		sb.WriteString(row + "\n")
	}

	sb.WriteString("\n")

	// When in flagging mode, replace the footer with the overlay.
	if m.flaggingActionID != 0 {
		sb.WriteString(m.flagOverlay())
	} else {
		sb.WriteString(queueFooter(m.lastLoad))
	}

	return sb.String()
}
