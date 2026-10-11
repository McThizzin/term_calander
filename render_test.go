package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"
)

func fixedModel(t *testing.T, start, now string) (Model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "calendar.json")
	m := NewModel(path, mustDate(t, start))
	m.Now = func() time.Time { return mustDate(t, now) }
	return m, path
}

func sizedModel(t *testing.T, start, now string, w, h int) Model {
	m, _ := fixedModel(t, start, now)
	m.Width, m.Height = w, h
	return m
}

func widths(s string) []int {
	lines := strings.Split(s, "\n")
	out := make([]int, len(lines))
	for i, ln := range lines {
		out[i] = runewidth.StringWidth(ln)
	}
	return out
}

func TestRenderContainsTitleAndDayNotes(t *testing.T) {
	m := sizedModel(t, "2025-01-10", "2025-01-10", 100, 40)
	m.Events = map[string][]Event{
		"2025-01-04": {{Title: "Standup"}},
		"2025-01-06": {{Title: "Lunch"}},
	}
	out := Render(m, PlainTheme)
	for _, want := range []string{"January 2025", "Standup", "Lunch"} {
		if !strings.Contains(out, want) {
			t.Errorf("Render output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderMonthFillsWidthAndHeight(t *testing.T) {
	const w, h = 100, 40
	m := sizedModel(t, "2025-01-10", "2025-01-10", w, h)
	out := Render(m, PlainTheme)
	if !strings.Contains(out, "January 2025") {
		t.Fatalf("missing month title:\n%s", out)
	}
	if !strings.Contains(out, "┌") || !strings.Contains(out, "│") {
		t.Fatalf("month grid has no box borders:\n%s", out)
	}
	ws := widths(out)
	if len(ws) != h {
		t.Fatalf("rendered %d lines, want %d (fill height)", len(ws), h)
	}
	for i, lw := range ws {
		if lw != w {
			t.Errorf("line %d width %d, want %d (fill width)", i, lw, w)
		}
	}
}

func TestRenderBoxesEqualSizeRegardlessOfNotes(t *testing.T) {
	empty := sizedModel(t, "2025-01-10", "2025-01-10", 100, 40)
	busy := sizedModel(t, "2025-01-10", "2025-01-10", 100, 40)
	busy.Events = map[string][]Event{"2025-01-10": {
		{Title: "a"}, {Title: "b"}, {Title: "c"}, {Title: "d"},
		{Title: "e"}, {Title: "f"}, {Title: "g"}, {Title: "h"},
	}}
	we := widths(Render(empty, PlainTheme))
	wb := widths(Render(busy, PlainTheme))
	if len(we) != len(wb) {
		t.Fatalf("line count differs: empty=%d busy=%d", len(we), len(wb))
	}
	for i := range we {
		if we[i] != wb[i] {
			t.Fatalf("line %d width empty=%d busy=%d", i, we[i], wb[i])
		}
	}
}

func TestRenderSixWeekMonthAtMinimumSize(t *testing.T) {
	// March 2025 spans 6 week rows on a Sunday-first grid.
	m := sizedModel(t, "2025-03-15", "2025-03-15", minWidth, minHeight)
	if GridWeeks(m.Month, m.WeekStart) != 6 {
		t.Fatalf("March 2025 should need 6 weeks, got %d", GridWeeks(m.Month, m.WeekStart))
	}
	ws := widths(Render(m, PlainTheme))
	if len(ws) != minHeight {
		t.Fatalf("rendered %d lines at min size, want %d", len(ws), minHeight)
	}
	for i, lw := range ws {
		if lw != minWidth {
			t.Errorf("line %d width %d, want %d", i, lw, minWidth)
		}
	}
}

func TestRenderWideCharactersKeepRowsEqual(t *testing.T) {
	m := sizedModel(t, "2025-01-10", "2025-01-10", 100, 40)
	m.Events = map[string][]Event{
		"2025-01-10": {{Title: "中文标题"}},
		"2025-01-12": {{Title: "plain"}},
	}
	ws := widths(Render(m, PlainTheme))
	for i := range ws {
		if ws[i] != ws[0] {
			t.Fatalf("line %d width %d != %d", i, ws[i], ws[0])
		}
	}
}

func TestRenderTooSmallShowsMessage(t *testing.T) {
	m := sizedModel(t, "2025-01-10", "2025-01-10", 40, 10)
	out := Render(m, PlainTheme)
	if !strings.Contains(strings.ToLower(out), "too small") {
		t.Fatalf("expected a too-small message:\n%s", out)
	}
}
