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

func TestEnterOpensDayOverlayAndEscCloses(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	if _, cmd := m.Update(keyType(tea.KeyEnter)); cmd != nil || m.Mode != ViewDay {
		t.Fatalf("Enter: View=%v cmd=%v, want ViewDay/nil", m.Mode, cmd)
	}
	if m.Editing || m.NoteSel != 0 {
		t.Fatalf("opened overlay Editing=%v NoteSel=%d, want false/0", m.Editing, m.NoteSel)
	}
	m.Update(keyType(tea.KeyEscape))
	if m.Mode != ViewMonth {
		t.Fatalf("Esc left View=%v, want ViewMonth", m.Mode)
	}
}

func TestOverlaySelectionMovesAndClamps(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	m.Events = map[string][]Event{"2025-01-10": {{Title: "a"}, {Title: "b"}, {Title: "c"}}}
	m.Update(keyType(tea.KeyEnter))
	m.Update(keyRunes("j"))
	if m.NoteSel != 1 {
		t.Fatalf("after j NoteSel=%d, want 1", m.NoteSel)
	}
	m.Update(keyType(tea.KeyDown))
	m.Update(keyType(tea.KeyDown))
	if m.NoteSel != 2 {
		t.Fatalf("selection should clamp at last note, got %d", m.NoteSel)
	}
	m.Update(keyRunes("k"))
	m.Update(keyRunes("k"))
	m.Update(keyRunes("k"))
	if m.NoteSel != 0 {
		t.Fatalf("selection should clamp at 0, got %d", m.NoteSel)
	}
}

func TestOverlayAddCommitPersistsAndStaysOpen(t *testing.T) {
	m, path := newTestModel(t, "2025-01-10")
	m.Update(keyType(tea.KeyEnter)) // open overlay
	m.Update(keyRunes("a"))         // start add
	if !m.Editing {
		t.Fatal("a should start adding")
	}
	m.Update(keyType(tea.KeySpace))
	m.Update(keyRunes("Lunch"))
	if m.Input != " Lunch" {
		t.Fatalf("Input = %q, want %q", m.Input, " Lunch")
	}
	_, cmd := m.Update(keyType(tea.KeyEnter)) // commit
	if m.Editing || m.Input != "" || m.Mode != ViewDay {
		t.Fatalf("commit: Editing=%v Input=%q View=%v", m.Editing, m.Input, m.Mode)
	}
	evs := m.Events["2025-01-10"]
	if len(evs) != 1 || evs[0].Title != "Lunch" {
		t.Fatalf("events = %#v, want trimmed Lunch", evs)
	}
	if m.NoteSel != 0 {
		t.Fatalf("NoteSel=%d, want the new note selected", m.NoteSel)
	}
	if cmd == nil {
		t.Fatal("commit returned nil persist cmd")
	}
	if msg := cmd(); msg != (savedMsg{}) {
		t.Fatalf("persist msg = %#v, want savedMsg{}", msg)
	}
	if reloaded, _ := Load(path); len(reloaded.Events["2025-01-10"]) != 1 {
		t.Fatalf("reloaded = %#v, want persisted note", reloaded.Events)
	}
}

func TestOverlayAddBlankAndEscCancel(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	m.Update(keyType(tea.KeyEnter))
	m.Update(keyRunes("a"))
	m.Update(keyRunes("   "))
	if _, cmd := m.Update(keyType(tea.KeyEnter)); cmd != nil || len(m.Events) != 0 {
		t.Fatalf("blank commit cmd=%v events=%#v", cmd, m.Events)
	}
	m.Update(keyRunes("a"))
	m.Update(keyRunes("draft"))
	m.Update(keyType(tea.KeyEscape))
	if m.Editing || m.Input != "" || len(m.Events) != 0 {
		t.Fatalf("Esc: Editing=%v Input=%q events=%#v", m.Editing, m.Input, m.Events)
	}
	if m.Mode != ViewDay {
		t.Fatalf("Esc in add mode must stay in the overlay, View=%v", m.Mode)
	}
}

func TestOverlayDeletePersistsAndKeepsSelectionInRange(t *testing.T) {
	m, path := newTestModel(t, "2025-01-10")
	m.Events = map[string][]Event{"2025-01-10": {{Title: "a"}, {Title: "b"}, {Title: "c"}}}
	_ = saveCalendar(path, NewCalendar(m.Events))
	m.Update(keyType(tea.KeyEnter))
	m.Update(keyRunes("j")) // select b
	_, cmd := m.Update(keyRunes("d"))
	if cmd == nil {
		t.Fatal("delete returned nil persist cmd")
	}
	if got := m.Events["2025-01-10"]; len(got) != 2 || got[1].Title != "c" {
		t.Fatalf("after delete = %#v, want a,c", got)
	}
	if m.NoteSel != 1 {
		t.Fatalf("NoteSel=%d, want 1 (clamped to last)", m.NoteSel)
	}
	// delete the rest; key must be removed
	m.Update(keyRunes("d"))
	m.Update(keyRunes("d"))
	if _, ok := m.Events["2025-01-10"]; ok {
		t.Fatalf("day key should be removed when empty: %#v", m.Events)
	}
	if m.NoteSel != 0 {
		t.Fatalf("NoteSel=%d on empty day, want 0", m.NoteSel)
	}
}

func TestQQuitsInMonthAndListButTypesWhileAdding(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	if _, cmd := m.Update(keyRunes("q")); cmd == nil {
		t.Fatal("q in month view should quit")
	}
	m2, _ := newTestModel(t, "2025-01-10")
	m2.Update(keyType(tea.KeyEnter))
	if _, cmd := m2.Update(keyRunes("q")); cmd == nil {
		t.Fatal("q in overlay list should quit")
	}
	m3, _ := newTestModel(t, "2025-01-10")
	m3.Update(keyType(tea.KeyEnter))
	m3.Update(keyRunes("a"))
	if _, cmd := m3.Update(keyRunes("q")); cmd != nil || m3.Input != "q" {
		t.Fatalf("q while adding: cmd=%v Input=%q, want nil/q", cmd, m3.Input)
	}
}

func TestArrowKeysNavigate(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	m.Update(keyType(tea.KeyDown))
	if DateKey(m.Focus) != "2025-01-11" {
		t.Fatalf("Down focus = %s", DateKey(m.Focus))
	}
	m.Update(keyType(tea.KeyUp))
	if DateKey(m.Focus) != "2025-01-10" {
		t.Fatalf("Up focus = %s", DateKey(m.Focus))
	}
	m.Update(keyType(tea.KeyRight))
	if m.Title != "February 2025" {
		t.Fatalf("Right title = %q", m.Title)
	}
	m.Update(keyType(tea.KeyLeft))
	if m.Title != "January 2025" {
		t.Fatalf("Left title = %q", m.Title)
	}
}

func TestCtrlCQuitsFromOverlayAndAddMode(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	m.Update(keyType(tea.KeyEnter))
	if _, cmd := m.Update(keyType(tea.KeyCtrlC)); cmd == nil {
		t.Fatal("ctrl+c should quit from overlay list")
	}
	m2, _ := newTestModel(t, "2025-01-10")
	m2.Update(keyType(tea.KeyEnter))
	m2.Update(keyRunes("a"))
	if _, cmd := m2.Update(keyType(tea.KeyCtrlC)); cmd == nil {
		t.Fatal("ctrl+c should quit from add mode")
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

func TestWindowSizeMsgStoresDimensions(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.Width != 120 || m.Height != 40 {
		t.Fatalf("dims = %dx%d, want 120x40", m.Width, m.Height)
	}
}

func TestDimsFallbackAndTooSmall(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	if w, h := m.dims(); w != defaultWidth || h != defaultHeight {
		t.Fatalf("unset dims = %dx%d, want %dx%d", w, h, defaultWidth, defaultHeight)
	}
	if m.tooSmall() {
		t.Fatal("unset dims should not be too small")
	}
	m.Width, m.Height = minWidth-1, minHeight
	if !m.tooSmall() {
		t.Fatalf("%dx%d should be too small", m.Width, m.Height)
	}
	m.Width, m.Height = minWidth, minHeight-1
	if !m.tooSmall() {
		t.Fatalf("%dx%d should be too small", m.Width, m.Height)
	}
	m.Width, m.Height = minWidth, minHeight
	if m.tooSmall() {
		t.Fatal("exactly the minimum should not be too small")
	}
}
