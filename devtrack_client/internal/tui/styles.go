package tui

import "github.com/charmbracelet/lipgloss"

// Adaptive color palette — light/dark terminal safe.
var (
	ColorAccent  = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}
	ColorSuccess = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"}
	ColorWarning = lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FCD34D"}
	ColorDanger  = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
	ColorInfo    = lipgloss.AdaptiveColor{Light: "#0284C7", Dark: "#38BDF8"}
	ColorMuted   = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	ColorSubtle  = lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#374151"}
	ColorFlash   = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"}
)

// StyleCard is a rounded-border panel with subtle border color and inner padding.
var StyleCard = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(ColorSubtle).
	Padding(0, 1)

// StyleBadge returns a background-colored inline badge with white text.
func StyleBadge(color lipgloss.TerminalColor) lipgloss.Style {
	return lipgloss.NewStyle().
		Background(color).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1).
		Bold(true)
}

var StyleHeader = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
var StyleMuted = lipgloss.NewStyle().Foreground(ColorMuted)
var StyleSection = lipgloss.NewStyle().Bold(true).Foreground(ColorMuted)
