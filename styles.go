package main

import "github.com/charmbracelet/lipgloss"

// Theme bundles the lipgloss styles used across the UI. Keeping them in one
// struct lets the renderer be exercised with color disabled (PlainTheme) for
// deterministic test output, or with color in the real program (Theme).
type Theme struct {
	Title  lipgloss.Style
	Header lipgloss.Style
	Today  lipgloss.Style
	Focus  lipgloss.Style
	Event  lipgloss.Style
	Help   lipgloss.Style

	// Day-box borders for the wall-calendar grid.
	DayBox      lipgloss.Style
	DayBoxFocus lipgloss.Style
	DayBoxToday lipgloss.Style

	// Day overlay.
	Overlay lipgloss.Style
	Dim     lipgloss.Style
	Cursor  lipgloss.Style
}

// DefaultTheme is the colored theme used by the running program.
var DefaultTheme = Theme{
	Title:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")),
	Header: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("244")),
	Today:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170")),
	Focus:  lipgloss.NewStyle().Bold(true).Reverse(true),
	Event:  lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
	Help:   lipgloss.NewStyle().Foreground(lipgloss.Color("241")),

	DayBox:      lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")),
	DayBoxFocus: lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("212")).Bold(true),
	DayBoxToday: lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("170")),

	Overlay: lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("212")),
	Dim:     lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
	Cursor:  lipgloss.NewStyle().Reverse(true),
}

// PlainTheme renders without foreground/background so tests get deterministic
// output regardless of terminal color support. Box borders are kept so the
// grid structure is visible in plain text.
var PlainTheme = Theme{
	DayBox:      lipgloss.NewStyle().Border(lipgloss.NormalBorder()),
	DayBoxFocus: lipgloss.NewStyle().Border(lipgloss.NormalBorder()),
	DayBoxToday: lipgloss.NewStyle().Border(lipgloss.NormalBorder()),

	Overlay: lipgloss.NewStyle().Border(lipgloss.NormalBorder()),
	Dim:     lipgloss.NewStyle(),
	Cursor:  lipgloss.NewStyle(),
}
