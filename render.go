package main

// View implements tea.Model. The full month-grid renderer is added in the
// rendering step; this placeholder keeps the model layer compiling until then.
func (m *Model) View() string {
	return m.Title + "\n"
}
