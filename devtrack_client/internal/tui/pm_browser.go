package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// BrowseItem is one row in the PM ticket browser.
type BrowseItem struct {
	Title    string // issue/work-item title
	Subtitle string // e.g. "[github] #42 · open · owner/repo"
	Body     string // detail shown in right pane
	URL      string // opened on Enter/o
}

// BrowseTickets opens an interactive split-pane browser over items.
// The user can navigate, filter, and open tickets in their browser.
// Returns immediately with a plain-text fallback when stdin is not a TTY.
func BrowseTickets(items []BrowseItem) error {
	if len(items) == 0 {
		fmt.Println("No open tickets assigned to you.")
		return nil
	}
	if !pickerIsTTY() {
		for _, it := range items {
			fmt.Printf("%s  %s\n  %s\n\n", it.Subtitle, it.Title, it.URL)
		}
		return nil
	}
	p := tea.NewProgram(newBrowserModel(items), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// ── bubbletea model ───────────────────────────────────────────────────────────

type browserModel struct {
	items     []BrowseItem
	visible   []int
	cursor    int
	filtering bool
	filter    string
	statusMsg string // transient feedback line
	width     int
	height    int
}

func newBrowserModel(items []BrowseItem) browserModel {
	m := browserModel{items: items, width: 80, height: 24}
	m.visible = make([]int, 0, len(items))
	m.recompute()
	return m
}

func (m *browserModel) recompute() {
	m.visible = m.visible[:0]
	f := strings.ToLower(strings.TrimSpace(m.filter))
	for i, it := range m.items {
		if f == "" || strings.Contains(strings.ToLower(it.Title+" "+it.Subtitle), f) {
			m.visible = append(m.visible, i)
		}
	}
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

type browserClearStatusMsg struct{}

func (m browserModel) Init() tea.Cmd { return nil }

func (m browserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case browserClearStatusMsg:
		m.statusMsg = ""

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.filtering {
			switch msg.Type {
			case tea.KeyEnter:
				m.filtering = false
			case tea.KeyEsc:
				m.filtering = false
				m.filter = ""
				m.recompute()
			case tea.KeyBackspace:
				if m.filter != "" {
					m.filter = m.filter[:len(m.filter)-1]
					m.recompute()
				}
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyRunes, tea.KeySpace:
				m.filter += string(msg.Runes)
				m.recompute()
			}
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.visible)-1 {
				m.cursor++
			}
		case "/":
			m.filtering = true
		case "enter", "o":
			if len(m.visible) > 0 {
				url := m.items[m.visible[m.cursor]].URL
				if url != "" {
					openBrowserURL(url)
					m.statusMsg = "Opened: " + url
					return m, tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
						return browserClearStatusMsg{}
					})
				}
				m.statusMsg = "No URL for this ticket"
				return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return browserClearStatusMsg{}
				})
			}
		case "esc", "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m browserModel) View() string {
	width := m.width
	if width < 40 {
		width = 80
	}
	height := m.height
	if height < 10 {
		height = 24
	}
	bodyH := height - 4

	listW := width * 2 / 5
	if listW < 24 {
		listW = 24
	}
	bodyW := width - listW - 3
	if bodyW < 20 {
		bodyW = 20
	}

	selStyle := lipgloss.NewStyle().Reverse(true)
	dimStyle := lipgloss.NewStyle().Faint(true)

	// Left pane: scrollable list
	var rows []string
	start := 0
	if m.cursor >= bodyH {
		start = m.cursor - bodyH + 1
	}
	for vi := start; vi < len(m.visible) && vi < start+bodyH; vi++ {
		it := m.items[m.visible[vi]]
		line := truncatePicker(it.Subtitle+"  "+it.Title, listW-2)
		if vi == m.cursor {
			rows = append(rows, selStyle.Render("> "+line))
		} else {
			rows = append(rows, "  "+line)
		}
	}
	if len(m.visible) == 0 {
		rows = append(rows, dimStyle.Render("  (no matches)"))
	}
	listPane := lipgloss.NewStyle().Width(listW).Height(bodyH).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		Render(strings.Join(rows, "\n"))

	// Right pane: selected ticket detail
	detail := ""
	if len(m.visible) > 0 {
		it := m.items[m.visible[m.cursor]]
		detail = lipgloss.NewStyle().Bold(true).Render(it.Title) + "\n" +
			dimStyle.Render(it.Subtitle)
		if it.Body != "" {
			body := it.Body
			if len(body) > 600 {
				body = body[:600] + "…"
			}
			detail += "\n\n" + body
		}
		if it.URL != "" {
			detail += "\n\n" + dimStyle.Render(it.URL)
		}
	}
	bodyPane := lipgloss.NewStyle().Width(bodyW).Height(bodyH).Padding(0, 1).Render(detail)

	header := lipgloss.NewStyle().Bold(true).
		Render(fmt.Sprintf("Assigned Tickets (%d shown)", len(m.visible)))

	var helpLine string
	switch {
	case m.filtering:
		helpLine = dimStyle.Render(fmt.Sprintf("filter: %s_  (enter to apply · esc to clear)", m.filter))
	case m.statusMsg != "":
		helpLine = lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Render(m.statusMsg)
	default:
		helpLine = dimStyle.Render("↑/↓  navigate  ·  /  filter  ·  enter/o  open in browser  ·  q  quit")
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, listPane, bodyPane)
	return header + "\n" + body + "\n" + helpLine
}

// openBrowserURL launches the default browser for url (best-effort, no error check).
func openBrowserURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
