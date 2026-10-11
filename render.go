package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

// View implements tea.Model.
func (m *Model) View() string {
	return Render(*m, DefaultTheme)
}

// weekdayNames returns the seven column labels starting at start.
func weekdayNames(start time.Weekday) []string {
	base := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	out := make([]string, 7)
	for i := range out {
		out[i] = base[(int(start)+i)%7]
	}
	return out
}

// pad right-pads s with spaces to w displayed cells.
func pad(s string, w int) string {
	if n := runewidth.StringWidth(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// dayPadding right-aligns a day number in w cells; a zero day yields blanks.
func dayPadding(w, day int) string {
	if day == 0 {
		return strings.Repeat(" ", w)
	}
	s := fmt.Sprintf("%d", day)
	if n := runewidth.StringWidth(s); n < w {
		return strings.Repeat(" ", w-n) + s
	}
	return s
}

// truncateToWidth cuts s to fit w displayed cells, appending an ellipsis when
// it does not fit.
func truncateToWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= w {
		return s
	}
	var b strings.Builder
	width := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if width+rw > w-1 {
			break
		}
		width += rw
		b.WriteRune(r)
	}
	return b.String() + "…"
}

// padToWidth truncates s ANSI-aware to w cells and then right-pads with spaces
// so every line of the result is exactly w displayed cells wide. It operates
// per line because ansi.Truncate counts cells cumulatively across newlines.
func padToWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		ln = ansi.Truncate(ln, w, "")
		if n := ansi.StringWidth(ln); n < w {
			ln += strings.Repeat(" ", w-n)
		}
		lines[i] = ln
	}
	return strings.Join(lines, "\n")
}

// boxPreviewLines renders up to max preview lines for a day's events within
// width cells, adding a "+N more" marker when events overflow.
func boxPreviewLines(evs []Event, max, width int) []string {
	if max <= 0 || len(evs) == 0 {
		return nil
	}
	var out []string
	if len(evs) <= max {
		for _, e := range evs {
			out = append(out, truncateToWidth("• "+e.Title, width))
		}
		return out
	}
	for i := 0; i < max-1; i++ {
		out = append(out, truncateToWidth("• "+evs[i].Title, width))
	}
	out = append(out, truncateToWidth(fmt.Sprintf("+%d more", len(evs)-(max-1)), width))
	return out
}

// cell holds the pre-rendered content for one grid position.
type cell struct {
	day      int      // 0 when the cell is blank (outside the displayed month)
	previews []string // event preview lines, already clipped to the box width
	today    bool
	focus    bool
}

// renderDayBox renders one bordered day cell with an innerW x innerH content
// area. Every box is drawn at the same size regardless of its content.
func renderDayBox(cl cell, innerW, innerH int, theme Theme) string {
	lines := make([]string, 0, innerH)
	lines = append(lines, pad(dayPadding(innerW, cl.day), innerW))
	for i := 0; i < innerH-1; i++ {
		if i < len(cl.previews) {
			lines = append(lines, pad(cl.previews[i], innerW))
		} else {
			lines = append(lines, strings.Repeat(" ", innerW))
		}
	}
	style := theme.DayBox
	switch {
	case cl.focus:
		style = theme.DayBoxFocus
	case cl.today:
		style = theme.DayBoxToday
	}
	return style.Width(innerW).Height(innerH).Render(strings.Join(lines, "\n"))
}

// tooSmallScreen centers the minimum-size message in a w x h canvas.
func tooSmallScreen(w, h int, theme Theme) string {
	msg := fmt.Sprintf("Terminal too small (%d×%d); need at least %d×%d", w, h, minWidth, minHeight)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, theme.Help.Render(msg))
}

// renderMonth lays out the full-screen month grid of equal day boxes. It is
// pure and deterministic: with PlainTheme (no colors) tests get stable output.
func renderMonth(m Model, theme Theme) string {
	W, H := m.dims()
	if m.tooSmall() {
		return tooSmallScreen(W, H, theme)
	}
	now := m.now()
	weeks := GridWeeks(m.Month, m.WeekStart)
	names := weekdayNames(m.WeekStart)

	footerLines := 1
	if m.Status != "" {
		footerLines++
	}
	gridH := H - 2 - 1 - footerLines // title + header + blank separator + footer
	if gridH < weeks {
		gridH = weeks
	}
	boxH := gridH / weeks
	leftoverV := gridH - boxH*weeks
	innerH := boxH - 2
	if innerH < 1 {
		innerH = 1
	}

	boxW := W / 7
	if boxW < 3 {
		boxW = 3
	}
	innerW := boxW - 2
	gridW := boxW * 7
	leftPad := (W - gridW) / 2
	if leftPad < 0 {
		leftPad = 0
	}

	// Build the cell matrix.
	grid := make([][]cell, weeks)
	for i := 0; i < weeks; i++ {
		grid[i] = make([]cell, 7)
		for c := 0; c < 7; c++ {
			date := m.Start.AddDate(0, 0, i*7+c)
			var cl cell
			if DayInMonth(date, m.Month) {
				key := DateKey(date)
				cl.day = date.Day()
				cl.today = key == DateKey(now)
				cl.focus = key == DateKey(m.Focus)
				cl.previews = boxPreviewLines(m.Events[key], innerH-1, innerW)
			}
			grid[i][c] = cl
		}
	}

	var out []string

	// Title, centered over the terminal.
	title := theme.Title.Render(FormatTitle(m.Month.Year(), m.Month.Month()))
	out = append(out, padToWidth(lipgloss.PlaceHorizontal(W, lipgloss.Center, title), W))

	// Weekday header, each name centered in its box column.
	var hb strings.Builder
	hb.WriteString(strings.Repeat(" ", leftPad))
	for _, n := range names {
		hb.WriteString(lipgloss.PlaceHorizontal(boxW, lipgloss.Center, theme.Header.Render(n)))
	}
	out = append(out, padToWidth(hb.String(), W))

	// Grid rows.
	for i := 0; i < weeks; i++ {
		boxes := make([]string, 7)
		for c := 0; c < 7; c++ {
			boxes[c] = renderDayBox(grid[i][c], innerW, innerH, theme)
		}
		row := strings.Repeat(" ", leftPad) + lipgloss.JoinHorizontal(lipgloss.Top, boxes...)
		out = append(out, padToWidth(row, W))
	}

	// Absorb the leftover rows so the grid stays bottom-aligned.
	for i := 0; i < leftoverV; i++ {
		out = append(out, padToWidth("", W))
	}

	// Blank separator, help footer, optional status line.
	out = append(out, padToWidth("", W))
	help := fmt.Sprintf("j/k move · h/l months · Enter open day · q quit · %s",
		now.Format("Mon 2006-01-02 15:04"))
	out = append(out, padToWidth(theme.Help.Render(help), W))
	if m.Status != "" {
		out = append(out, padToWidth(theme.Help.Render("⚠ "+m.Status), W))
	}

	return strings.Join(out, "\n")
}

// Render renders the current view. It is pure and takes the theme by value so
// tests can pin deterministic output with PlainTheme.
func Render(m Model, theme Theme) string {
	return renderMonth(m, theme)
}
