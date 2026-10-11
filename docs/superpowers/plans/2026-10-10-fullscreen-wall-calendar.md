# Full-Screen Wall Calendar + Day Overlay Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign `term-calendar` into a full-screen wall calendar of equal bordered day boxes, with an `Enter`-opened single-day overlay to view/add/remove notes.

**Architecture:** The Bubble Tea model gains a view mode (`ViewMonth`/`ViewDay`), terminal dimensions, and a day-overlay selection/editing state. Rendering is split into a month-grid renderer (a `weeks × 7` matrix of fixed-size bordered boxes sized from the terminal) and a day-overlay renderer (a centered panel composited over the dimmed month grid with `github.com/charmbracelet/x/ansi`). Storage is unchanged: every add/remove calls the existing mutex-guarded, snapshot-based `Calendar.Persist`.

**Tech Stack:** Go 1.24, Bubble Tea v1.3.7, Lipgloss v1.1.0, go-runewidth v0.0.16, charmbracelet/x/ansi v0.10.1.

**Spec:** `docs/superpowers/specs/2026-10-10-fullscreen-wall-calendar-design.md`

## Global Constraints

- Minimum terminal size: **80 columns × 24 rows**. Below that, render a centered "too small" message; `q`/`Ctrl-C` still quit.
- Month grid is always **7 columns** and `GridWeeks(...)` rows (5 or 6); every day box on screen has identical outer width and identical outer height regardless of its notes.
- Do not change the on-disk JSON schema or the local `YYYY-MM-DD` date-key semantics (`dates.go` is untouched).
- Every add and delete persists immediately via `NewCalendar(m.Events).Persist(m.StoragePath)`; `storage.go` is untouched.
- Keep key matching on exported `tea.*` constants plus rune checks for `j/k/h/l/q/a/d/x`.
- `Render` stays pure and takes `theme Theme` by value; tests use `PlainTheme`. `PlainTheme` now includes border styles (plain box-drawing runes, no ANSI color).
- Wide (CJK/emoji) text must keep every row at equal **display** width, measured with `go-runewidth` / `ansi.StringWidth`.

## Review Focus

The spec implies these inputs; no task's happy-path tests cover them by default, and each can bite a user:

1. **Overlay rows must never exceed the panel width.** A long note title that wraps or overflows breaks the centered splice (misaligned box, stray text). → pinned in Task 4.
2. **Terminal resize while the overlay is open.** A `WindowSizeMsg` arriving in `ViewDay` must re-layout or fall back to the too-small message without panicking. → Task 4.
3. **A 6-week month at the minimum height (80×24).** Boxes must stay equal and at least 3 rows tall. → Task 2.
4. **More notes than the overlay body can show.** Selection must scroll into view, and deleting must keep the selection in range. → Task 3 and Task 4.
5. **Wide-character note titles.** Boxes and overlay rows must stay at equal display width. → Task 2 and Task 4.

---

## File Structure

- `model.go` — terminal size, view state, day-overlay selection/editing, key routing, delete/add persistence. (Modify)
- `render.go` — full rewrite: month box grid + day-overlay panel + compositing. (Modify)
- `styles.go` — new themed styles (`DayBox`, `DayBoxFocus`, `DayBoxToday`, `Overlay`, `Dim`); `PlainTheme` gets explicit border styles. (Modify)
- `model_test.go` — sizing + overlay state/key tests; remove prompt tests. (Modify)
- `render_test.go` — box-layout + overlay render tests; remove old offset tests. (Modify)
- `main.go` — updated usage/help keybindings. (Modify)
- `main_test.go` — usage assertion for the day overlay. (Modify)
- `README.md` — new layout + keybindings. (Modify)
- `go.mod` / `go.sum` — promote `github.com/charmbracelet/x/ansi` to a direct dependency. (Modify)

---

### Task 1: Terminal dimensions in the model

**Files:**
- Modify: `model.go`
- Test: `model_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `Model.Width int`, `Model.Height int`; constants `defaultWidth = 80`, `defaultHeight = 24`, `minWidth = 80`, `minHeight = 24`; methods `(*Model) dims() (int, int)` and `(*Model) tooSmall() bool`.

- [ ] **Step 1: Write the failing tests**

Add to `model_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run 'TestWindowSizeMsgStoresDimensions|TestDimsFallbackAndTooSmall'`
Expected: FAIL — `m.dims`/`m.tooSmall` undefined and `WindowSizeMsg` unhandled.

- [ ] **Step 3: Implement sizing in `model.go`**

Add the constants near the top of `model.go`:

```go
const (
	defaultWidth  = 80
	defaultHeight = 24
	minWidth      = 80
	minHeight     = 24
)
```

Add `Width int` and `Height int` to `Model` (with comments: "terminal columns/rows; 0 until the first WindowSizeMsg"). Add:

```go
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
```

Handle the message in `Update` (alongside the existing cases):

```go
case tea.WindowSizeMsg:
	m.Width = msg.Width
	m.Height = msg.Height
	return m, nil
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./... -run 'TestWindowSizeMsgStoresDimensions|TestDimsFallbackAndTooSmall'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add model.go model_test.go
git commit -m "feat: track terminal dimensions in the model"
```

---

### Task 2: Full-screen month grid of equal day boxes

**Files:**
- Modify: `render.go` (rewrite)
- Modify: `styles.go`
- Modify: `go.mod`, `go.sum`
- Test: `render_test.go` (rewrite)

**Interfaces:**
- Consumes: `Model.dims()`, `Model.tooSmall()`, `GridWeeks`, `GridStart`, `DayInMonth`, `DateKey`, `weekdayNames`, `truncateToWidth`, `Theme`, `github.com/charmbracelet/x/ansi`.
- Produces: `Render(m Model, theme Theme) string` (month view only for now); `boxPreviewLines(evs []Event, max, width int) []string`; `renderDayBox(cl cell, innerW, innerH int, theme Theme) string`; `padToWidth(s string, w int) string`; `tooSmallScreen(w, h int, theme Theme) string`; `Theme.DayBox`, `Theme.DayBoxFocus`, `Theme.DayBoxToday`.

**Layout contract (implement exactly):**

Given `W, H := m.dims()` and `weeks := GridWeeks(m.Month, m.WeekStart)`:
- If `m.tooSmall()`, return `tooSmallScreen(W, H, theme)`.
- Vertical budget: `gridH := H - 2 /*title+header*/ - 1 /*blank*/ - 1 /*help*/`, minus 1 more when `m.Status != ""`. Then `boxH := gridH / weeks` (outer height, borders included); `leftoverV := gridH - boxH*weeks` (append this many blank lines below the grid).
- Horizontal budget: `boxW := W / 7` (outer width); `gridW := boxW * 7`; `leftPad := (W - gridW) / 2`.
- `innerW := boxW - 2`, `innerH := boxH - 2` (borders). At the enforced minimum, `boxH >= 3` and `boxW >= 11`, so both are positive.
- Output lines, in order: centered title, weekday header (each name centered in `boxW`, prefixed by `leftPad` spaces), `weeks` rows of 7 horizontally-joined boxes, the `leftoverV` blank lines, a blank separator, the help line, and an optional status line. Join with `"\n"` and emit **no trailing newline**, so the result is exactly `H` lines. **Every emitted line must be exactly `W` display cells wide**, via `padToWidth`; this makes the grid fill the terminal and keeps wide-character rows aligned. The help line text is: `j/k move · h/l months · Enter open day · q quit · <now formatted "Mon 2006-01-02 15:04">`.
- `padToWidth(s string, w int) string` truncates `s` ANSI-aware to `w` cells (`ansi.Truncate`) and then right-pads with spaces until `ansi.StringWidth(s) == w`.

Cell content (`innerH` lines): line 0 is the right-aligned day number (`dayPadding(innerW, cl.day)`, blanks for out-of-month); the remaining `innerH-1` lines are the cell's precomputed `previews` (from `boxPreviewLines(evs, innerH-1, innerW)`), blank-padded. Build each cell's previews in the grid walk (the `cell` struct gains `previews []string`). Box style: `theme.DayBoxFocus` if `cl.focus`, else `theme.DayBoxToday` if `cl.today`, else `theme.DayBox`, each chained with `.Width(innerW).Height(innerH)` then `.Render(strings.Join(lines, "\n"))`.

`boxPreviewLines` returns `nil` when `max <= 0` or there are no events; otherwise, if the count fits, one `truncateToWidth("• "+title, width)` per event, else `max-1` such lines followed by `truncateToWidth(fmt.Sprintf("+%d more", len(evs)-(max-1)), width)`.

- [ ] **Step 1: Write the failing tests**

Replace the layout-dependent tests in `render_test.go` with the following. (At 100 columns `innerW` is 12, so the note titles below stay short enough not to be ellipsized.)

```go
func sizedModel(t *testing.T, start, now string, w, h int) Model {
	m, _ := fixedModel(t, start, now)
	m.Width, m.Height = w, h
	return m
}

func TestRenderContainsTitleAndDayNotes(t *testing.T) {
	m := sizedModel(t, "2025-01-10", "2025-01-10", 100, 40)
	m.Events = map[string][]Event{
		"2025-01-04": {{Title: "Standup"}},
		"2025-01-06": {{Title: "Lunch"}},
	}
	out := Render(m, PlainTheme)
	for _, want := range []string{"January 2025", "Standup", "Lunch"} {
		if !strings.Contains(out, want) {
			t.Errorf("Render output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderMonthFillsWidthAndHeight(t *testing.T) {
	const w, h = 100, 40
	m := sizedModel(t, "2025-01-10", "2025-01-10", w, h)
	out := Render(m, PlainTheme)
	if !strings.Contains(out, "January 2025") {
		t.Fatalf("missing month title:\n%s", out)
	}
	if !strings.Contains(out, "┌") || !strings.Contains(out, "│") {
		t.Fatalf("month grid has no box borders:\n%s", out)
	}
	ws := widths(out)
	if len(ws) != h {
		t.Fatalf("rendered %d lines, want %d (fill height)", len(ws), h)
	}
	for i, lw := range ws {
		if lw != w {
			t.Errorf("line %d width %d, want %d (fill width)", i, lw, w)
		}
	}
}

func TestRenderBoxesEqualSizeRegardlessOfNotes(t *testing.T) {
	empty := sizedModel(t, "2025-01-10", "2025-01-10", 100, 40)
	busy := sizedModel(t, "2025-01-10", "2025-01-10", 100, 40)
	busy.Events = map[string][]Event{"2025-01-10": {
		{Title: "a"}, {Title: "b"}, {Title: "c"}, {Title: "d"},
		{Title: "e"}, {Title: "f"}, {Title: "g"}, {Title: "h"},
	}}
	we := widths(Render(empty, PlainTheme))
	wb := widths(Render(busy, PlainTheme))
	if len(we) != len(wb) {
		t.Fatalf("line count differs: empty=%d busy=%d", len(we), len(wb))
	}
	for i := range we {
		if we[i] != wb[i] {
			t.Fatalf("line %d width empty=%d busy=%d", i, we[i], wb[i])
		}
	}
}

func widths(s string) []int {
	lines := strings.Split(s, "\n")
	out := make([]int, len(lines))
	for i, ln := range lines {
		out[i] = runewidth.StringWidth(ln)
	}
	return out
}

func TestRenderSixWeekMonthAtMinimumSize(t *testing.T) {
	// March 2025 spans 6 week rows on a Sunday-first grid.
	m := sizedModel(t, "2025-03-15", "2025-03-15", minWidth, minHeight)
	if GridWeeks(m.Month, m.WeekStart) != 6 {
		t.Fatalf("March 2025 should need 6 weeks, got %d", GridWeeks(m.Month, m.WeekStart))
	}
	ws := widths(Render(m, PlainTheme))
	if len(ws) != minHeight {
		t.Fatalf("rendered %d lines at min size, want %d", len(ws), minHeight)
	}
	for i, lw := range ws {
		if lw != minWidth {
			t.Errorf("line %d width %d, want %d", i, lw, minWidth)
		}
	}
}

func TestRenderWideCharactersKeepRowsEqual(t *testing.T) {
	m := sizedModel(t, "2025-01-10", "2025-01-10", 100, 40)
	m.Events = map[string][]Event{
		"2025-01-10": {{Title: "中文标题"}},
		"2025-01-12": {{Title: "plain"}},
	}
	ws := widths(Render(m, PlainTheme))
	for i := range ws {
		if ws[i] != ws[0] {
			t.Fatalf("line %d width %d != %d", i, ws[i], ws[0])
		}
	}
}

func TestRenderTooSmallShowsMessage(t *testing.T) {
	m := sizedModel(t, "2025-01-10", "2025-01-10", 40, 10)
	out := Render(m, PlainTheme)
	if !strings.Contains(strings.ToLower(out), "too small") {
		t.Fatalf("expected a too-small message:\n%s", out)
	}
}
```

Delete the now-obsolete tests `TestRenderGridRowsHaveEqualWidth`, `TestRenderHeaderAlignsWithDayColumns`, `TestRenderLeadingBlanksForOffsetMonth`, `TestRenderPromptLineShowsInputCursor`, `TestRenderCapsEventPreviewsWithMoreIndicator`, `TestRenderEqualDisplayWidthWithWideCharacters`, and `TestRenderCapsColumnWidthForLongTitles`. Import `github.com/mattn/go-runewidth` (already a direct dependency).

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run TestRender`
Expected: FAIL — the old renderer has no box borders / no too-small message.

- [ ] **Step 3: Rewrite `render.go` for the box grid**

Keep `Render` as the public entry point (it will dispatch in Task 4). Reuse `weekdayNames`, `pad`, `dayPadding`, `truncateToWidth` from the current file. Remove `maxEventLines`, `colWidthCap`, and `previewLines`. Implement `renderMonth` (the contract above), `boxPreviewLines`, `renderDayBox`, `padToWidth`, and `tooSmallScreen` (centered via `lipgloss.Place(W, H, lipgloss.Center, lipgloss.Center, theme.Help.Render(fmt.Sprintf("Terminal too small (%d×%d); need at least %d×%d", w, h, minWidth, minHeight)))`). The `cell` struct keeps `day`, `today`, `focus`, and gains `previews []string` (computed with `boxPreviewLines(m.Events[key], innerH-1, innerW)` during the grid walk).

- [ ] **Step 4: Add the box styles to `styles.go`**

Add to `Theme`: `DayBox`, `DayBoxFocus`, `DayBoxToday lipgloss.Style`. In `DefaultTheme`:

```go
DayBox:      lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")),
DayBoxFocus: lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("212")).Bold(true),
DayBoxToday: lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("170")),
```

Replace the `var PlainTheme = Theme{}` line with an explicit value that gives every theme slot a border-free or normal-border style and no colors, e.g.:

```go
var PlainTheme = Theme{
	DayBox:      lipgloss.NewStyle().Border(lipgloss.NormalBorder()),
	DayBoxFocus: lipgloss.NewStyle().Border(lipgloss.NormalBorder()),
	DayBoxToday: lipgloss.NewStyle().Border(lipgloss.NormalBorder()),
}
```

(The title/header/event/help slots stay zero-valued and produce no ANSI.)

- [ ] **Step 5: Run the tests to verify they pass**

Run:
```bash
go mod tidy
go test ./...
```

Expected: `go.mod` lists `github.com/charmbracelet/x/ansi v0.10.1` in the direct `require` block (padding uses `ansi.StringWidth`/`ansi.Truncate`); tests PASS. (Model prompt tests still pass because `Model.Prompting` and `Update` are untouched in this task.)

- [ ] **Step 6: Commit**

```bash
git add render.go render_test.go styles.go go.mod go.sum
git commit -m "feat: render the month as a full-screen wall grid of equal boxes"
```

---

### Task 3: Day-overlay state machine and key routing

**Files:**
- Modify: `model.go`
- Test: `model_test.go`

**Interfaces:**
- Consumes: `Model.dims()`, `AddEvent`, `DateKey`, `Persist`, `printableRunes`.
- Produces: `type ViewMode int` with `ViewMonth` (zero value) and `ViewDay`; fields `Model.View ViewMode`, `Model.NoteSel int`, `Model.Editing bool`; methods `(*Model) openDay(time.Time)`, `(*Model) closeDay()`, `(*Model) dayEvents() []Event`, `(*Model) moveNoteSel(int)`, `(*Model) clampNoteSel()`, `(*Model) DeleteEvent(time.Time, int) bool`, `(*Model) commitNote() tea.Cmd`, `(*Model) deleteSelectedNote() tea.Cmd`, `(*Model) handleMonthKey(tea.KeyMsg) tea.Cmd`, `(*Model) handleDayKey(tea.KeyMsg) tea.Cmd`, `(*Model) handleAddKey(tea.KeyMsg) tea.Cmd`. The `Prompting` field is removed.

**Key contract (implement exactly):**
- `handleKey` routes to `handleDayKey` when `m.View == ViewDay`, else `handleMonthKey`.
- Month mode: `Ctrl-C`→quit; `Enter`→`openDay(m.Focus)`; `Up`/`k`→`moveFocus(-1)`; `Down`/`j`→`moveFocus(1)`; `Left`/`h`→`shiftMonth(-1)`; `Right`/`l`→`shiftMonth(1)`; `q`→quit.
- Add mode (`m.Editing`): `Ctrl-C`→quit; `Esc`→cancel (`Editing=false`, `Input=""`); `Enter`→`commitNote`; `Backspace` edits; `Space` and printable `Runes` append (so `q` types a literal `q`).
- List mode: `Ctrl-C`→quit; `Esc`→`closeDay`; `Up`/`k`→`moveNoteSel(-1)`; `Down`/`j`→`moveNoteSel(1)`; `Enter` or `a`→start add (`Editing=true`, `Input=""`); `Delete`, `d`, or `x`→`deleteSelectedNote`; `q`→quit.
- `commitNote`: trim `Input`; clear `Editing`/`Input`; if blank return `nil`; else `AddEvent(title, m.Focus)`, set `NoteSel` to the last index, return `NewCalendar(m.Events).Persist(m.StoragePath)`.
- `deleteSelectedNote`: no-op (`nil`) when the index is out of range; else `DeleteEvent(m.Focus, m.NoteSel)`, `clampNoteSel()`, return a persist command.
- `moveNoteSel`: clamp to `[0, len(dayEvents())-1]`, or `0` when empty.
- `DeleteEvent(day, idx)`: remove the element; delete the map key when the list becomes empty; return whether anything was removed.

- [ ] **Step 1: Write the failing tests**

Replace the prompt-related tests in `model_test.go` (`TestEnterPromptSubmitPersists`, `TestEnterSubmitBlankDoesNotPersistOrCreateEvent`, `TestEscapeCancelsPromptDiscardingInput`, `TestBackspaceEditsInput`, `TestQQuitsNormallyButTypesWhilePrompting`, `TestSpacebarTypesIntoInput`) with:

```go
func TestEnterOpensDayOverlayAndEscCloses(t *testing.T) {
	m, _ := newTestModel(t, "2025-01-10")
	if _, cmd := m.Update(keyType(tea.KeyEnter)); cmd != nil || m.View != ViewDay {
		t.Fatalf("Enter: View=%v cmd=%v, want ViewDay/nil", m.View, cmd)
	}
	if m.Editing || m.NoteSel != 0 {
		t.Fatalf("opened overlay Editing=%v NoteSel=%d, want false/0", m.Editing, m.NoteSel)
	}
	m.Update(keyType(tea.KeyEscape))
	if m.View != ViewMonth {
		t.Fatalf("Esc left View=%v, want ViewMonth", m.View)
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
	if m.Editing || m.Input != "" || m.View != ViewDay {
		t.Fatalf("commit: Editing=%v Input=%q View=%v", m.Editing, m.Input, m.View)
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
	if m.View != ViewDay {
		t.Fatalf("Esc in add mode must stay in the overlay, View=%v", m.View)
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
```

Keep `TestCtrlCQuits`, `TestCursorBlinksOnToggleMsgOnly`, and `TestSaveErrorSetsStatusAndRenderShowsIt`. Update `TestSaveErrorSetsStatusAndRenderShowsIt` only if it references removed fields (it does not).

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run 'TestEnterOpensDayOverlayAndEscCloses|TestOverlay|TestQQuitsInMonthAndListButTypesWhileAdding|TestArrowKeysNavigate'`
Expected: FAIL — `View`/`Editing`/`NoteSel` undefined, `Prompting` still present.

- [ ] **Step 3: Implement the overlay state machine in `model.go`**

Add the `ViewMode` type and constants, the three fields, and the methods described in the Interfaces/Key contract. Rewrite `handleKey` to route by `m.View`; split the old `handlePromptKey` into `handleAddKey` (fixed to append printable runes as text) and add `handleMonthKey`/`handleDayKey`. Delete `submit` and the `Prompting` field. Keep `blinkCmd`/`cursorToggleMsg` and the `savedMsg`/`errMsg` handling unchanged.

- [ ] **Step 4: Run the full suite**

Run: `go test ./...`
Expected: PASS. (The renderer still draws only the month grid, so `ViewDay` renders the calendar for now; that is completed in Task 4.)

- [ ] **Step 5: Commit**

```bash
git add model.go model_test.go
git commit -m "feat: add day-overlay state, selection, add/delete and key routing"
```

---

### Task 4: Day-overlay renderer composited over the dimmed calendar

**Files:**
- Modify: `render.go`
- Modify: `styles.go`
- Test: `render_test.go`

**Interfaces:**
- Consumes: `Model.View`, `Model.Editing`, `Model.NoteSel`, `Model.Input`, `Model.CursorOn`, `Model.dayEvents()`, `Model.dims()`, `renderMonth`, `Theme`, `github.com/charmbracelet/x/ansi`.
- Produces: `Render` dispatches on `m.View`; `renderDay(m Model, theme Theme) string`; `renderPanel(m Model, theme Theme, panelW, panelH int) string`; `composeOverlay(bg, panel string, w, h int, theme Theme) string`; `Theme.Overlay`, `Theme.Dim`, `Theme.Cursor`.

**Panel contract:**
- `panelW := min(W-4, 64)` and `panelH := min(H-2, 18)`; if that drops below `20×7`, clamp to `W×H`.
- `innerW := panelW-2`, `innerH := panelH-2`. Lines: 0 = the date title `theme.Title.Render(m.Focus.Format("Monday 2006-01-02"))` centered in `innerW`; 1 = blank; then the body; then the footer. List-mode footer is 1 line (`j/k select · a add · d delete · Esc close` rendered with `theme.Help`); add-mode footer is 2 lines (`New note: ` + `m.Input` + (`theme.Cursor.Render("▌")` when `CursorOn`) and `Enter save · Esc cancel`). Streams are truncated with `truncateToWidth` to `innerW`.
- Body height `bodyH := innerH - 2 - footerH` (min 1). Notes are `m.dayEvents()`. If empty, the body shows `(no notes)`. Otherwise the visible window is `off := 0; if m.NoteSel >= bodyH { off = m.NoteSel - bodyH + 1 }`, each visible note rendered as `"> "`/`"  "` + truncated title, padded with blank lines to `bodyH`.
- Apply `theme.Overlay.Width(innerW).Height(innerH).Render(strings.Join(lines, "\n"))`.

**Composite contract:**
- `bg := renderMonth(...)`; take its lines, pad each to visible width `W` (append spaces computed from `ansi.StringWidth`), apply `theme.Dim.Render` to each, and pad the slice to exactly `H` lines.
- `panelLines := strings.Split(panel, "\n")`; `left := (W-panelW)/2`, `top := (H-panelH)/2`.
- For row `i` in `[top, top+panelH)`: `ansi.Cut(bg[i], 0, left) + panelLines[i-top] + ansi.Cut(bg[i], left+panelW, W)`. Other rows are `bg[i]`. Join with `"\n"`.
- When `m.tooSmall()`, `renderDay` returns `tooSmallScreen(W, H, theme)` (never composite).

- [ ] **Step 1: Write the failing tests**

Add to `render_test.go`:

```go
func TestRenderOverlayShowsNotesAndSelection(t *testing.T) {
	m := sizedModel(t, "2025-01-10", "2025-01-10", 100, 40)
	m.Events = map[string][]Event{"2025-01-10": {{Title: "Party"}, {Title: "Gym"}}}
	m.View = ViewDay
	m.NoteSel = 1
	out := Render(m, PlainTheme)
	for _, want := range []string{"2025-01-10", "Party", "Gym", "> Gym"} {
		if !strings.Contains(out, want) {
			t.Errorf("overlay missing %q:\n%s", want, out)
		}
	}
}

func TestRenderOverlayEmptyShowsPlaceholder(t *testing.T) {
	m := sizedModel(t, "2025-01-10", "2025-01-10", 100, 40)
	m.View = ViewDay
	if out := Render(m, PlainTheme); !strings.Contains(out, "(no notes)") {
		t.Fatalf("empty overlay missing placeholder:\n%s", out)
	}
}

func TestRenderOverlayAddModeShowsInput(t *testing.T) {
	m := sizedModel(t, "2025-01-10", "2025-01-10", 100, 40)
	m.View = ViewDay
	m.Editing = true
	m.Input = "Call"
	m.CursorOn = true
	out := Render(m, PlainTheme)
	if !strings.Contains(out, "New note") || !strings.Contains(out, "Call") {
		t.Fatalf("add-mode overlay missing input:\n%s", out)
	}
}

func TestRenderOverlayLinesStayWithinPanelWidth(t *testing.T) {
	m := sizedModel(t, "2025-01-10", "2025-01-10", 100, 40)
	m.View = ViewDay
	m.Events = map[string][]Event{"2025-01-10": {{Title: strings.Repeat("wide-title-", 30)}}}
	out := Render(m, PlainTheme)
	for _, ln := range strings.Split(out, "\n") {
		if w := runewidth.StringWidth(ln); w != 100 {
			t.Errorf("line width %d, want 100 (overlay must not overflow): %q", w, ln)
		}
	}
	if !strings.Contains(out, "…") {
		t.Fatalf("over-long note title should be ellipsized:\n%s", out)
	}
}

func TestRenderOverlaySelectionScrollsIntoView(t *testing.T) {
	m := sizedModel(t, "2025-01-10", "2025-01-10", 100, 40)
	m.View = ViewDay
	evs := make([]Event, 30)
	for i := range evs {
		evs[i] = Event{Title: fmt.Sprintf("note-%02d", i)}
	}
	m.Events = map[string][]Event{"2025-01-10": evs}
	m.NoteSel = 29
	out := Render(m, PlainTheme)
	if !strings.Contains(out, "> note-29") {
		t.Fatalf("selected note not scrolled into view:\n%s", out)
	}
}

func TestRenderOverlayResizeDoesNotPanicAndFallsBack(t *testing.T) {
	m := sizedModel(t, "2025-01-10", "2025-01-10", 120, 50)
	m.View = ViewDay
	_ = Render(m, PlainTheme)
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	if out := Render(m, PlainTheme); !strings.Contains(strings.ToLower(out), "too small") {
		t.Fatalf("shrunk overlay should show too-small message:\n%s", out)
	}
}
```

Add `"fmt"` and `tea "github.com/charmbracelet/bubbletea"` to `render_test.go` imports if absent.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./... -run 'TestRenderOverlay'`
Expected: FAIL — `View` is ignored, no panel is drawn.

- [ ] **Step 3: Implement the overlay renderer**

Add `Overlay`, `Dim`, and `Cursor` to `Theme` in `styles.go`. `DefaultTheme.Overlay = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("212"))` (no padding, so the panel is exactly `innerW`×`innerH` inside its border); `DefaultTheme.Dim = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))`; `DefaultTheme.Cursor = lipgloss.NewStyle().Reverse(true)`. `PlainTheme.Overlay` gets the same border with no color; `PlainTheme.Dim` and `PlainTheme.Cursor` stay zero-valued.

In `render.go`, rename the Task 2 body to `renderMonth`, make `Render` dispatch on `m.View`, and implement `renderDay`, `renderPanel`, and `composeOverlay` per the contracts above. Import `github.com/charmbracelet/x/ansi`.

- [ ] **Step 4: Run the suite**

Run: `go test ./...`
Expected: all tests PASS. (The `x/ansi` dependency was already promoted to direct in Task 2.)

- [ ] **Step 5: Commit**

```bash
git add render.go render_test.go styles.go
git commit -m "feat: render the day overlay composited over a dimmed calendar"
```

---

### Task 5: Update keybinding help, usage text, and README

**Files:**
- Modify: `main.go`
- Modify: `main_test.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: nothing new.
- Produces: updated `printUsage` text and README.

- [ ] **Step 1: Write the failing test**

Extend `TestRunHelpPrintsUsageWithoutLaunchingTUI` in `main_test.go` to require the new keybinding text:

```go
	for _, want := range []string{"Usage", "--week-start", "--storage", "j / k", "overlay"} {
		if !strings.Contains(out, want) {
			t.Errorf("usage output missing %q:\n%s", want, out)
		}
	}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./... -run TestRunHelpPrintsUsageWithoutLaunchingTUI`
Expected: FAIL — usage output has no "overlay".

- [ ] **Step 3: Update `printUsage` in `main.go` and `README.md`**

In `printUsage`, replace the `Keys:` block with text that still contains the `j / k` substring (the existing assertion depends on it), e.g.:

```
Keys (calendar):
  j / k, ↑/↓   move focus down / up one day
  h / l, ←/→   previous / next month
  Enter        open the highlighted day (overlay)
  q            quit
Keys (day overlay):
  j / k, ↑/↓   move between notes
  a / Enter    add a note (Enter saves, Esc cancels)
  d / x        delete the selected note
  Esc          back to the calendar
Minimum terminal size: 80x24.
```

Update `README.md` to describe the full-screen wall grid, the day overlay, the minimum size, and both keybinding sets.

- [ ] **Step 4: Run the full suite**

Run: `go test ./... && go vet ./... && go build ./...`
Expected: PASS, no vet findings, clean build.

- [ ] **Step 5: Commit**

```bash
git add main.go main_test.go README.md
git commit -m "docs: update keybindings and usage for the wall calendar and overlay"
```

---

## Self-Review Notes

- **Spec coverage:** full-screen equal boxes (Task 2), `Enter` overlay (Tasks 3–4), add/remove notes (Tasks 3–4), immediate persistence (Task 3), min-size lock + message (Tasks 1–2, 4), structured list editing (Tasks 3–4), README/keybindings (Task 5), scrolling note lists (Tasks 3–4), dimmed composited modal (Task 4).
- **Type consistency:** `ViewMode`/`ViewMonth`/`ViewDay`, `NoteSel`, `Editing`, `dims()`, `tooSmall()`, `dayEvents()`, `moveNoteSel()`, `clampNoteSel()`, `DeleteEvent()`, `commitNote()`, `deleteSelectedNote()`, `boxPreviewLines()`, `renderMonth()`, `renderDay()`, `renderPanel()`, `composeOverlay()`, `tooSmallScreen()` are each defined once and referenced by the same name later.
- **Task ordering:** each task compiles and keeps the existing suite green; Task 3 removes `Prompting` only after Task 2 removed its renderer use.
