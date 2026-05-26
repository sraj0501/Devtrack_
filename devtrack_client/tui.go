package main

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tuiTab int

const (
	tabOverview   tuiTab = 0
	tabActivity   tuiTab = 1
	tabWorkspaces tuiTab = 2
	tabAlerts     tuiTab = 3
)

var tuiTabNames = []string{"Overview", "Activity", "Workspaces", "Alerts"}

type tuiTickMsg time.Time

type tuiModel struct {
	activeTab  tuiTab
	db         *Database
	overview   overviewModel
	activity   activityModel
	workspaces workspacesModel
	alerts     alertsModel
	width      int
	height     int
}

func newTUIModel(db *Database) tuiModel {
	return tuiModel{
		db:         db,
		overview:   newOverviewModel(db),
		activity:   newActivityModel(db),
		workspaces: newWorkspacesModel(),
		alerts:     newAlertsModel(db),
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(
		m.overview.load(),
		m.activity.load(),
		m.workspaces.load(),
		m.alerts.load(),
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

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % tuiTab(len(tuiTabNames))
		case "1":
			m.activeTab = tabOverview
		case "2":
			m.activeTab = tabActivity
		case "3":
			m.activeTab = tabWorkspaces
		case "4":
			m.activeTab = tabAlerts
		case "r":
			cmds = append(cmds,
				m.overview.load(),
				m.activity.load(),
				m.workspaces.load(),
				m.alerts.load(),
			)
		}

	case tuiTickMsg:
		cmds = append(cmds,
			m.overview.load(),
			tea.Tick(30*time.Second, func(t time.Time) tea.Msg { return tuiTickMsg(t) }),
		)

	case overviewDataMsg:
		m.overview, _ = m.overview.Update(msg)
	case activityDataMsg:
		m.activity, _ = m.activity.Update(msg)
	case workspacesDataMsg:
		m.workspaces, _ = m.workspaces.Update(msg)
	case alertsDataMsg:
		m.alerts, _ = m.alerts.Update(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m tuiModel) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	header := renderTUITabBar(m.activeTab, m.width)

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
	}

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("  Tab / 1-4: switch tab   r: refresh   q: quit")

	return header + "\n" + body + "\n" + footer
}

func renderTUITabBar(active tuiTab, width int) string {
	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("63")).
		Padding(0, 2)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(0, 2)

	bar := ""
	for i, name := range tuiTabNames {
		if tuiTab(i) == active {
			bar += activeStyle.Render(fmt.Sprintf("%d %s", i+1, name))
		} else {
			bar += inactiveStyle.Render(fmt.Sprintf("%d %s", i+1, name))
		}
	}
	return lipgloss.NewStyle().Width(width).Render(bar)
}

// RunTUI opens the Bubble Tea TUI dashboard.
func RunTUI() error {
	db, err := NewDatabase()
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}
	defer db.Close()

	m := newTUIModel(db)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
