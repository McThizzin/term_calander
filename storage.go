package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// Event is a single calendar entry on a day.
type Event struct {
	Title   string `json:"title"`
	Time    string `json:"time,omitempty"`       // optional freeform "HH:MM"
	Created int64  `json:"created_ms,omitempty"` // unix milliseconds
}

// Calendar is the persisted top-level document: day key -> ordered events.
type Calendar struct {
	Events map[string][]Event `json:"events"`
}

// EmptyCalendar returns a calendar with an initialised (non-nil) event map.
func EmptyCalendar() *Calendar {
	return &Calendar{Events: map[string][]Event{}}
}

// NewCalendar wraps an existing event map, substituting an empty one for nil.
func NewCalendar(events map[string][]Event) *Calendar {
	if events == nil {
		events = map[string][]Event{}
	}
	return &Calendar{Events: events}
}

// savedMsg is delivered to the Bubble Tea loop after a successful persist.
type savedMsg struct{}

// errMsg is delivered to the Bubble Tea loop when a persist fails.
type errMsg string

// Load reads a calendar from path. A missing or unreadable file yields an
// empty calendar and no error; malformed JSON yields an empty calendar plus
// the parse error so the caller can surface it without crashing.
func Load(path string) (*Calendar, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return EmptyCalendar(), nil
		}
		return EmptyCalendar(), nil
	}
	var c Calendar
	if err := json.Unmarshal(data, &c); err != nil {
		return EmptyCalendar(), err
	}
	if c.Events == nil {
		c.Events = map[string][]Event{}
	}
	return &c, nil
}

// storageMu serialises concurrent writers within the process.
var storageMu sync.Mutex

// saveCalendar writes c to path crash-safely: a temp file in the same
// directory is written and then renamed into place, and all writers are
// serialised so a rename never races another write.
func saveCalendar(path string, c *Calendar) error {
	storageMu.Lock()
	defer storageMu.Unlock()

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// snapshot deep-copies the event map so an in-flight save is unaffected by
// later model mutations.
func (c *Calendar) snapshot() *Calendar {
	out := EmptyCalendar()
	if c == nil || c.Events == nil {
		return out
	}
	for k, evs := range c.Events {
		cp := make([]Event, len(evs))
		copy(cp, evs)
		out.Events[k] = cp
	}
	return out
}

// Persist returns a Bubble Tea command that saves the calendar off the update
// loop and reports success (savedMsg) or failure (errMsg) back to the program.
func (c *Calendar) Persist(storagePath string) tea.Cmd {
	snap := c.snapshot()
	return func() tea.Msg {
		if err := saveCalendar(storagePath, snap); err != nil {
			return errMsg(err.Error())
		}
		return savedMsg{}
	}
}
