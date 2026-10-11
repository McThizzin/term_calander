# Design: full-screen wall calendar + day overlay

Date: 2026-10-10
Status: awaiting approval

## Intent and why

The current TUI draws the month as a loose text grid pinned to the top-left of
the terminal. Day numbers are packed together, and once a day has notes its
column widens and the rows go ragged. The user wants a wall-calendar feel:

1. The grid fills the terminal, with each day in its own equal-size bordered
   box so busy and empty days look identical in size.
2. Pressing `Enter` on the highlighted day opens a larger single-day overlay
   with room to read the notes and to add or remove them.
3. The overlay's editing is a **structured note list** (each note an item;
   add a new one, delete the selected one) — chosen by the user over a
   free-form notepad.

## Goals

- Grid occupies the full terminal and stays visually even for any mix of
  empty and crowded days.
- A day overlay (modal-style) shows one day's notes and supports add, remove,
  and return-to-calendar.
- Notes persist immediately on add/remove, reusing the existing storage layer.
- Graceful behavior when the terminal is below a minimum size.

## Non-goals

- No mouse interaction.
- No editing the text of an existing note (add/remove only; remove then re-add
  to change one).
- No times UI on notes (the `Event.Time` field stays in the data model but is
  not shown or edited).
- No week/agenda views, no holidays/weekend coloring, no color-profile flag.
- No change to the on-disk JSON schema or date semantics (local calendar
  dates).

## Anti-goals (what would make this a failure even if it "works")

- Boxes whose width/height change when a day gains or loses notes — the whole
  point is that they never move.
- Losing notes, double-writing, or corrupting the storage file during
  add/remove.
- Crashing, hanging, or drawing garbage on a tiny terminal.
- Input state leaking between the calendar and the overlay (typing a note
  title moving the calendar focus, etc.).

## Constraints and existing context

- Go 1.24 module `term-calendar`; Bubble Tea v1.3.7, Lipgloss v1.1.0,
  go-runewidth already a direct dependency.
- Existing packages/types are reused: `Model`, `Update`, `Render`, `Event`,
  `Calendar`, `Load`, `Persist`, `DateKey`, `GridStart`, `GridWeeks`,
  `DayInMonth`, `DaysInMonth`.
- Key matching stays on exported `tea.*` constants (plus rune checks for
  `j/k/h/l/q` and `a/d/x`).
- Storage stays at the configured path; both add and delete trigger the
  existing crash-safe `Persist` command.

## Decisions

**The user decided:**
- Structured note list for overlay editing (option A).
- `Enter` on the highlighted calendar day opens the overlay.
- The overlay offers add-note, remove-note, and a way back to the calendar.

**My calls (builder decisions — flag any you dislike):**
- **Navigation:** `j`/`↓` and `k`/`↑` move the focus by one day within the
  displayed month (clamped, as today); `h`/`←` and `l`/`→` change the month.
  Arrow keys are supported explicitly via `tea.KeyUp`/`KeyDown`/`KeyLeft`/
  `KeyRight` as aliases for `k`/`j`/`h`/`l`; the letter keys keep working.
- **Minimum terminal size:** 80 columns × 24 rows. Below that, the screen
  shows a centered "Terminal too small (need 80×24)" message; `q`/`Ctrl-C`
  still quit. Box previews shrink to zero lines before the app refuses to
  render (at 24 rows with a 6-week month each box has only the day-number
  line — acceptable degradation).
- **Box size:** computed once from the terminal size so every box is
  identical. 7 columns, `R` rows (5 or 6 for the month); each box gets a
  1-cell border. Day number on the first inner line; then as many note
  preview lines as fit; a final `+N more` (ellipsized to fit) when notes
  overflow.
- **Overlay layout:** a true modal composited over the calendar. The calendar
  is rendered first and passed through the theme's dim/faint style, then the
  centered panel is spliced over it by visible cell width (go-runewidth), so
  the day grid stays visible but de-emphasized behind the panel. The panel is
  up to ~64 cols × ~18 rows (capped to terminal minus margin), with a
  full-date title, the notes list (selected item marked and highlighted), and
  a hint footer. When empty it shows "(no notes)".
- **Overlay keys:**
  - List mode: `j`/`↓` `k`/`↑` move the selection; `Enter` or `a` start
    adding a note; `d` (or `x`, or `Delete`) remove the selected note
    (persists immediately); `Esc` close the overlay.
  - Add mode: printable runes and space append; `Backspace` edits; `Enter`
    commits the note (trims; empty is ignored) and persists; `Esc` cancels
    the add without saving. In add mode every printable rune is text — `q`
    types the letter `q` rather than quitting, so a draft is never lost.
  - `q` quits the program (except in add mode); `Ctrl-C` quits from
    anywhere, including add mode.
- **Long note lists scroll.** When a day has more notes than the overlay body
  can show, the list scrolls so every note stays reachable and deletable. The
  visible window follows the selection (the model tracks the selection index;
  the renderer derives the scroll offset so the selected row is always
  visible).
- `Enter` in the calendar no longer starts an inline input; it opens the
  overlay. The old calendar-view inline prompt is removed.
- A successful save clears any error status; a failed save shows the existing
  footer warning.
- **Persistence semantics:** add and delete both reuse the existing
  `Calendar.Persist` unchanged. `saveCalendar` already takes a process-wide
  mutex and `Persist` snapshots the event map at dispatch time, so writes
  never corrupt the file or race each other. We are not adding save
  sequencing; the only residual effect of extremely rapid edits is that a
  just-deleted note could briefly reappear if an earlier, slower save lands
  last — benign, self-correcting on the next edit, and no data is lost.

## Layout mockups (ASCII)

Main view (terminal fills the boxes; every box identical):

```
                                  October 2026
   ┌─────────┬─────────┬─────────┬─────────┬─────────┬─────────┬─────────┐
   │ Sun     │ Mon     │ Tue     │ Wed     │ Thu     │ Fri     │ Sat     │
   ├─────────┼─────────┼─────────┼─────────┼─────────┼─────────┼─────────┤
   │         │         │         │      1  │      2  │      3  │      4  │
   │         │         │         │         │         │         │ • Stand…│
   ├─────────┼─────────┼─────────┼─────────┼─────────┼─────────┼─────────┤
   │      5  │      6  │      7  │      8  │      9  │     10  │     11  │
   │         │ • Lunch │         │         │         │ • Party │         │
   │         │ +1 more │         │         │         │ • Gym   │         │
   └─────────┴─────────┴─────────┴─────────┴─────────┴─────────┴─────────┘
   j/k move · h/l months · Enter open day · q quit · Sat 2026-10-10 19:20
```

Day overlay (calendar dimmed behind):

```
                    ┌──────────────────────────────────────────┐
                    │            Friday 2026-10-10             │
                    │                                          │
                    │   > Party                                │
                    │     Gym                                  │
                    │     Dinner                               │
                    │                                          │
                    │   j/k select · a add · d delete · Esc     │
                    └──────────────────────────────────────────┘
```

Overlay while adding:

```
                    ┌──────────────────────────────────────────┐
                    │            Friday 2026-10-10             │
                    │                                          │
                    │     Party                                │
                    │     Gym                                  │
                    │                                          │
                    │   New note: Call├                         │
                    │   Enter save · Esc cancel                │
                    └──────────────────────────────────────────┘
```

## Behavior details / edge cases

- Empty month rows: a month needing 6 week-rows uses 6; a 5-row month uses 5
  — the grid still fills the available height by making each box taller.
- Today and focus: the focused box gets a highlighted border; today gets its
  own accent on the day number (today styling survives even when not focused).
- Long note titles are truncated to the box inner width with `…`; the overlay
  widens to fit and truncates only when it exceeds the panel width.
- Removing the last remaining note leaves the day with an empty list and the
  overlay showing "(no notes)".
- Deleting adjusts the selection index so it stays in range.
- A terminal resize (`tea.WindowSizeMsg`) recomputes box size and re-renders.
- Wide (CJK/emoji) titles keep boxes aligned via go-runewidth.
- Focus is clamped to the displayed month; month change re-anchors the
  focused day-of-month as today.

## Testing plan

- **Model/Update:** `Enter` opens the overlay on the focused day; `Esc`
  closes it; selection moves and clamps; `a`/`Enter` starts add mode; runes and
  space append; `Backspace` edits; `Enter` commits and returns a persist cmd;
  empty commit is ignored; `Esc` cancels add; `d`/`x`/`Delete` deletes the
  selected note and returns a persist cmd; delete keeps selection in range;
  calendar keys don't leak into overlay state and vice-versa;
  `tea.WindowSizeMsg` updates width/height.
- **Render:** boxes have equal dimensions across a month mixing empty days and
  days with 1/4/6 notes; the grid respects a given terminal size; the weekday
  header aligns to box columns; wide titles keep rows equal width; the overlay
  contains the day's note titles and the selection marker; add mode shows the
  input; the min-size message appears below 80×24.
- Existing date/storage/CLI tests stay green.

## What's left to the builder

Concretely: exact lipgloss border style and colors, the precise box dimension
formula and rounding, overlay centering/splicing math, truncation edge cases,
and any helper decomposition in `render.go`. These do not change what gets
built.
```
