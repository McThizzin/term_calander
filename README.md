# term-calendar

A small terminal month-calendar TUI written in Go with
[Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lipgloss](https://github.com/charmbracelet/lipgloss). The full-screen month grid
renders each day as an equal-sized bordered box (wall-calendar style). Press
`Enter` on the highlighted day to open a day overlay: read notes, add new notes,
or delete selected notes; persistence is immediate.

## Requirements

- Go 1.24+ (developed against the Go 1.27 toolchain)
- Minimum terminal size: 80×24; a too-small message appears if undersized

## Build & run

```sh
go build ./...
go run .
```

`go run .` opens an alternate-screen TUI. Events are stored at
`<user-config-dir>/term-calendar/calendar.json` (typically
`~/.config/term-calendar/calendar.json`); the startup line prints the exact
path. If the user config directory is unavailable it falls back to
`.calendar.json` in the current directory.

## Keybindings

### Calendar view

| Key | Action |
|---|---|
| `j` / `k` / `↑` / `↓` | move focus down/up |
| `h` / `l` / `←` / `→` | previous/next month |
| `Enter` | open the highlighted day (day overlay) |
| `q` / `Ctrl-C` | quit |

### Day overlay

| Key | Action |
|---|---|
| `j` / `k` / `↑` / `↓` | move between notes |
| `a` or `Enter` | add a new note; `Enter` saves, `Esc` cancels |
| `d` or `x` | delete the selected note |
| `Esc` | return to the calendar |
| `q` / `Ctrl-C` | quit |

When adding a note, space and printable characters type into the input; `q`
types a literal `q`. Enter saves (trimming whitespace; blank notes are not
saved), Esc cancels the input but stays in the overlay.

Focus is clamped to the displayed month, so moving past the first/last day
does nothing. Changing months re-anchors focus to the same day-of-month
(clamped to the month length, e.g. Jan 31 → Feb 28).

## Flags

All flags are optional.

| Flag | Meaning |
|---|---|
| `--start YYYY-MM-DD` | start on this date (default: today) |
| `--today` | start on today's date |
| `--storage PATH` | calendar JSON file to read/write |
| `--week-start sun\|mon` | first column of the grid (default `sun`) |
| `--help` | print usage and exit |

Example:

```sh
go run . --start 2025-01-10 --week-start mon --storage /tmp/calendar.json
```

## Storage format

A single JSON document keyed by local calendar date (`YYYY-MM-DD`):

```json
{
  "events": {
    "2025-01-04": [{ "title": "Standup" }],
    "2025-01-06": [{ "title": "Lunch w/ Amy", "time": "12:30" }]
  }
}
```

Writes are crash-safe: the calendar is written to `<path>.tmp` and then
renamed into place under a mutex, so a partial write never corrupts the file
and concurrent saves are serialized. A missing file loads as an empty
calendar; malformed JSON loads as empty and reports the error without
crashing.

## Tests

```sh
go test ./...
go vet ./...
```

The suite covers date math (leap years, grid alignment), storage
(round-trip, missing/corrupt files, atomic save), the model
(navigation, prompt input, persistence command), rendering (title/event
presence, equal row widths, header alignment, overflow indicator), and CLI
parsing.

## Project layout

```
main.go     CLI flags + program wiring
model.go    Model state, Update(), navigation, event input
render.go   Render() + View(): month grid, prompt line, help footer
dates.go    pure calendar math (DaysInMonth, GridStart, GridWeeks, ...)
storage.go  Event/Calendar types, Load, crash-safe Persist
styles.go   Lipgloss theme (DefaultTheme) + PlainTheme for tests
```

## Module path

`go.mod` declares the local module path `term-calendar`. If you push this to
a repository, replace it with the real import path (there are no internal
imports yet, so a rename is mechanical).

## Possible extensions

- Week and agenda views.
- Editing/removing an individual event by line number.
- Holiday and weekend colouring.
- Colour-profile selection flag.
