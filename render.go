package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// maxEventLines caps how many event preview lines are shown per day cell.
const maxEventLines = 4

// colWidthCap bounds every grid column so one huge title cannot blow up the
// whole layout; longer previews are ellipsized.
const colWidthCap = 20

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

// previewLines renders up to maxEventLines preview strings for a day, adding a
// "+N more" indicator when events are dropped.
func previewLines(evs []Event) []string {
	if len(evs) == 0 {
		return nil
	}
	var out []string
	for i, e := range evs {
		if i >= maxEventLines {
			break
		}
		if i == maxEventLines-1 && len(evs) > maxEventLines {
			out = append(out, fmt.Sprintf("+%d more", len(evs)-i))
			break
		}
		title := e.Title
		if e.Time != "" {
			title = e.Time + " " + title
		}
		out = append(out, "• "+title)
	}
	return out
}

// cell holds the pre-rendered content for one grid position.
type cell struct {
	day      int      // 0 when the cell is blank (outside the displayed month)
	previews []string // event preview lines
	today    bool
	focus    bool
}

// Render lays out the month grid, prompt line, and help footer. It is pure and
// deterministic: with PlainTheme (no colors) tests get stable output.
func Render(m Model, theme Theme) string {
	now := m.now()
	weeks := GridWeeks(m.Month, m.WeekStart)
	names := weekdayNames(m.WeekStart)

	colWidth := 0
	for _, n := range names {
		if w := runewidth.StringWidth(n); w > colWidth {
			colWidth = w
		}
	}

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
				cl.previews = previewLines(m.Events[key])
			}
			grid[i][c] = cl
			if w := runewidth.StringWidth(fmt.Sprintf("%d", cl.day)); w > colWidth {
				colWidth = w
			}
			for _, ln := range cl.previews {
				if w := runewidth.StringWidth(ln); w > colWidth {
					colWidth = w
				}
			}
		}
	}
	if colWidth > colWidthCap {
		colWidth = colWidthCap
	}

	totalWidth := 7*colWidth + 6*2 // 6 gaps of two spaces

	var b strings.Builder

	// Title line, centered.
	title := theme.Title.Render(FormatTitle(m.Month.Year(), m.Month.Month()))
	b.WriteString(lipgloss.PlaceHorizontal(totalWidth, lipgloss.Center, title))
	b.WriteByte('\n')

	// Weekday header.
	for c, n := range names {
		if c > 0 {
			b.WriteString("  ")
		}
		b.WriteString(theme.Header.Render(pad(n, colWidth)))
	}
	b.WriteByte('\n')

	// Grid rows.
	for i := 0; i < weeks; i++ {
		maxLines := 1
		for c := 0; c < 7; c++ {
			if n := 1 + len(grid[i][c].previews); n > maxLines {
				maxLines = n
			}
		}
		for ln := 0; ln < maxLines; ln++ {
			for c := 0; c < 7; c++ {
				if c > 0 {
					b.WriteString("  ")
				}
				cl := grid[i][c]
				switch {
				case ln == 0:
					padded := dayPadding(colWidth, cl.day)
					switch {
					case cl.focus:
						b.WriteString(theme.Focus.Render(padded))
					case cl.today:
						b.WriteString(theme.Today.Render(padded))
					default:
						b.WriteString(padded)
					}
				case ln-1 < len(cl.previews):
					b.WriteString(theme.Event.Render(pad(truncateToWidth(cl.previews[ln-1], colWidth), colWidth)))
				default:
					b.WriteString(strings.Repeat(" ", colWidth))
				}
			}
			b.WriteByte('\n')
		}
	}

	// Blank separator, optional prompt line, help footer.
	b.WriteByte('\n')
	if m.Prompting {
		marker := ""
		if m.CursorOn {
			marker = "▌"
		}
		b.WriteString(theme.Help.Render(fmt.Sprintf("Add event for %s: %s%s", DateKey(m.Focus), m.Input, marker)))
		b.WriteByte('\n')
	}
	b.WriteString(theme.Help.Render(fmt.Sprintf("j/k move · h/l months · Enter add · Esc cancel · q quit · %s",
		now.Format("Mon 2006-01-02 15:04"))))
	b.WriteByte('\n')
	if m.Status != "" {
		b.WriteString(theme.Help.Render("⚠ " + m.Status))
		b.WriteByte('\n')
	}

	return b.String()
}
