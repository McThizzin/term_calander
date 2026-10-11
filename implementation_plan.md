# Implementation Plan — Term Calendar TUI (Bubble Tea + Lipgloss)

## Project context / environment (verified)
- Working dir: `/home/user/Projects/calander` (currently empty → greenfield).
- Toolchain: Go **1.27** (`go version go1.27.2-X:nodwarf5 linux/amd64`).
- Dependencies confirmed resolvable over the proxy: `github.com/charmbracelet/bubbletea v1.3.7` and `github.com/charmbracelet/lipgloss v1.1.0`.
- **API findings that shape this plan** (verified from module sources):
  - Bubble Tea v1.3.7 exposes keys through its own `package tea`: `KeyMsg` with an exported `.Type` (`tea.KeyRunes`, `tea.KeyEnter`, `tea.KeyBackspace`, `tea.KeyEscape`, `tea.KeyCtrlC`, `tea.KeyBreak`, …) and `.Runes []rune`. Plain-letter presses have `Type==tea.KeyRunes` with `k.Runes == [letter]`. We will match on exported **constants** (robust across versions) rather than string literals, except for Enter where we use the stable `"enter"` via a rune/string check.
  - `lipgloss.Prompt` does **not** exist in v1.1.0 → implement a tiny manual input buffer inside the model instead of depending on it.
  - Standard lipgloss styling API present (`Style{Foreground, Background, Bold, Width, Padding, Border}`, `.Render`, `.Color("#hex"|"name")`). Colors degrade gracefully when not TTY-colored.
- Module path: placeholder `github.com/example/term-calendar`. Replace with the project's real import path once pushed to git (no internal imports yet, so local build is unaffected).

## Scope — MVP feature set
A single **monthly day-grid** calendar with focus navigation and event add/remove + JSON persistence. Explicitly deferred (Phase-2 ideas noted at the end): week/agenda views, multi-event edit by line number, holidays/weekend coloring, color-profile selection flag.

### Keybindings (to implement exactly)
| Key | Action |
|---|---|
| `j` / `↓` | move focus down one calendar day |
| `k` / `↑` | move focus up one calendar day |
| `h` / `←` | previous month |
| `l` / `→` | next month |
| `Enter` (in normal mode) | open "add event" input for the focused day |
| `Enter` (while typing) | submit event to focused day, persist immediately |
| `Backspace` (while typing) | delete last typed character |
| `Esc` (while typing) | cancel current add (discard buffer), stay on focus day |
| `q` or `Ctrl-C` | quit the program |

## File layout (single Go module, all in `package main`)
```
go.mod            # created in step 1
.gitignore        # ./.git ; storage json ; /testdata
main.go           # CLI wiring: flags + tea.NewProgram(model)      [step 6]
styles.go         # lipgloss theme vars + render helpers (colors)   [step 2]
dates.go          # pure calendar math (DaysInMonth, FirstDayOfWeek…) [step 3]
storage.go        # Event type, Calendar map, load/save JSON, persist cmd [step 4]
model.go          # Model struct + NewModel + Update() (+ helpers)   [step 5]
render.go         # Render(m, theme) string + (m.Model) View()       [step 7]
*_test.go         # table-driven render tests + date-math tests       [step 8]
README.md         # usage / keybindings / storage / build/run        [step 9]
```

## Data model & persistence contract (shared by all steps)
- `Event{ Title string; Time string (optional HH:MM freeform); Created int64 }`.
- On-disk JSON shape for a Calendar file:
  ```json
  { "events": { "2025-01-04": [ {"title":"Standup"} ], "2025-01-06": [ {"title":"Lunch w/ Amy"} ] } }
  ```
  Key is the UTC date `"2006-01-02"`. Empty/missing file → `{"events":{}}`.

## Design decisions / assumptions to confirm before execution
1. **Week start**: default **Sunday** first column; flag `--week-start sun|mon` optional (adds minimal logic). Confirm?
2. **Storage location** (default, overridable with `--storage`): `<UserConfigDir>/term-calendar/calendar.json`, e.g. `~/.config/term-calendar/calendar.json`. Fallback to `.calendar.json` in CWD if config dir is unavailable. Confirm?
3. **Timezone**: all calendar math uses the local zone for display but compares dates by UTC date-component (`2006-01-02`). Confirm?
4. **Quit**: `q` and Ctrl-C; no mouse interaction (alt-screen TUI). Confirm?
5. Module path placeholder as above — confirm/replace later.

## Step-by-Step Implementation Strategy

### Phase 1: Scaffold & dependency resolution [Milestone M1]
- **[ ] Step S1** — Create `go.mod` (`module github.com/example/term-calendar`; `go 1.23` for a stable floor that builds on the 1.27 toolchain), then run:
  - `go get github.com/charmbracelet/bubbletea@v1.3.7`
  - `go get github.com/charmbracelet/lipgloss@v1.1.0`
  - `go mod tidy` (network needed for first fetch).
  - Write `.gitignore` ignoring: `.git/`, the storage file (`*.calendar.json`, and the config path glob), `/testdata/`.
- **[ ] Step S2** — Write a minimal but **compiling & runnable** skeleton so the dependency graph + TUI runtime are proven before logic is added. Two files:
  - `styles.go`: package-level lipgloss style values used by everything later (reuse in step 7): e.g. `titleStyle`, `todayStyle`, `focusStyle`, `headerStyle`, `eventStyle`, `helpStyle`. Keep them simple (bold + color, no sizing yet). Also define a **plain (no-color) theme variant** (`var PlainTheme`) whose styles set only `Width`/`Padding` and *no* foreground/background — used by tests so rendered output is deterministic.
  - `main.go`: minimal program — `tea.NewProgram(model{}, tea.WithAltScreen())` where the model's `View()` currently returns a static string `"term-calendar (stub)"`. This proves build + run; will be replaced by full wiring in Phase 6 (step 6).

### Phase 2: Pure logic layer [Milestone M2]
- **[ ] Step S3** — Create `dates.go` with pure, unit-testable helpers:
  ```go
  func DaysInMonth(t time.Time) int        // days in t's month/year (handles Feb/leap years via AddDate(0,0,1).Day()-1)
  func WeekdaysInMonth(t time.Time) []time.Time   // slice length ceil(daysInMonth/7), first element = 1st of month
  func DateKey(t time.Time) string        // t.Format("2006-01-02") (UTC date component)
  func FormatTitle(y int, m time.Month) string // e.g. "January 2025"
  ```
  These are the single source of truth for month/row math used by model + renderer; tests target these directly.

### Phase 3: Data layer [Milestone M3]
- **[ ] Step S4** — Create `storage.go`:
  ```go
  type Event struct { Title string `json:"title"`; Time string `json:"time,omitempty"`; Created int64 `json:"created_ms,omitempty"` }
  type Calendar struct { Events map[string][]Event `json:"events"` }
  func EmptyCalendar() *Calendar
  func Load(path string) (*Calendar, error)   // returns &EmptyCalendar{} on not-found/unreadable (do not error on missing file)
  func (c *Calendar) Persist(storagePath string) tea.Cmd
  ```
  `Persist` must be **crash-safe and serialized**: use a package-level `sync.Mutex` to serialize writers, write to `<path>.tmp` then `os.Rename` into place (`os.WriteFile` + `os.Rename`). Implement as a `tea.Cmd` that runs in a goroutine and sends a private `savedMsg{}` (on success) or `errMsg string` back to the loop. No blocking; tolerate write errors gracefully (return error to log, never crash).
  Also export a helper the model will use:
  ```go
  func NewCalendar(events map[string][]Event) *Calendar // for reconstructing state before save
  ```

### Phase 4: Application model logic [Milestone M4]
- **[ ] Step S5** — Create `model.go`. Exact contract so step 7 (render) can be written against it:
  ```go
  type Model struct {
      Title       string                  // formatted month title, e.g. "January 2025"
      Start        time.Time               // first day of displayed month (local zone; UTC date used for keys/today)
      Focus        time.Time               // focused calendar day pointer (drives selection highlight); may be any absolute date
      Events       map[string][]Event      // key == DateKey(day), value == ordered events
      StoragePath  string                  // full on-disk path (from flag or config dir default)
      Input        string                  // in-progress typed text while adding an event
      Prompting    bool                   // true when showing the "add event" input line
  }

  func NewModel(storagePath string, start time.Time) Model {
      cal := Load(storagePath); if cal==nil { cal = EmptyCalendar() }
      return Model{ Title: FormatTitle(...), Start: start.UTC(), Events: cal.Events /*copy*/, StoragePath: storagePath }
  }

  func (m *Model) AddEvent(title string, day time.Time)   // append Event{title}; title trimmed; non-empty only
  ```
- `Update()` behavior:
  - Match on exported key constants via a type switch `switch msg := msg.(type)` then `.Type`:
    - `tea.KeyCtrlC`, `tea.KeyBreak` → quit (return model, nil).
    - plain letter `'q'` (`k.Type==tea.KeyRunes && string(k.Runes)=="q"`) → quit.
    - `tea.KeyBackspace` while `Prompting` → if `len(m.Input)>0 { m.Input = m.Input[:len-1] }`.
    - `tea.KeyEscape` while `Prompting` → cancel: `m.Prompting=false; m.Input=""`. (discard buffer)
    - `tea.KeyRunes`: for each rune in `k.Runes`, if it's printable and not a control char (`<= 0x7f`), append to `m.Input`; ignore anything else (arrow sequences start with `\x1b`, multi-char combos handled by Alt flag). (Arrows are **not** supported — only `j/k/h/l` letters.)
    - `tea.KeyEnter`: if `!m.Prompting` → begin prompt (`m.Prompting=true`). If `m.Prompting` → submit: `m.AddEvent(strings.TrimSpace(m.Input), m.FocusedDay())`, then set `Input=""`/`Prompting=false`, and return model + a persist command via `NewCalendar(copy).Persist(m.StoragePath)` (or an inline cmd that builds the calendar and saves). Trim title; skip if empty.
  - Navigation:
    - `'j'` → move focus down one day, clamped to not cross into a later month than `Start.AddDate(0,0,weeksInMonth)+lastRow`? Keep simple: `m.Focus = m.Focus.AddDate(0,0,1)` but clamp so it stays within the visible grid range `[today-ish..]` — simplest correct behavior: allow focus anywhere across rows; highlight only applies when inside displayed month. Implement `_clampFocus(m)` to keep Focus between `Start.AddDays(-6)`? To avoid negatives, just ensure `m.Focus` never moves below the first cell of the current week by re-deriving from Start + focused offset. **Simplest robust rule**: store focus as an absolute day; after each move clamp so its UTC date is within `[Start, Start+DaysInMonth-1]`. If a plain up/down would go outside that window (leading/trailing blanks), leave Focus unchanged. This gives intuitive "stay on the grid" behavior with no negative dates.
    - `'h'` → previous month: `m.Start = m.Start.AddDate(0,-1,0)`; then clamp/re-anchor `Focus` to its same day-of-month if it still exists in the new month else keep Focus (still valid date pointer). Update `Title`.
    - `'l'` → next month analogously with `AddDate(0,+1,0)`.
  - Blinking cursor: every `Update()` returns a `tea.Cmd` that toggles a package-level `cursorOn atomic.Bool` every ~800ms (e.g. `func toggleCursorCmd() tea.Cmd`). `View()/Render` reads the shared bool to decide focus styling. Safe because Bubble Tea only reads the model synchronously before the next Update; the flag is set between updates on that loop boundary.
  - Return type `(tea.Model, tea.Cmd)` = `return m, cmd`.

### Phase 5: Rendering [Milestone M5]
- **[ ] Step S7** — Create `render.go` implementing a pure `Render(m Model, theme *Theme) string` plus the method `func (m Model) View() string { return Render(m, Theme) }`. Layout, top-to-bottom:
  1. **Title line**: month title via `theme.titleStyle.Render(FormatTitle(...))`, centered with lipgloss.Center/Bold; subtitle optional showing focus date/key hint dimmed.
  2. **Weekday header row**: `"Sun Mon Tue Wed Thu Fri Sat"` split into per-column cells (Sunday-start grid).
  3. **Grid rows** for `i` in `0..weeksInMonth-1`:
     - For column `c` in `0..6`, compute the concrete date = `m.Start.AddDate(0, i*7+c, 0)`; if its UTC date is outside `[Start … end-of-month]` render it as a blank cell (`"   "`).
     - Otherwise render a **cell** whose first line is the day number (2-digit), followed by up to N=4 short event preview lines below it (e.g. `"• Standup")`, each preview line indented so its start aligns under the date column within that cell.
     - **Column alignment**: precompute a global min `colWidth` = max(weekday-name width, day-number width, longest single-event-preview across all cells). Format every cell's lines to that width and pad; join columns with `"  "` (2 spaces) or `"│"`. Because each row is built independently from the same `colWidth`, header/number column and event-indent align vertically across weeks.
     - Today (`DateKey(focus)==DateKey(now)`): wrap in `theme.todayStyle`; focused day when inside the displayed month: additionally apply `theme.focusStyle` (or rely on the blinking cursor). Mark both unconditionally so today always stands out even if not the focus.
  4. **Footer/help line**: dimmed, lists keybindings (`j/k move · h/l months · Enter add · q quit`). Include current local date/time for orientation.
  - Provide helper `func (m Model) FocusedDay() time.Time { return m.Focus }` and `func dayIsInMonth(day,time.Time) bool`. Keep formatting helpers small and localized to this file so the coder can iterate on spacing easily. Tests call `Render(m, PlainTheme)` (color disabled).

### Phase 6: CLI wiring [Milestone M6]
- **[ ] Step S6** — Rewrite `main.go` to wire the real program (keep step-2 stub replaced):
  ```go
  func main() {
      if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil { log.Fatal(err) } // or handle per-flag errors
  }
  func run(args []string, stdin io.Reader, stdout io.Writer) error {
      // stdlib flag parsing; see flags below
      if showHelp { printUsage(); return nil }
      storage := flagStorageValue(args, defaults...)
      start   := flagStartOrNow(args)         // parse --today/--start "2006-01-02"; default today's local date
      m := NewModel(storagePath, start)
      p, err := tea.NewProgram(m, tea.WithAltScreen())
      if err != nil { return fmt.Errorf("starting program: %w", err) }
      _ = p.Start()
      return nil
  }
  ```
  Flags (all optional): `--today` (start on today's date), `--storage PATH` (override storage location; default = config-dir path from step-1 decision #2), `--week-start sun|mon`, `--help`. Validate month/year ranges (`time.Parse("2006-01-02")`); return an error (with a clear message) on bad input instead of panicking. Print to stderr at startup: where events are stored + version hint, so it's inspectable without the TUI. Keep `flag` package (no extra deps).

### Phase 7: Tests [Milestone M7]
- **[ ] Step S8** — Add tests under `_test.go` in the module root:
  - `dates_test.go`: table-driven for `DaysInMonth`, leap-year Feb, and `WeekdaysInMonth` length/first element (e.g., a Jan that starts on Wednesday → first week has leading blanks). Assert exact UTC date components.
  - `render_test.go`: build a Model with several events spread across days + set `Focus` on one of them; call `Render(m, PlainTheme)`. Assertions:
    - output contains the month title string and each event's title substring (plain theme keeps substrings intact).
    - every grid row has identical visual width (`strings.Count(line,"\n")` split → all but last equal length when split by `\n`).
    - header weekday cells align with day-number column for at least one representative week.
  Keep assertions structural (widths, presence) rather than exact ANSI so they're stable regardless of color support. Tests run via `go test ./...`.

### Phase 8: Documentation [Milestone M8]
- **[ ] Step S9** — Write `README.md`: what it is; install/run (`go run .`); flags/keybindings table (from the KeyBindings table above); storage location + how events persist & where they live; brief extension ideas (week/agenda views, multi-event edit, holidays). Include the module-path placeholder note.

## Verification / acceptance
- `go build ./...` and `go vet ./...` pass clean with no external key package dependency.
- `go test ./...` green for date math + render alignment.
- Manual: run `go run . --today`; navigate months, focus days with j/k, type an event on Enter (submit), Backspace to edit mid-typing, Esc to cancel, h/l across months, quit with q/Ctrl-C; confirm events survive a restart (`--today` again) by re-reading the JSON file.

## Risks & mitigations
- **lipgloss.Prompt removed** → covered by manual input buffer (Step 5), which is also unit-testable.
- **Key API drift across versions** → matched on exported `tea.*` constants + stable `"enter"`, avoiding string-literal key matching; verified against v1.3.7 source.
- **Multi-line event preview misalignment** → fixed global `colWidth` computed before formatting rows (Step 7 spec).
- **Concurrent/partial saves** → serialized mutex + temp-file rename in `Persist`.
