# term-calendar

A small terminal month-calendar TUI written in Go with
[Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lipgloss](https://github.com/charmbracelet/lipgloss). Navigate a monthly
day grid with vi-style keys and add events to any day; events persist to JSON
and survive restarts.

## Requirements

- Go 1.24+ (developed against the Go 1.27 toolchain)

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

| Key | Action |
|---|---|
| `j` | move focus down one calendar day |
| `k` | move focus up one calendar day |
| `h` | previous month |
| `l` | next month |
| `Enter` | open the "add event" input for the focused day |
| `Enter` (typing) | save the event to the focused day (persists immediately) |
| `Backspace` (typing) | delete the last typed character |
| `Esc` (typing) | cancel the current input, discarding it |
| `q` / `Ctrl-C` | quit |

Arrow keys are not bound; use `j`/`k`/`h`/`l`.

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
