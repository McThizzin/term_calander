package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func keyType(tp tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: tp}
}

func newTestModel(t *testing.T, start string) (*Model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "calendar.json")
	m := NewModel(path, mustDate(t, start))
	return &m, path
}

func TestNewModelLoadsPersistedEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calendar.json")
	cal := NewCalendar(map[string][]Event{"2025-01-04": {{Title: "Standup"}}})
	if err := saveCalendar(path, cal); err != nil {
		t.Fatal(err)
	}
	m := NewModel(path, mustDate(t, "2025-01-10"))
	if got := m.Events["2025-01-04"]; len(got) != 1 || got[0].Title != "Standup" {
		t.Fatalf("loaded events = %#v, want Standup on 2025-01-04", m.Events)
	}
	if m.Title != "January 2025" {
		t.Fatalf("Title = %q, want January 2025", m.Title)
	}
}

func TestNewModelWithMissingStorageIsEmpty(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	if len(m.Events) != 0 {
		t.Fatalf("Events = %#v, want empty", m.Events)
	}
}

func TestAddEventTrimsAndSkipsEmpty(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	m.AddEvent("   ", m.Focus)
	if len(m.Events) != 0 {
		t.Fatalf("blank title created an event: %#v", m.Events)
	}
	m.AddEvent("  Standup  ", m.Focus)
	got := m.Events[DateKey(m.Focus)]
	if len(got) != 1 || got[0].Title != "Standup" {
		t.Fatalf("AddEvent = %#v, want trimmed Standup", got)
	}
}

func TestMoveFocusClampsWithinDisplayedMonth(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-15")
	// down
	m.Update(keyRunes("j"))
	if DateKey(m.Focus) != "2025-01-16" {
		t.Fatalf("after j focus = %s, want 2025-01-16", DateKey(m.Focus))
	}
	// up
	m.Update(keyRunes("k"))
	if DateKey(m.Focus) != "2025-01-15" {
		t.Fatalf("after k focus = %s, want 2025-01-15", DateKey(m.Focus))
	}
	// clamp at top of month
	m.Focus = mustDate(t, "2025-01-01")
	m.Update(keyRunes("k"))
	if DateKey(m.Focus) != "2025-01-01" {
		t.Fatalf("up past first day moved focus to %s, want clamp 2025-01-01", DateKey(m.Focus))
	}
	// clamp at bottom of month
	m.Focus = mustDate(t, "2025-01-31")
	m.Update(keyRunes("j"))
	if DateKey(m.Focus) != "2025-01-31" {
		t.Fatalf("down past last day moved focus to %s, want clamp 2025-01-31", DateKey(m.Focus))
	}
}

func TestShiftMonthUpdatesTitleAndFocus(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-31")
	m.Update(keyRunes("l"))
	if m.Title != "February 2025" {
		t.Fatalf("Title = %q, want February 2025", m.Title)
	}
	if DateKey(m.Focus) != "2025-02-28" {
		t.Fatalf("focus after next-month = %s, want re-anchored 2025-02-28", DateKey(m.Focus))
	}
	m.Update(keyRunes("h"))
	if m.Title != "January 2025" {
		t.Fatalf("Title = %q, want January 2025", m.Title)
	}
}

func TestGridStartTracksWeekStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calendar.json")
	m := NewModelWithWeekStart(path, mustDate(t, "2025-01-15"), time.Monday)
	if DateKey(m.Start) != "2024-12-30" {
		t.Fatalf("Start = %s, want Monday-aligned 2024-12-30", DateKey(m.Start))
	}
}

func TestEnterPromptSubmitPersists(t *testing.T) {
	m, path := newTestModel(t, "2025-01-10")
	if _, cmd := m.Update(keyType(tea.KeyEnter)); cmd != nil || !m.Prompting {
		t.Fatalf("Enter should open prompt with no cmd; Prompting=%v cmd=%v", m.Prompting, cmd)
	}
	m.Update(keyRunes("Lunch"))
	if m.Input != "Lunch" {
		t.Fatalf("Input = %q, want Lunch", m.Input)
	}
	_, cmd := m.Update(keyType(tea.KeyEnter))
	if m.Prompting || m.Input != "" {
		t.Fatalf("submit left Prompting=%v Input=%q", m.Prompting, m.Input)
	}
	evs := m.Events["2025-01-10"]
	if len(evs) != 1 || evs[0].Title != "Lunch" {
		t.Fatalf("events after submit = %#v", m.Events)
	}
	if cmd == nil {
		t.Fatal("submit returned nil persist cmd")
	}
	if msg := cmd(); msg != (savedMsg{}) {
		t.Fatalf("persist cmd msg = %#v, want savedMsg{}", msg)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Events["2025-01-10"]) != 1 {
		t.Fatalf("reloaded events = %#v, want persisted Lunch", reloaded.Events)
	}
}

func TestEnterSubmitBlankDoesNotPersistOrCreateEvent(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	m.Update(keyType(tea.KeyEnter)) // open
	m.Update(keyRunes("   "))
	_, cmd := m.Update(keyType(tea.KeyEnter))
	if cmd != nil {
		t.Fatal("blank submit returned a persist cmd, want nil")
	}
	if len(m.Events) != 0 {
		t.Fatalf("blank submit created events: %#v", m.Events)
	}
}

func TestEscapeCancelsPromptDiscardingInput(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	m.Update(keyType(tea.KeyEnter))
	m.Update(keyRunes("abc"))
	m.Update(keyType(tea.KeyEscape))
	if m.Prompting || m.Input != "" {
		t.Fatalf("after Esc Prompting=%v Input=%q, want false/empty", m.Prompting, m.Input)
	}
	if len(m.Events) != 0 {
		t.Fatalf("Esc persisted events: %#v", m.Events)
	}
}

func TestBackspaceEditsInput(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	m.Update(keyType(tea.KeyEnter))
	m.Update(keyRunes("abc"))
	m.Update(keyType(tea.KeyBackspace))
	if m.Input != "ab" {
		t.Fatalf("Input = %q after backspace, want ab", m.Input)
	}
	// backspace on empty input is a no-op
	m.Input = ""
	m.Update(keyType(tea.KeyBackspace))
	if m.Input != "" {
		t.Fatalf("Input = %q, want empty", m.Input)
	}
}

func TestQQuitsNormallyButTypesWhilePrompting(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	if _, cmd := m.Update(keyRunes("q")); cmd == nil {
		t.Fatal("q in normal mode should quit")
	}
	m2, _ := newTestModel(t, "2025-01-10")
	m2.Update(keyType(tea.KeyEnter))
	m2.Update(keyRunes("q"))
	if m2.Input != "q" {
		t.Fatalf("q while prompting = %q, want typed into input", m2.Input)
	}
}

func TestCtrlCQuits(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	if _, cmd := m.Update(keyType(tea.KeyCtrlC)); cmd == nil {
		t.Fatal("ctrl+c should quit")
	}
}

func TestCursorBlinksOnToggleMsgOnly(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init should schedule the blink tick")
	}
	if m.CursorOn {
		t.Fatal("CursorOn should start false")
	}
	_, cmd := m.Update(cursorToggleMsg{})
	if !m.CursorOn {
		t.Fatal("cursorToggleMsg should turn CursorOn true")
	}
	if cmd == nil {
		t.Fatal("cursorToggleMsg should schedule the next tick")
	}
	// a navigation key must not itself schedule a blink chain
	if _, cmd := m.Update(keyRunes("j")); cmd != nil {
		t.Fatalf("navigation key returned cmd %v, want nil", cmd)
	}
}

// Regression: bubbletea classifies a lone space press as KeySpace, not
// KeyRunes; it must still be typed into the event input.
func TestSpacebarTypesIntoInput(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	m.Update(keyType(tea.KeyEnter))
	m.Update(keyType(tea.KeySpace))
	m.Update(keyRunes("ok"))
	if m.Input != " ok" {
		t.Fatalf("Input = %q, want %q", m.Input, " ok")
	}
}

// Regression: save failures must surface to the user, not vanish.
func TestSaveErrorSetsStatusAndRenderShowsIt(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	m.Update(errMsg("disk full"))
	if !strings.Contains(m.Status, "disk full") {
		t.Fatalf("Status = %q, want it to mention the save failure", m.Status)
	}
	out := Render(*m, PlainTheme)
	if !strings.Contains(out, "disk full") {
		t.Errorf("render output missing save error status:\n%s", out)
	}

	m2, _ := newTestModel(t, "2025-01-10")
	m2.Update(savedMsg{})
	if m2.Status != "" {
		t.Fatalf("Status = %q after successful save, want empty", m2.Status)
	}
}
