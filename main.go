package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// version is a human hint printed at startup.
const version = "0.1.0"

// options holds parsed command-line configuration.
type options struct {
	StoragePath string
	Start       time.Time
	WeekStart   time.Weekday
	Help        bool
}

// parseWeekStart maps the --week-start value to a weekday.
func parseWeekStart(s string) (time.Weekday, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "sun", "sunday":
		return time.Sunday, nil
	case "mon", "monday":
		return time.Monday, nil
	}
	return time.Sunday, fmt.Errorf("invalid --week-start %q: want sun or mon", s)
}

// parseOptions parses CLI arguments into options. It never writes to stdout.
func parseOptions(args []string) (options, error) {
	fs := flag.NewFlagSet("term-calendar", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		storage      = fs.String("storage", "", "path to the calendar JSON file")
		startStr     = fs.String("start", "", "start date, YYYY-MM-DD (default today)")
		today        = fs.Bool("today", false, "start on today's date")
		weekStartStr = fs.String("week-start", "sun", "first day of the week: sun or mon")
		help         = fs.Bool("help", false, "show help and exit")
	)
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}

	ws, err := parseWeekStart(*weekStartStr)
	if err != nil {
		return options{}, err
	}

	todayMidnight := func() time.Time {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	}

	opt := options{StoragePath: *storage, WeekStart: ws, Help: *help, Start: todayMidnight()}
	if *startStr != "" {
		d, err := time.ParseInLocation(dateKeyLayout, *startStr, time.Local)
		if err != nil {
			return options{}, fmt.Errorf("invalid --start %q: want YYYY-MM-DD", *startStr)
		}
		opt.Start = d
	}
	if *today {
		opt.Start = todayMidnight()
	}
	return opt, nil
}

// resolveStoragePath returns the explicit flag value, else the default under
// the user config dir, else a CWD fallback.
func resolveStoragePath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "term-calendar", "calendar.json")
	}
	return ".calendar.json"
}

// printUsage writes the help text to w.
func printUsage(w io.Writer) {
	fmt.Fprintf(w, `term-calendar %s - a terminal month calendar

Usage:
  term-calendar [flags]

Flags:
  --start YYYY-MM-DD   start on this date (default: today)
  --today              start on today's date
  --storage PATH       calendar JSON file (default: <config-dir>/term-calendar/calendar.json)
  --week-start sun|mon first day of the week (default: sun)
  --help               show this help and exit

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
`, version)
}

// loadWarning returns user-facing text when path exists but cannot be read
// (corrupt or unreadable), or an empty string when loading is fine.
func loadWarning(path string) string {
	if _, err := Load(path); err != nil {
		return fmt.Sprintf("could not read %s (%v); starting empty", path, err)
	}
	return ""
}

// run parses args, prints a startup hint, and launches the TUI.
func run(args []string, stdout io.Writer) error {
	opt, err := parseOptions(args)
	if err != nil {
		return err
	}
	if opt.Help {
		printUsage(stdout)
		return nil
	}
	storagePath := resolveStoragePath(opt.StoragePath)
	fmt.Fprintf(stdout, "term-calendar %s: events stored at %s\n", version, storagePath)
	if w := loadWarning(storagePath); w != "" {
		fmt.Fprintln(stdout, "term-calendar: warning: "+w)
	}

	m := NewModelWithWeekStart(storagePath, opt.Start, opt.WeekStart)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "term-calendar:", err)
		os.Exit(1)
	}
}
