package tui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PickItem is one selectable row in the ticket picker. It is intentionally
// decoupled from any PM type so this package stays domain-agnostic.
type PickItem struct {
	Title    string // primary line (issue/work-item title)
	Subtitle string // e.g. "#42 · open"
	Body     string // detail shown in the right pane
}

// PickTicket shows an interactive split-pane picker over items and returns the
// selected index into items. preselect is the row the cursor starts on (e.g.
// the highest-likelihood match); pass 0 for the first row. skip is true when
// the user chooses to skip (Esc/q) or there are no items. createNew is true
// when the user asks to create a new ticket (the 'n' key). On a non-interactive
// stdin (piped/CI) it falls back to a numbered-list prompt.
func PickTicket(items []PickItem, preselect int) (selected int, skip bool, createNew bool, err error) {
	if len(items) == 0 {
		return -1, true, false, nil
	}
	if !pickerIsTTY() {
		return pickTicketFallback(items)
	}

	p := tea.NewProgram(newPickerModel(items, preselect), tea.WithAltScreen())
	res, err := p.Run()
	if err != nil {
		return -1, true, false, err
	}
	fm, ok := res.(pickerModel)
	if !ok {
		return -1, true, false, nil
	}
	if fm.create {
		return -1, false, true, nil
	}
	if fm.chosen < 0 {
		return -1, true, false, nil
	}
	return fm.chosen, false, false, nil
}

// --- bubbletea model ---

type pickerModel struct {
	items     []PickItem
	visible   []int // indices into items, after filtering
	cursor    int   // index into visible
	chosen    int   // chosen items[] index; -1 = none
	create    bool  // true when the user asked to create a new ticket
	filtering bool
	filter    string
	width     int
	height    int
}

func newPickerModel(items []PickItem, preselect int) pickerModel {
	m := pickerModel{items: items, chosen: -1, width: 80, height: 24}
	m.recompute()
	if preselect > 0 && preselect < len(m.visible) {
		m.cursor = preselect
	}
	return m
}

func (m *pickerModel) recompute() {
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

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
				m.chosen = -1
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
		case "n":
			m.create = true
			return m, tea.Quit
		case "enter":
			if len(m.visible) > 0 {
				m.chosen = m.visible[m.cursor]
			}
			return m, tea.Quit
		case "esc", "q", "ctrl+c":
			m.chosen = -1
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m pickerModel) View() string {
	width := m.width
	if width < 40 {
		width = 80
	}
	height := m.height
	if height < 10 {
		height = 24
	}
	bodyH := height - 4 // header + filter/help lines

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

	// Left: scrollable list
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

	// Right: selected body
	detail := ""
	if len(m.visible) > 0 {
		it := m.items[m.visible[m.cursor]]
		detail = lipgloss.NewStyle().Bold(true).Render(it.Title) + "\n" +
			dimStyle.Render(it.Subtitle) + "\n\n" + it.Body
	}
	bodyPane := lipgloss.NewStyle().Width(bodyW).Height(bodyH).Padding(0, 1).Render(detail)

	header := lipgloss.NewStyle().Bold(true).Render("Link this commit to a ticket")
	var help string
	if m.filtering {
		help = dimStyle.Render(fmt.Sprintf("filter: %s_  (enter to apply · esc to clear)", m.filter))
	} else {
		help = dimStyle.Render("↑/↓ move · / filter · enter link · n new · esc/q skip")
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, listPane, bodyPane)
	return header + "\n" + body + "\n" + help
}

// --- helpers ---

func truncatePicker(s string, max int) string {
	if max < 1 {
		max = 1
	}
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}

func pickerIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func pickTicketFallback(items []PickItem) (int, bool, bool, error) {
	fmt.Println("Link this commit to a ticket:")
	for i, it := range items {
		fmt.Printf("  %d. %s  %s\n", i+1, it.Subtitle, it.Title)
	}
	fmt.Print("Enter number to link ('n' to create new, 's'/Enter to skip): ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.TrimSpace(line)
	if strings.EqualFold(line, "n") {
		return -1, false, true, nil
	}
	if line == "" || strings.EqualFold(line, "s") {
		return -1, true, false, nil
	}
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > len(items) {
		return -1, true, false, nil
	}
	return n - 1, false, false, nil
}
