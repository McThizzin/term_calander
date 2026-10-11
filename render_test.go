package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

func fixedModel(t *testing.T, start, now string) (Model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "calendar.json")
	m := NewModel(path, mustDate(t, start))
	m.Now = func() time.Time { return mustDate(t, now) }
	return m, path
}

func TestRenderContainsTitleAndEventTitles(t *testing.T) {
	m, path := fixedModel(t, "2025-01-10", "2025-01-10")
	if err := saveCalendar(path, NewCalendar(map[string][]Event{
		"2025-01-04": {{Title: "Standup"}},
		"2025-01-06": {{Title: "Lunch w/ Amy"}},
	})); err != nil {
		t.Fatal(err)
	}
	m.Events = map[string][]Event{
		"2025-01-04": {{Title: "Standup"}},
		"2025-01-06": {{Title: "Lunch w/ Amy"}},
	}
	out := Render(m, PlainTheme)
	for _, want := range []string{"January 2025", "Standup", "Lunch w/ Amy"} {
		if !strings.Contains(out, want) {
			t.Errorf("Render output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderGridRowsHaveEqualWidth(t *testing.T) {
	m, _ := fixedModel(t, "2025-01-10", "2025-01-10")
	out := Render(m, PlainTheme)
	lines := strings.Split(out, "\n")
	weeks := GridWeeks(m.Month, m.WeekStart)
	if len(lines) < 2+weeks {
		t.Fatalf("only %d lines, need at least %d", len(lines), 2+weeks)
	}
	rows := lines[2 : 2+weeks]
	w := utf8.RuneCountInString(rows[0])
	if w == 0 {
		t.Fatal("grid row width is zero")
	}
	for i, r := range rows {
		if got := utf8.RuneCountInString(r); got != w {
			t.Errorf("grid row %d width = %d, want %d\nrow=%q", i, got, w, r)
		}
	}
}

func TestRenderHeaderAlignsWithDayColumns(t *testing.T) {
	for _, ws := range []time.Weekday{time.Sunday, time.Monday} {
		path := filepath.Join(t.TempDir(), "calendar.json")
		m := NewModelWithWeekStart(path, mustDate(t, "2025-01-10"), ws)
		m.Now = func() time.Time { return mustDate(t, "2025-01-10") }
		out := Render(m, PlainTheme)
		lines := strings.Split(out, "\n")
		header := lines[1]
		row := lines[2]
		total := utf8.RuneCountInString(row)
		if (total-12)%7 != 0 {
			t.Fatalf("weekStart=%v: row width %d is not 7*colWidth+12", ws, total)
		}
		col := (total - 12) / 7
		names := weekdayNames(ws)
		for c, name := range names {
			want := c * (col + 2)
			if got := strings.Index(header, name); got != want {
				t.Errorf("weekStart=%v: header %q at offset %d, want %d\nheader=%q", ws, name, got, want, header)
			}
		}
	}
}

func TestRenderLeadingBlanksForOffsetMonth(t *testing.T) {
	// Jan 2025 starts on Wednesday, so with a Sunday-first grid the first three
	// cells (Dec 29/30/31) must be blank and day 1 must sit in column 3.
	path := filepath.Join(t.TempDir(), "calendar.json")
	m := NewModel(path, mustDate(t, "2025-01-10"))
	m.Now = func() time.Time { return mustDate(t, "2025-01-10") }
	out := Render(m, PlainTheme)
	lines := strings.Split(out, "\n")
	row0 := lines[2]
	total := utf8.RuneCountInString(row0)
	col := (total - 12) / 7
	firstDayCol := strings.Index(row0, "1") // day number is right-aligned in its cell
	if firstDayCol != 3*(col+2)+col-1 {
		t.Errorf("day 1 at offset %d, want %d (column 3)", firstDayCol, 3*(col+2)+col-1)
	}
	// "31" (December) must not be rendered anywhere in the first row.
	if strings.Contains(row0, "31") {
		t.Errorf("first row rendered an out-of-month day: %q", row0)
	}
}

func TestRenderPromptLineShowsInputCursor(t *testing.T) {
	m, _ := fixedModel(t, "2025-01-10", "2025-01-10")
	m.Prompting = true
	m.Input = "Party"
	m.CursorOn = true
	out := Render(m, PlainTheme)
	if !strings.Contains(out, "Party") {
		t.Errorf("prompt input missing:\n%s", out)
	}
	if !strings.Contains(out, "2025-01-10") {
		t.Errorf("prompt should name the focused day:\n%s", out)
	}
}

func TestRenderCapsEventPreviewsWithMoreIndicator(t *testing.T) {
	m, _ := fixedModel(t, "2025-01-10", "2025-01-10")
	m.Events = map[string][]Event{
		"2025-01-10": {
			{Title: "a"}, {Title: "b"}, {Title: "c"},
			{Title: "d"}, {Title: "e"}, {Title: "f"},
		},
	}
	out := Render(m, PlainTheme)
	if !strings.Contains(out, "+3 more") {
		t.Errorf("expected an overflow indicator for 6 events (max 4 lines):\n%s", out)
	}
}

// Regression: cells with double-width characters (CJK) must keep every grid
// row at the same *displayed* width, not just the same rune count.
func TestRenderEqualDisplayWidthWithWideCharacters(t *testing.T) {
	m, _ := fixedModel(t, "2025-01-10", "2025-01-10")
	m.Events = map[string][]Event{
		"2025-01-10": {{Title: "中文标题"}},
		"2025-01-12": {{Title: "plain"}},
	}
	out := Render(m, PlainTheme)
	lines := strings.Split(out, "\n")
	weeks := GridWeeks(m.Month, m.WeekStart)
	rows := lines[2 : 2+weeks]
	w := runewidth.StringWidth(rows[0])
	if w == 0 {
		t.Fatal("grid row display width is zero")
	}
	for i, r := range rows {
		if got := runewidth.StringWidth(r); got != w {
			t.Errorf("grid row %d displayed width = %d, want %d\nrow=%q", i, got, w, r)
		}
	}
}

// Regression: long event titles must be clamped to a sane column width with an
// ellipsis instead of inflating every column.
func TestRenderCapsColumnWidthForLongTitles(t *testing.T) {
	long := "abcdefghijklmnopqrstuvwxyz0123456789"
	m, _ := fixedModel(t, "2025-01-10", "2025-01-10")
	m.Events = map[string][]Event{"2025-01-10": {{Title: long}}}
	out := Render(m, PlainTheme)
	lines := strings.Split(out, "\n")
	weeks := GridWeeks(m.Month, m.WeekStart)
	rows := lines[2 : 2+weeks]
	total := utf8.RuneCountInString(rows[0])
	col := (total - 12) / 7
	if col > colWidthCap {
		t.Errorf("column width = %d, want capped at %d", col, colWidthCap)
	}
	if !strings.Contains(out, "…") {
		t.Errorf("expected an ellipsis for an over-long title:\n%s", out)
	}
}
