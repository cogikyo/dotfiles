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
	KittyPaneID int    `json:"kitty_pane_id"`
	KittyPID    int    `json:"kitty_pid"`
	TUIPID      int    `json:"tui_pid"`
	Directory   string `json:"directory"`
	SessionID   string `json:"session_id,omitempty"`
	Focused     bool   `json:"focused"`
	PromptDirty *bool  `json:"prompt_dirty,omitempty"`
}

type paneKey struct {
	KittyPID int
	PaneID   int
}

func takeSnapshot(jobID string, mode Mode) (Snapshot, []string, error) {
	panes, err := kitty.Inventory()
	if err != nil && panes == nil {
		return Snapshot{}, nil, fmt.Errorf("kitty inventory: %w", err)
	}
	var warnings []string
	if err != nil {
		warnings = append(warnings, "kitty inventory incomplete: "+err.Error())
	}

	snapshot := Snapshot{JobID: jobID, Mode: mode, CreatedAt: time.Now()}
	for _, located := range panes {
		tui, found := tuiProcess(located.Pane)
		if !found {
			continue
		}

		directory := tui.CWD
		if directory == "" {
			directory = located.Pane.CWD
		}

		sessionID, recorded, contextErr := shownSession(tui.PID)
		if contextErr != nil {
			warnings = append(warnings, fmt.Sprintf("TUI %d context unavailable: %v", tui.PID, contextErr))
		}
		if !recorded {
			sessionID = option(tui.Cmdline, "--session", "-s")
		}
		if !validSessionID(sessionID) {
			sessionID = ""
		}

		snapshot.Panes = append(snapshot.Panes, Pane{
			KittyPaneID: located.Pane.ID,
			KittyPID:    located.KittyPID,
			TUIPID:      tui.PID,
			Directory:   cleanDirectory(directory),
			SessionID:   sessionID,
			Focused:     located.Focused,
		})
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

func tuiProcess(pane kitty.Pane) (kitty.Process, bool) {
	for _, process := range pane.ForegroundProcesses {
		if process.PID > 0 && process.PID != pane.PID && isTUICommand(process.Cmdline) {
			return process, true
		}
	}
	return kitty.Process{}, false
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

func validSessionID(id string) bool {
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

func shownSession(pid int) (string, bool, error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return "", false, fmt.Errorf("XDG_RUNTIME_DIR is unset")
	}
	path := filepath.Join(runtimeDir, "opencode", "tui", strconv.Itoa(pid)+".json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	var shown struct {
		Start   string  `json:"start"`
		Session *string `json:"session"`
	}
	if err := json.Unmarshal(data, &shown); err != nil {
		return "", false, fmt.Errorf("parse %s: %w", path, err)
	}
	start, err := processStart(pid)
	if err != nil {
		return "", false, err
	}
	if shown.Start != start {
		return "", false, nil
	}
	if shown.Session == nil {
		return "", true, nil
	}
	return *shown.Session, true, nil
}

func processStart(pid int) (string, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return "", err
	}
	end := strings.LastIndexByte(string(data), ')')
	fields := strings.Fields(string(data)[end+1:])
	if end < 0 || len(fields) < 20 {
		return "", fmt.Errorf("unexpected /proc/%d/stat", pid)
	}
	return fields[19], nil
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
