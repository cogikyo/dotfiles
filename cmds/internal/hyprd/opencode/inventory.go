package opencode

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"dotfiles/cmds/internal/hyprd/kitty"
)

type Snapshot struct {
	JobID     string    `json:"job_id"`
	Mode      Mode      `json:"mode"`
	CreatedAt time.Time `json:"created_at"`
	Panes     []Pane    `json:"panes"`
}

type Pane struct {
	KittyPaneID int       `json:"kitty_pane_id"`
	KittyPID    int       `json:"kitty_pid"`
	TUIPID      int       `json:"tui_pid"`
	Directory   string    `json:"directory"`
	SessionID   string    `json:"session_id,omitempty"`
	Started     time.Time `json:"started"`
	Shell       bool      `json:"shell"`
	Shown       *bool     `json:"shown"`
	Hidden      time.Time `json:"hidden_since,omitzero"`
	Report      *Report   `json:"report"`
}

func takeSnapshot(views *kitty.Tracker, jobID string, mode Mode) (Snapshot, []string, error) {
	panes, seen, err := views.View()
	if panes == nil {
		return Snapshot{}, nil, fmt.Errorf("kitty inventory: %w", err)
	}
	var warnings []string
	if err != nil {
		warnings = append(warnings, "kitty view incomplete: "+err.Error())
	}

	snapshot := Snapshot{JobID: jobID, Mode: mode, CreatedAt: time.Now()}
	for _, located := range panes {
		pane, found, paneWarnings := inspect(located, seen)
		warnings = append(warnings, paneWarnings...)
		if found {
			snapshot.Panes = append(snapshot.Panes, pane)
		}
	}

	sort.Slice(snapshot.Panes, func(i, j int) bool {
		if snapshot.Panes[i].KittyPID != snapshot.Panes[j].KittyPID {
			return snapshot.Panes[i].KittyPID < snapshot.Panes[j].KittyPID
		}
		return snapshot.Panes[i].KittyPaneID < snapshot.Panes[j].KittyPaneID
	})

	if err := persistSnapshot(snapshot); err != nil {
		return Snapshot{}, nil, err
	}
	return snapshot, warnings, nil
}

func inspect(located kitty.LocatedPane, seen map[kitty.PaneKey]kitty.Seen) (Pane, bool, []string) {
	tui, found := tuiProcess(located.Pane)
	if !found {
		return Pane{}, false, nil
	}

	var warnings []string
	directory := tui.CWD
	if directory == "" {
		directory = located.Pane.CWD
	}
	started, err := processStarted(tui.PID)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("TUI %d start time unavailable: %v", tui.PID, err))
	}
	report, err := readReport(tui.PID)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("TUI %d report unavailable: %v", tui.PID, err))
	}

	pane := Pane{
		KittyPaneID: located.Pane.ID,
		KittyPID:    located.KittyPID,
		TUIPID:      tui.PID,
		Directory:   cleanDirectory(directory),
		SessionID:   sessionID(tui, report),
		Started:     started,
		Shell:       loginZshCommand(located.Pane.Cmdline),
		Report:      report,
	}
	if view, known := seen[located.Key()]; known {
		pane.Shown = &view.Shown
		pane.Hidden = view.Hidden
	}
	return pane, true, warnings
}

func current(views *kitty.Tracker, target Pane) (Pane, string) {
	panes, seen, err := views.View()
	for _, located := range panes {
		if located.Key() != target.key() {
			continue
		}
		pane, found, _ := inspect(located, seen)
		if !found || pane.TUIPID != target.TUIPID {
			return Pane{}, "foreground changed"
		}
		return pane, ""
	}
	if err != nil {
		return Pane{}, "foreground unknown"
	}
	return Pane{}, "pane closed"
}

func tuiProcess(pane kitty.Pane) (kitty.Process, bool) {
	for _, process := range pane.ForegroundProcesses {
		if process.PID > 0 && process.PID != pane.PID && isTUICommand(process.Cmdline) {
			return process, true
		}
	}
	return kitty.Process{}, false
}

func foregroundTUI(pane kitty.Pane) (kitty.Process, bool) {
	i := slices.IndexFunc(pane.ForegroundProcesses, func(p kitty.Process) bool {
		return p.PID > 0 && isTUICommand(p.Cmdline)
	})
	if i < 0 {
		return kitty.Process{}, false
	}
	return pane.ForegroundProcesses[i], true
}

func sessionID(tui kitty.Process, report *Report) string {
	id := option(tui.Cmdline, "--session", "-s")
	if report != nil {
		id = ""
		if report.Session != nil {
			id = *report.Session
		}
	}
	if !ValidSessionID(id) {
		return ""
	}
	return id
}

var (
	subcommands = []string{
		"upgrade", "update", "uninstall", "acp", "api", "debug", "auth", "mcp", "plugin",
		"models", "stats", "mini", "run", "session", "service", "reload", "pair", "serve",
	}
	valueFlags = []string{"--server", "--session", "-s", "--prompt", "--log-level", "--completions"}
)

func isTUICommand(args []string) bool {
	if len(args) == 0 || filepath.Base(args[0]) != "opencode" {
		return false
	}
	positionals := 0
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			if slices.Contains(valueFlags, arg) {
				i++
			}
			continue
		}
		positionals++
		if positionals > 1 || slices.Contains(subcommands, arg) {
			return false
		}
	}
	return true
}

func option(args []string, names ...string) string {
	for i, arg := range args {
		for _, name := range names {
			if arg == name && i+1 < len(args) {
				return strings.TrimSpace(args[i+1])
			}
			if value, ok := strings.CutPrefix(arg, name+"="); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func ValidSessionID(id string) bool {
	if !strings.HasPrefix(id, "ses_") || len(id) == len("ses_") {
		return false
	}
	for _, r := range id[len("ses_"):] {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func readReport(pid int) (*Report, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return nil, fmt.Errorf("XDG_RUNTIME_DIR is unset")
	}
	path := filepath.Join(runtimeDir, "opencode", "tui", strconv.Itoa(pid)+".json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	fields, err := processStat(pid)
	if err != nil {
		return nil, err
	}
	if report.Start != fields[19] {
		return nil, nil
	}
	return &report, nil
}

func processStarted(pid int) (time.Time, error) {
	fields, err := processStat(pid)
	if err != nil {
		return time.Time{}, err
	}
	ticks, err := strconv.ParseInt(fields[19], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse /proc/%d/stat start: %w", pid, err)
	}
	boot, err := bootTime()
	if err != nil {
		return time.Time{}, err
	}
	return boot.Add(time.Duration(ticks) * (time.Second / clockTicks)), nil
}

const clockTicks = 100

func bootTime() (time.Time, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}, err
	}
	for line := range strings.Lines(string(data)) {
		if value, ok := strings.CutPrefix(line, "btime "); ok {
			seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil {
				return time.Time{}, fmt.Errorf("parse /proc/stat btime: %w", err)
			}
			return time.Unix(seconds, 0), nil
		}
	}
	return time.Time{}, fmt.Errorf("/proc/stat has no btime")
}

func processStat(pid int) ([]string, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return nil, err
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return nil, fmt.Errorf("unexpected /proc/%d/stat", pid)
	}
	fields := strings.Fields(string(data)[end+1:])
	if len(fields) < 20 {
		return nil, fmt.Errorf("unexpected /proc/%d/stat", pid)
	}
	return fields, nil
}

func persistSnapshot(snapshot Snapshot) error {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return fmt.Errorf("XDG_RUNTIME_DIR is unset")
	}
	directory := filepath.Join(runtimeDir, "hyprd", "opencode")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return fmt.Errorf("secure snapshot directory: %w", err)
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	path := filepath.Join(directory, "refresh-snapshot.json")
	temporary := path + ".new"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publish snapshot: %w", err)
	}
	return nil
}

func cleanDirectory(directory string) string {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return ""
	}
	return filepath.Clean(directory)
}
