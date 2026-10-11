package main

import (
	"testing"
	"time"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		t.Fatalf("bad test date %q: %v", s, err)
	}
	return d
}

func TestDaysInMonth(t *testing.T) {
	cases := []struct {
		date string
		want int
	}{
		{"2025-01-15", 31},
		{"2025-04-01", 30},
		{"2024-02-10", 29}, // leap year
		{"2025-02-10", 28},
		{"2000-02-10", 29}, // divisible by 400 -> leap
		{"1900-02-10", 28}, // divisible by 100, not 400 -> not leap
		{"2025-12-31", 31},
	}
	for _, c := range cases {
		if got := DaysInMonth(mustDate(t, c.date)); got != c.want {
			t.Errorf("DaysInMonth(%s) = %d, want %d", c.date, got, c.want)
		}
	}
}

func TestGridStart(t *testing.T) {
	cases := []struct {
		name      string
		date      string
		weekStart time.Weekday
		want      string
	}{
		{"sunday-start before a Wednesday 1st", "2025-01-01", time.Sunday, "2024-12-29"},
		{"monday-start before a Wednesday 1st", "2025-01-01", time.Monday, "2024-12-30"},
		{"sunday-start when the 1st is already Sunday", "2026-02-01", time.Sunday, "2026-02-01"},
		{"monday-start when the 1st is Sunday", "2026-02-01", time.Monday, "2026-01-26"},
	}
	for _, c := range cases {
		got := DateKey(GridStart(mustDate(t, c.date), c.weekStart))
		if got != c.want {
			t.Errorf("%s: GridStart(%s, %v) = %s, want %s", c.name, c.date, c.weekStart, got, c.want)
		}
	}
}

func TestGridWeeks(t *testing.T) {
	cases := []struct {
		name string
		date string
		want int
	}{
		{"Jan 2025: Wed 1st, 31 days", "2025-01-01", 5},
		{"Feb 2025: Sat 1st, 28 days", "2025-02-01", 5},
		{"Feb 2026: Sun 1st, 28 days", "2026-02-01", 4},
		{"Feb 2027: Mon 1st, 28 days", "2027-02-01", 5},
		{"Sep 2025: Mon 1st, 30 days", "2025-09-01", 5},
	}
	for _, c := range cases {
		if got := GridWeeks(mustDate(t, c.date), time.Sunday); got != c.want {
			t.Errorf("%s: GridWeeks = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestDayInMonth(t *testing.T) {
	month := mustDate(t, "2025-01-15")
	in := []string{"2025-01-01", "2025-01-15", "2025-01-31"}
	out := []string{"2024-12-29", "2024-12-31", "2025-02-01"}
	for _, s := range in {
		if !DayInMonth(mustDate(t, s), month) {
			t.Errorf("DayInMonth(%s, Jan 2025) = false, want true", s)
		}
	}
	for _, s := range out {
		if DayInMonth(mustDate(t, s), month) {
			t.Errorf("DayInMonth(%s, Jan 2025) = true, want false", s)
		}
	}
}

func TestDateKey(t *testing.T) {
	if got := DateKey(mustDate(t, "2025-07-04")); got != "2025-07-04" {
		t.Errorf("DateKey = %q, want %q", got, "2025-07-04")
	}
}

func TestFormatTitle(t *testing.T) {
	if got := FormatTitle(2025, time.January); got != "January 2025" {
		t.Errorf("FormatTitle = %q, want %q", got, "January 2025")
	}
	if got := FormatTitle(2024, time.February); got != "February 2024" {
		t.Errorf("FormatTitle = %q, want %q", got, "February 2024")
	}
}
