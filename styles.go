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
}

// DefaultTheme is the colored theme used by the running program.
var DefaultTheme = Theme{
	Title:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")),
	Header: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("244")),
	Today:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170")),
	Focus:  lipgloss.NewStyle().Bold(true).Reverse(true),
	Event:  lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
	Help:   lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
}

// PlainTheme renders without foreground/background so tests get deterministic
// output regardless of terminal color support.
var PlainTheme = Theme{}
