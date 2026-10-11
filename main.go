package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// stubModel is a throwaway placeholder used only to prove that the dependency
// graph, build, and TUI runtime work before any logic exists. It is replaced
// by the real Model in the CLI-wiring step.
type stubModel struct{}

func (stubModel) Init() tea.Cmd { return nil }

func (stubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyCtrlC, tea.KeyEscape:
			return stubModel{}, tea.Quit
		case tea.KeyRunes:
			if string(k.Runes) == "q" {
				return stubModel{}, tea.Quit
			}
		}
	}
	return stubModel{}, nil
}

func (stubModel) View() string {
	return "term-calendar (stub)\n\nPress q to quit.\n"
}

func main() {
	p := tea.NewProgram(stubModel{}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "term-calendar:", err)
		os.Exit(1)
	}
}
