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

// Model is the Bubble Tea application state.
type Model struct {
	Title       string             // formatted month title, e.g. "January 2025"
	Month       time.Time          // first day of the displayed month (local)
	Start       time.Time          // top-left grid cell (weekday-aligned, local)
	Focus       time.Time          // focused day (absolute, local)
	Events      map[string][]Event // key == DateKey(day)
	StoragePath string             // full on-disk path
	Input       string             // in-progress typed text while adding
	Prompting   bool               // true while the add-event input is open
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

func (m *Model) handleKey(k tea.KeyMsg) tea.Cmd {
	if m.Prompting {
		return m.handlePromptKey(k)
	}
	switch k.Type {
	case tea.KeyCtrlC:
		return tea.Quit
	case tea.KeyEnter:
		m.Prompting = true
		m.Input = ""
		return nil
	case tea.KeyRunes:
		return m.handleNormalRunes(k.Runes)
	}
	return nil
}

func (m *Model) handlePromptKey(k tea.KeyMsg) tea.Cmd {
	switch k.Type {
	case tea.KeyCtrlC:
		return tea.Quit
	case tea.KeyEscape:
		m.Prompting = false
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
		return m.submit()
	case tea.KeyRunes:
		m.Input += printableRunes(k.Runes)
		return nil
	}
	return nil
}

// handleNormalRunes maps j/k/h/l to navigation and q to quit. It returns a
// quit command only when a quit rune was seen.
func (m *Model) handleNormalRunes(runes []rune) tea.Cmd {
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

// submit finalises the prompt: trims the input, stores the event on the
// focused day, and returns a persist command (nil when the input is blank).
func (m *Model) submit() tea.Cmd {
	title := strings.TrimSpace(m.Input)
	m.Prompting = false
	m.Input = ""
	if title == "" {
		return nil
	}
	m.AddEvent(title, m.Focus)
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
