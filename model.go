package main

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Terminal-size limits and defaults. The grid needs a floor to render equal
// boxes; before the first WindowSizeMsg the model falls back to the default.
const (
	defaultWidth  = 80
	defaultHeight = 24
	minWidth      = 80
	minHeight     = 24
)

// ViewMode selects which screen the model renders.
type ViewMode int

const (
	ViewMonth ViewMode = iota // full-screen calendar grid
	ViewDay                   // single-day note overlay
)

// Model is the Bubble Tea application state.
type Model struct {
	Title       string             // formatted month title, e.g. "January 2025"
	Month       time.Time          // first day of the displayed month (local)
	Start       time.Time          // top-left grid cell (weekday-aligned, local)
	Focus       time.Time          // focused day (absolute, local)
	Events      map[string][]Event // key == DateKey(day)
	StoragePath string             // full on-disk path
	Input       string             // in-progress typed text while adding
	Mode        ViewMode           // active screen; named Mode because View is the tea.Model method
	NoteSel     int                // selected note index in the day overlay
	Editing     bool               // true while the add-note input is open
	WeekStart   time.Weekday       // first column of the grid (Sunday or Monday)
	CursorOn    bool               // blink state for the input cursor
	Status      string             // transient user-facing status, e.g. save errors
	Now         func() time.Time   // injectable clock (defaults to time.Now)
	Width       int                // terminal columns; 0 until the first WindowSizeMsg
	Height      int                // terminal rows; 0 until the first WindowSizeMsg
}

// cursorToggleMsg flips the input cursor blink state.
type cursorToggleMsg struct{}

// cursorBlinkInterval is how long each cursor blink phase lasts.
const cursorBlinkInterval = 800 * time.Millisecond

// blinkCmd schedules the next cursor blink.
func blinkCmd() tea.Cmd {
	return tea.Tick(cursorBlinkInterval, func(time.Time) tea.Msg { return cursorToggleMsg{} })
}

// NewModel builds a model starting on start, with a Sunday-first grid.
func NewModel(storagePath string, start time.Time) Model {
	return NewModelWithWeekStart(storagePath, start, time.Sunday)
}

// NewModelWithWeekStart builds a model whose grid begins on weekStart.
func NewModelWithWeekStart(storagePath string, start time.Time, weekStart time.Weekday) Model {
	cal, _ := Load(storagePath)
	if cal == nil {
		cal = EmptyCalendar()
	}
	month := FirstOfMonth(start)
	return Model{
		Title:       FormatTitle(month.Year(), month.Month()),
		Month:       month,
		Start:       GridStart(month, weekStart),
		Focus:       start,
		Events:      cal.Events,
		StoragePath: storagePath,
		WeekStart:   weekStart,
		Now:         time.Now,
	}
}

// Init starts the cursor blink loop.
func (m *Model) Init() tea.Cmd { return blinkCmd() }

// now returns the current time using the injected clock when present.
func (m *Model) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

// FocusedDay returns the currently focused day.
func (m *Model) FocusedDay() time.Time { return m.Focus }

// dims returns the terminal size, falling back to a default before the first
// WindowSizeMsg arrives.
func (m *Model) dims() (int, int) {
	w, h := m.Width, m.Height
	if w <= 0 {
		w = defaultWidth
	}
	if h <= 0 {
		h = defaultHeight
	}
	return w, h
}

// tooSmall reports whether the terminal cannot show the full grid.
func (m *Model) tooSmall() bool {
	w, h := m.dims()
	return w < minWidth || h < minHeight
}

// AddEvent appends a trimmed, non-empty event to day's list.
func (m *Model) AddEvent(title string, day time.Time) {
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	if m.Events == nil {
		m.Events = map[string][]Event{}
	}
	key := DateKey(day)
	m.Events[key] = append(m.Events[key], Event{Title: title, Created: m.now().UnixMilli()})
}

// DeleteEvent removes the event at idx from day's list, dropping the map key
// when the list becomes empty. It reports whether anything was removed.
func (m *Model) DeleteEvent(day time.Time, idx int) bool {
	key := DateKey(day)
	evs := m.Events[key]
	if idx < 0 || idx >= len(evs) {
		return false
	}
	evs = append(evs[:idx], evs[idx+1:]...)
	if len(evs) == 0 {
		delete(m.Events, key)
	} else {
		m.Events[key] = evs
	}
	return true
}

// openDay switches to the single-day overlay for day.
func (m *Model) openDay(day time.Time) {
	m.Focus = day
	m.Mode = ViewDay
	m.NoteSel = 0
	m.Editing = false
	m.Input = ""
}

// closeDay returns to the month grid.
func (m *Model) closeDay() {
	m.Mode = ViewMonth
	m.NoteSel = 0
	m.Editing = false
	m.Input = ""
}

// dayEvents returns the focused day's notes.
func (m *Model) dayEvents() []Event { return m.Events[DateKey(m.Focus)] }

// moveNoteSel shifts the note selection by delta and clamps it in range.
func (m *Model) moveNoteSel(delta int) {
	m.NoteSel += delta
	m.clampNoteSel()
}

// clampNoteSel keeps NoteSel within the focused day's notes (0 when empty).
func (m *Model) clampNoteSel() {
	n := len(m.dayEvents())
	switch {
	case n == 0:
		m.NoteSel = 0
	case m.NoteSel < 0:
		m.NoteSel = 0
	case m.NoteSel > n-1:
		m.NoteSel = n - 1
	}
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case cursorToggleMsg:
		m.CursorOn = !m.CursorOn
		return m, blinkCmd()
	case savedMsg:
		m.Status = ""
		return m, nil
	case errMsg:
		m.Status = "save failed: " + string(msg)
		return m, nil
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m, m.handleKey(msg)
	}
	return m, nil
}

// handleKey routes a key press to the handler for the active screen.
func (m *Model) handleKey(k tea.KeyMsg) tea.Cmd {
	if m.Mode == ViewDay {
		return m.handleDayKey(k)
	}
	return m.handleMonthKey(k)
}

// handleMonthKey handles keys while the month grid is showing.
func (m *Model) handleMonthKey(k tea.KeyMsg) tea.Cmd {
	switch k.Type {
	case tea.KeyCtrlC:
		return tea.Quit
	case tea.KeyEnter:
		m.openDay(m.Focus)
		return nil
	case tea.KeyUp:
		m.moveFocus(-1)
		return nil
	case tea.KeyDown:
		m.moveFocus(1)
		return nil
	case tea.KeyLeft:
		m.shiftMonth(-1)
		return nil
	case tea.KeyRight:
		m.shiftMonth(1)
		return nil
	case tea.KeyRunes:
		return m.handleMonthRunes(k.Runes)
	}
	return nil
}

// handleMonthRunes maps j/k/h/l to navigation and q to quit.
func (m *Model) handleMonthRunes(runes []rune) tea.Cmd {
	for _, r := range runes {
		switch r {
		case 'q':
			return tea.Quit
		case 'j':
			m.moveFocus(1)
		case 'k':
			m.moveFocus(-1)
		case 'h':
			m.shiftMonth(-1)
		case 'l':
			m.shiftMonth(1)
		}
	}
	return nil
}

// handleDayKey handles keys while the day overlay is showing.
func (m *Model) handleDayKey(k tea.KeyMsg) tea.Cmd {
	if m.Editing {
		return m.handleAddKey(k)
	}
	switch k.Type {
	case tea.KeyCtrlC:
		return tea.Quit
	case tea.KeyEscape:
		m.closeDay()
		return nil
	case tea.KeyUp:
		m.moveNoteSel(-1)
		return nil
	case tea.KeyDown:
		m.moveNoteSel(1)
		return nil
	case tea.KeyEnter:
		m.Editing = true
		m.Input = ""
		return nil
	case tea.KeyDelete:
		return m.deleteSelectedNote()
	case tea.KeyRunes:
		return m.handleDayRunes(k.Runes)
	}
	return nil
}

// handleDayRunes maps j/k to selection, a to add, d/x to delete, q to quit.
func (m *Model) handleDayRunes(runes []rune) tea.Cmd {
	for _, r := range runes {
		switch r {
		case 'q':
			return tea.Quit
		case 'j':
			m.moveNoteSel(1)
		case 'k':
			m.moveNoteSel(-1)
		case 'a':
			m.Editing = true
			m.Input = ""
			return nil
		case 'd', 'x':
			return m.deleteSelectedNote()
		}
	}
	return nil
}

// handleAddKey edits the in-progress note title. In this mode letters (even q)
// are typed rather than interpreted as commands.
func (m *Model) handleAddKey(k tea.KeyMsg) tea.Cmd {
	switch k.Type {
	case tea.KeyCtrlC:
		return tea.Quit
	case tea.KeyEscape:
		m.Editing = false
		m.Input = ""
		return nil
	case tea.KeyBackspace:
		if r := []rune(m.Input); len(r) > 0 {
			m.Input = string(r[:len(r)-1])
		}
		return nil
	case tea.KeySpace:
		m.Input += " "
		return nil
	case tea.KeyEnter:
		return m.commitNote()
	case tea.KeyRunes:
		m.Input += printableRunes(k.Runes)
		return nil
	}
	return nil
}

// commitNote finalises the add-note input: it trims, closes the editor, stores
// the note on the focused day, and returns a persist command (nil if blank).
func (m *Model) commitNote() tea.Cmd {
	title := strings.TrimSpace(m.Input)
	m.Editing = false
	m.Input = ""
	if title == "" {
		return nil
	}
	m.AddEvent(title, m.Focus)
	m.NoteSel = len(m.dayEvents()) - 1
	return NewCalendar(m.Events).Persist(m.StoragePath)
}

// deleteSelectedNote removes the selected note and persists. It is a no-op
// (nil command) when there is nothing selected.
func (m *Model) deleteSelectedNote() tea.Cmd {
	if !m.DeleteEvent(m.Focus, m.NoteSel) {
		return nil
	}
	m.clampNoteSel()
	return NewCalendar(m.Events).Persist(m.StoragePath)
}

// moveFocus shifts focus by delta days but clamps it within the displayed
// month so navigation never leaves the visible grid.
func (m *Model) moveFocus(delta int) {
	candidate := m.Focus.AddDate(0, 0, delta)
	if DayInMonth(candidate, m.Month) {
		m.Focus = candidate
	}
}

// shiftMonth moves the displayed month by delta, realigns the grid, and
// re-anchors focus to the same day-of-month (clamped to the new month length).
func (m *Model) shiftMonth(delta int) {
	m.Month = FirstOfMonth(m.Month).AddDate(0, delta, 0)
	m.Start = GridStart(m.Month, m.WeekStart)
	m.Title = FormatTitle(m.Month.Year(), m.Month.Month())
	dom := m.Focus.Day()
	if last := DaysInMonth(m.Month); dom > last {
		dom = last
	}
	m.Focus = time.Date(m.Month.Year(), m.Month.Month(), dom, 0, 0, 0, 0, m.Month.Location())
}

// printableRunes keeps only printable ASCII from a key rune slice.
func printableRunes(runes []rune) string {
	var b strings.Builder
	for _, r := range runes {
		if r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		}
	}
	return b.String()
}
