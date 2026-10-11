package main

import "time"

// dateKeyLayout is the canonical date-key format used for map keys and the
// on-disk JSON (local calendar date, YYYY-MM-DD).
const dateKeyLayout = "2006-01-02"

// DaysInMonth reports the number of days in t's month and year (leap-aware).
func DaysInMonth(t time.Time) int {
	first := FirstOfMonth(t)
	return first.AddDate(0, 1, -1).Day()
}

// FirstOfMonth returns midnight on the 1st of t's month in t's location.
func FirstOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// GridStart returns the first cell of the month grid containing t: the
// week-start day on or before the 1st of t's month. weekStart is the weekday
// that begins each row (time.Sunday or time.Monday).
func GridStart(t time.Time, weekStart time.Weekday) time.Time {
	offset := (int(FirstOfMonth(t).Weekday()) - int(weekStart) + 7) % 7
	return FirstOfMonth(t).AddDate(0, 0, -offset)
}

// GridWeeks reports how many week rows are needed to display t's month.
func GridWeeks(t time.Time, weekStart time.Weekday) int {
	offset := (int(FirstOfMonth(t).Weekday()) - int(weekStart) + 7) % 7
	return (offset + DaysInMonth(t) + 6) / 7
}

// DateKey renders t as a local calendar date key (YYYY-MM-DD).
func DateKey(t time.Time) string {
	return t.Format(dateKeyLayout)
}

// DayInMonth reports whether day falls within month's year and month.
func DayInMonth(day, month time.Time) bool {
	return day.Year() == month.Year() && day.Month() == month.Month()
}

// FormatTitle renders a human month title, e.g. "January 2025".
func FormatTitle(y int, m time.Month) string {
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).Format("January 2006")
}
