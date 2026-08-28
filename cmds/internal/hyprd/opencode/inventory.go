package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"dotfiles/cmds/internal/hyprd/kitty"
)

type Snapshot struct {
	JobID       string    `json:"job_id"`
	Mode        Mode      `json:"mode"`
	CreatedAt   time.Time `json:"created_at"`
	Directories []string  `json:"directories"`
	Panes       []Pane    `json:"panes"`
}

type Pane struct {
	KittyPaneID int    `json:"kitty_pane_id"`
	KittyPID    int    `json:"kitty_pid"`
	AttachPID   int    `json:"attach_pid"`
	Directory   string `json:"directory"`
	SessionID   string `json:"session_id,omitempty"`
	Focused     bool   `json:"focused"`
	PromptDirty *bool  `json:"prompt_dirty,omitempty"`
	Generation  int64  `json:"generation,omitempty"`
}

type paneKey struct {
	KittyPID int
	PaneID   int
}

type contextRecord struct {
	KittyPID    int    `json:"kitty_pid"`
	KittyWindow int    `json:"kitty_window_id"`
	UpdatedAt   int64  `json:"updated_at"`
	Directory   string `json:"directory"`
	Generation  int64  `json:"generation"`
	sessionID   string
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

	contexts, contextErr := readContexts()
	live := make(map[paneKey]struct{}, len(panes))
	for _, pane := range panes {
		live[paneKey{KittyPID: pane.KittyPID, PaneID: pane.Pane.ID}] = struct{}{}
	}
	byPane := contextsByPane(contexts, live)
	directories := make(map[string]struct{})
	for _, ctx := range byPane {
		addDirectory(directories, ctx.Directory)
	}

	snapshot := Snapshot{JobID: jobID, Mode: mode, CreatedAt: time.Now()}
	for _, located := range panes {
		attach, found := attachProcess(located.Pane)
		if !found {
			continue
		}

		key := paneKey{KittyPID: located.KittyPID, PaneID: located.Pane.ID}
		ctx := byPane[key]
		directory := option(attach.Cmdline, "--dir")
		if directory != "" && !filepath.IsAbs(directory) {
			base := attach.CWD
			if base == "" {
				base = located.Pane.CWD
			}
			directory = filepath.Join(base, directory)
		}
		if directory == "" {
			directory = attach.CWD
		}
		if directory == "" {
			directory = located.Pane.CWD
		}
		directory = cleanDirectory(directory)
		addDirectory(directories, directory)

		sessionID := option(attach.Cmdline, "--session", "-s")
		if !validSessionID(sessionID) {
			sessionID = ctx.sessionID
		}
		if !validSessionID(sessionID) {
			sessionID = ""
		}

		snapshot.Panes = append(snapshot.Panes, Pane{
			KittyPaneID: located.Pane.ID,
			KittyPID:    located.KittyPID,
			AttachPID:   attach.PID,
			Directory:   directory,
			SessionID:   sessionID,
			Focused:     located.Focused,
			Generation:  ctx.Generation,
		})
	}

	sort.Slice(snapshot.Panes, func(i, j int) bool {
		if snapshot.Panes[i].KittyPID != snapshot.Panes[j].KittyPID {
			return snapshot.Panes[i].KittyPID < snapshot.Panes[j].KittyPID
		}
		return snapshot.Panes[i].KittyPaneID < snapshot.Panes[j].KittyPaneID
	})
	for directory := range directories {
		snapshot.Directories = append(snapshot.Directories, directory)
	}
	sort.Strings(snapshot.Directories)

	if err := persistSnapshot(snapshot); err != nil {
		return Snapshot{}, nil, err
	}
	if contextErr != nil {
		warnings = append(warnings, "Kitty context unavailable: "+contextErr.Error())
	}
	return snapshot, warnings, nil
}

func attachProcess(pane kitty.Pane) (kitty.Process, bool) {
	for _, process := range pane.ForegroundProcesses {
		if process.PID > 0 && process.PID != pane.PID && isAttachCommand(process.Cmdline) {
			return process, true
		}
	}
	return kitty.Process{}, false
}

func isAttachCommand(args []string) bool {
	for i := 0; i+1 < len(args); i++ {
		if filepath.Base(args[i]) == "opencode" && args[i+1] == "attach" {
			return true
		}
	}
	return false
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

func readContexts() (map[string]contextRecord, error) {
	path := contextPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]contextRecord{}, nil
		}
		return map[string]contextRecord{}, fmt.Errorf("read %s: %w", path, err)
	}
	var contexts map[string]contextRecord
	if err := json.Unmarshal(data, &contexts); err != nil {
		return map[string]contextRecord{}, fmt.Errorf("parse %s: %w", path, err)
	}
	for id, ctx := range contexts {
		ctx.sessionID = id
		contexts[id] = ctx
	}
	return contexts, nil
}

func contextsByPane(contexts map[string]contextRecord, live map[paneKey]struct{}) map[paneKey]contextRecord {
	byPane := make(map[paneKey]contextRecord)
	for _, ctx := range contexts {
		key := paneKey{KittyPID: ctx.KittyPID, PaneID: ctx.KittyWindow}
		if _, ok := live[key]; !ok {
			continue
		}
		if current, ok := byPane[key]; !ok || ctx.UpdatedAt > current.UpdatedAt {
			byPane[key] = ctx
		}
	}
	return byPane
}

func contextPath() string {
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "opencode", "kitty-context.json")
	}
	return filepath.Join("/tmp", "opencode-"+strconv.Itoa(os.Getuid()), "kitty-context.json")
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

func addDirectory(set map[string]struct{}, directory string) {
	if directory = cleanDirectory(directory); directory != "" {
		set[directory] = struct{}{}
	}
}

func directoryList(set map[string]struct{}) []string {
	list := make([]string, 0, len(set))
	for directory := range set {
		list = append(list, directory)
	}
	return list
}

func cleanDirectory(directory string) string {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return ""
	}
	return filepath.Clean(directory)
}
