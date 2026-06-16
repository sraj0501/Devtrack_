package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sraj0501/Devtrack_/devtrack_client/internal/db"
)

type tuiTab int

const (
	tabOverview   tuiTab = 0
	tabActivity   tuiTab = 1
	tabWorkspaces tuiTab = 2
	tabAlerts     tuiTab = 3
	tabQueue      tuiTab = 4
)

var tuiTabNames = []string{"Overview", "Activity", "Workspaces", "Alerts", "Queue"}

type tuiTickMsg time.Time

// tuiFlashMsg is fired 150ms after a tab switch to clear the flash state.
type tuiFlashMsg struct{}

type tuiModel struct {
	activeTab      tuiTab
	db             *db.Database
	overview       overviewModel
	activity       activityModel
	workspaces     workspacesModel
	alerts         alertsModel
	queue          queueModel
	width          int
	height         int
	refreshSpinner spinner.Model
	refreshing     bool
	flash          bool
}

func newTUIModel(database *db.Database) tuiModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorAccent)
	return tuiModel{
		db:             database,
		overview:       newOverviewModel(database),
		activity:       newActivityModel(database),
		workspaces:     newWorkspacesModel(),
		alerts:         newAlertsModel(database),
		queue:          newQueueModel(database),
		refreshSpinner: sp,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(
		m.overview.load(),
		m.activity.load(),
		m.workspaces.load(),
		m.alerts.load(),
		m.queue.load(),
		m.overview.spinner.Tick,
		m.activity.spinner.Tick,
		m.workspaces.spinner.Tick,
		m.alerts.spinner.Tick,
		m.queue.spinner.Tick,
		m.refreshSpinner.Tick,
		tea.Tick(30*time.Second, func(t time.Time) tea.Msg { return tuiTickMsg(t) }),
	)
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentH := msg.Height - 4
		m.overview.width, m.overview.height = msg.Width, contentH
		m.activity.width, m.activity.height = msg.Width, contentH
		m.workspaces.width, m.workspaces.height = msg.Width, contentH
		m.alerts.width, m.alerts.height = msg.Width, contentH
		m.queue.width, m.queue.height = msg.Width, contentH
		// Forward window resize to tabs that manage a viewport.
		var aCmd, alCmd tea.Cmd
		m.activity, aCmd = m.activity.Update(msg)
		m.alerts, alCmd = m.alerts.Update(msg)
		if aCmd != nil {
			cmds = append(cmds, aCmd)
		}
		if alCmd != nil {
			cmds = append(cmds, alCmd)
		}

	case spinner.TickMsg:
		// Forward spinner tick to root refresh spinner and all tab spinners.
		var spCmd tea.Cmd
		m.refreshSpinner, spCmd = m.refreshSpinner.Update(msg)
		if spCmd != nil {
			cmds = append(cmds, spCmd)
		}
		var ovCmd, actCmd, wsCmd, alCmd, qCmd tea.Cmd
		m.overview, ovCmd = m.overview.Update(msg)
		m.activity, actCmd = m.activity.Update(msg)
		m.workspaces, wsCmd = m.workspaces.Update(msg)
		m.alerts, alCmd = m.alerts.Update(msg)
		m.queue, qCmd = m.queue.Update(msg)
		for _, c := range []tea.Cmd{ovCmd, actCmd, wsCmd, alCmd, qCmd} {
			if c != nil {
				cmds = append(cmds, c)
			}
		}

	case tuiFlashMsg:
		m.flash = false

	case tea.KeyMsg:
		// Queue tab gets its own key handling for cursor/approve/reject.
		if m.activeTab == tabQueue {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "tab":
				m.activeTab = (m.activeTab + 1) % tuiTab(len(tuiTabNames))
				m.flash = true
				cmds = append(cmds, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return tuiFlashMsg{} }))
			case "1":
				m.activeTab = tabOverview
				m.flash = true
				cmds = append(cmds, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return tuiFlashMsg{} }))
			case "2":
				m.activeTab = tabActivity
				m.flash = true
				cmds = append(cmds, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return tuiFlashMsg{} }))
			case "3":
				m.activeTab = tabWorkspaces
				m.flash = true
				cmds = append(cmds, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return tuiFlashMsg{} }))
			case "4":
				m.activeTab = tabAlerts
				m.flash = true
				cmds = append(cmds, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return tuiFlashMsg{} }))
			case "5":
				m.activeTab = tabQueue
				m.flash = true
				cmds = append(cmds, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return tuiFlashMsg{} }))
			default:
				var qCmd tea.Cmd
				m.queue, qCmd = m.queue.Update(msg)
				if qCmd != nil {
					cmds = append(cmds, qCmd)
				}
			}
		} else {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "tab":
				m.activeTab = (m.activeTab + 1) % tuiTab(len(tuiTabNames))
				m.flash = true
				cmds = append(cmds, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return tuiFlashMsg{} }))
			case "1":
				m.activeTab = tabOverview
				m.flash = true
				cmds = append(cmds, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return tuiFlashMsg{} }))
			case "2":
				m.activeTab = tabActivity
				m.flash = true
				cmds = append(cmds, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return tuiFlashMsg{} }))
			case "3":
				m.activeTab = tabWorkspaces
				m.flash = true
				cmds = append(cmds, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return tuiFlashMsg{} }))
			case "4":
				m.activeTab = tabAlerts
				m.flash = true
				cmds = append(cmds, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return tuiFlashMsg{} }))
			case "5":
				m.activeTab = tabQueue
				m.flash = true
				cmds = append(cmds, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg { return tuiFlashMsg{} }))
			case "r":
				m.refreshing = true
				cmds = append(cmds,
					m.overview.load(),
					m.activity.load(),
					m.workspaces.load(),
					m.alerts.load(),
					m.queue.load(),
					m.refreshSpinner.Tick,
				)
			}
		}

	case tuiTickMsg:
		cmds = append(cmds,
			m.overview.load(),
			tea.Tick(30*time.Second, func(t time.Time) tea.Msg { return tuiTickMsg(t) }),
		)
		// Fan tickMsg to queue for its auto-refresh and pulse countdown.
		var qCmd tea.Cmd
		m.queue, qCmd = m.queue.Update(msg)
		if qCmd != nil {
			cmds = append(cmds, qCmd)
		}

	case overviewDataMsg:
		m.overview, _ = m.overview.Update(msg)
		m.refreshing = false
	case activityDataMsg:
		m.activity, _ = m.activity.Update(msg)
	case workspacesDataMsg:
		m.workspaces, _ = m.workspaces.Update(msg)
	case alertsDataMsg:
		m.alerts, _ = m.alerts.Update(msg)
	case queueDataMsg:
		m.queue, _ = m.queue.Update(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m tuiModel) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	header := m.renderHeader()
	tabBar := m.renderTabBar()
	sep := lipgloss.NewStyle().Foreground(ColorSubtle).Render(strings.Repeat("─", m.width))

	var body string
	switch m.activeTab {
	case tabOverview:
		body = m.overview.View()
	case tabActivity:
		body = m.activity.View()
	case tabWorkspaces:
		body = m.workspaces.View()
	case tabAlerts:
		body = m.alerts.View()
	case tabQueue:
		body = m.queue.View()
	}

	var footer string
	if m.refreshing {
		footer = StyleMuted.Render("  " + m.refreshSpinner.View() + " Refreshing…   Tab/1-5: switch   q: quit")
	} else {
		footer = StyleMuted.Render("  Tab/1-5: switch   r: refresh   q: quit")
	}

	return header + "\n" + tabBar + "\n" + sep + "\n" + body + "\n" + footer
}

// renderHeader renders the top bar with branding and mode/version right-aligned.
func (m tuiModel) renderHeader() string {
	brand := StyleHeader.Render("  ◆ DevTrack")
	right := StyleMuted.Render("managed  v3.0.10  ")
	spacer := m.width - lipgloss.Width(brand) - lipgloss.Width(right)
	if spacer < 0 {
		spacer = 0
	}
	return brand + strings.Repeat(" ", spacer) + right
}

// renderTabBar renders the pill-style tab row.
func (m tuiModel) renderTabBar() string {
	var sb strings.Builder
	for i, name := range tuiTabNames {
		sb.WriteString(m.renderTabLabel(tuiTab(i), name))
	}
	return lipgloss.NewStyle().Width(m.width).Render(sb.String())
}

// renderTabLabel renders a single pill-style tab button.
// Active tab uses Accent background; during flash it uses ColorFlash background.
func (m tuiModel) renderTabLabel(t tuiTab, name string) string {
	isActive := m.activeTab == t
	label := fmt.Sprintf("%d %s", int(t)+1, name)
	if isActive {
		var bg lipgloss.TerminalColor
		if m.flash {
			bg = ColorFlash
		} else {
			bg = ColorAccent
		}
		return lipgloss.NewStyle().
			Background(bg).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 2).
			Render(label)
	}
	return StyleMuted.Padding(0, 2).Render(label)
}

// RunTUI opens the Bubble Tea TUI dashboard.
func RunTUI() error {
	database, err := db.NewDatabase()
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}
	defer database.Close()

	m := newTUIModel(database)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
