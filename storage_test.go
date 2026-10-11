package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	cal, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("Load(missing) error = %v, want nil", err)
	}
	if cal == nil {
		t.Fatal("Load(missing) returned nil calendar")
	}
	if len(cal.Events) != 0 {
		t.Fatalf("Load(missing) events = %v, want empty", cal.Events)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calendar.json")
	want := NewCalendar(map[string][]Event{
		"2025-01-04": {{Title: "Standup"}},
		"2025-01-06": {{Title: "Lunch w/ Amy"}, {Title: "Review", Time: "14:00"}},
	})
	if err := saveCalendar(path, want); err != nil {
		t.Fatalf("saveCalendar error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	if !reflect.DeepEqual(got.Events, want.Events) {
		t.Fatalf("round trip mismatch:\n got %#v\nwant %#v", got.Events, want.Events)
	}
}

func TestLoadCorruptJSONReturnsErrorWithEmptyCalendar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	cal, err := Load(path)
	if err == nil {
		t.Fatal("Load(corrupt) error = nil, want non-nil")
	}
	if cal == nil || len(cal.Events) != 0 {
		t.Fatalf("Load(corrupt) calendar = %#v, want empty non-nil", cal)
	}
}

func TestPersistCmdSendsSavedMsg(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "calendar.json")
	cal := NewCalendar(map[string][]Event{"2025-03-01": {{Title: "x"}}})
	msg := cal.Persist(path)()
	if _, ok := msg.(savedMsg); !ok {
		t.Fatalf("Persist cmd msg = %T, want savedMsg", msg)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Persist did not create file: %v", err)
	}
}

func TestPersistLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "calendar.json")
	cal := EmptyCalendar()
	cal.Events["2025-05-05"] = []Event{{Title: "y", Created: time.Now().UnixMilli()}}
	if msg := cal.Persist(path)(); msg != (savedMsg{}) {
		t.Fatalf("Persist msg = %#v, want savedMsg{}", msg)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind: err=%v", err)
	}
}

func TestNewCalendarNilSafe(t *testing.T) {
	cal := NewCalendar(nil)
	if cal == nil || cal.Events == nil {
		t.Fatalf("NewCalendar(nil) = %#v, want non-nil with init map", cal)
	}
}
