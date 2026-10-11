package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseOptionsDefaults(t *testing.T) {
	opt, err := parseOptions(nil)
	if err != nil {
		t.Fatalf("parseOptions(nil) error = %v", err)
	}
	if opt.WeekStart != time.Sunday {
		t.Errorf("WeekStart = %v, want Sunday", opt.WeekStart)
	}
	if opt.Help {
		t.Error("Help = true, want false")
	}
	if opt.StoragePath != "" {
		t.Errorf("StoragePath = %q, want empty", opt.StoragePath)
	}
	now := time.Now()
	want := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	if !opt.Start.Equal(want) {
		t.Errorf("Start = %v, want today midnight %v", opt.Start, want)
	}
}

func TestParseOptionsStartDate(t *testing.T) {
	opt, err := parseOptions([]string{"--start", "2025-03-15"})
	if err != nil {
		t.Fatal(err)
	}
	if DateKey(opt.Start) != "2025-03-15" {
		t.Errorf("Start = %s, want 2025-03-15", DateKey(opt.Start))
	}
}

func TestParseOptionsInvalidStartIsError(t *testing.T) {
	if _, err := parseOptions([]string{"--start", "not-a-date"}); err == nil {
		t.Fatal("parseOptions(bad start) error = nil, want error")
	}
}

func TestParseOptionsWeekStart(t *testing.T) {
	opt, err := parseOptions([]string{"--week-start", "mon"})
	if err != nil {
		t.Fatal(err)
	}
	if opt.WeekStart != time.Monday {
		t.Errorf("WeekStart = %v, want Monday", opt.WeekStart)
	}
	if _, err := parseOptions([]string{"--week-start", "funday"}); err == nil {
		t.Fatal("parseOptions(bad week-start) error = nil, want error")
	}
}

func TestParseOptionsTodayOverridesStart(t *testing.T) {
	opt, err := parseOptions([]string{"--start", "2000-01-01", "--today"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if DateKey(opt.Start) != DateKey(now) {
		t.Errorf("Start = %s, want today %s", DateKey(opt.Start), DateKey(now))
	}
}

func TestParseOptionsStorage(t *testing.T) {
	opt, err := parseOptions([]string{"--storage", "/tmp/x/cal.json"})
	if err != nil {
		t.Fatal(err)
	}
	if opt.StoragePath != "/tmp/x/cal.json" {
		t.Errorf("StoragePath = %q", opt.StoragePath)
	}
}

func TestResolveStoragePath(t *testing.T) {
	if got := resolveStoragePath("/custom/cal.json"); got != "/custom/cal.json" {
		t.Errorf("override = %q, want /custom/cal.json", got)
	}
	got := resolveStoragePath("")
	dir, err := os.UserConfigDir()
	if err == nil && dir != "" {
		want := filepath.Join(dir, "term-calendar", "calendar.json")
		if got != want {
			t.Errorf("default = %q, want %q", got, want)
		}
	} else if got != ".calendar.json" {
		t.Errorf("fallback = %q, want .calendar.json", got)
	}
}

func TestRunHelpPrintsUsageWithoutLaunchingTUI(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"--help"}, &buf); err != nil {
		t.Fatalf("run(--help) error = %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Usage", "--week-start", "--storage", "j / k", "overlay"} {
		if !strings.Contains(out, want) {
			t.Errorf("usage output missing %q:\n%s", want, out)
		}
	}
}

func TestRunBadFlagReturnsError(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"--start", "nope"}, &buf); err == nil {
		t.Fatal("run(bad flag) error = nil, want error")
	}
}

func TestLoadWarning(t *testing.T) {
	if w := loadWarning(filepath.Join(t.TempDir(), "does-not-exist.json")); w != "" {
		t.Errorf("loadWarning(missing) = %q, want empty", w)
	}
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w := loadWarning(path); !strings.Contains(w, "could not read") {
		t.Errorf("loadWarning(corrupt) = %q, want warning containing could not read", w)
	}
	if w := loadWarning(renderFixturePath()); w != "" {
		t.Errorf("loadWarning(valid) = %q, want empty", w)
	}
}

// renderFixturePath writes a small valid calendar file for loadWarning tests.
func renderFixturePath() string {
	path := filepath.Join(os.TempDir(), "term-calendar-test-fixture.json")
	_ = saveCalendar(path, EmptyCalendar())
	return path
}
