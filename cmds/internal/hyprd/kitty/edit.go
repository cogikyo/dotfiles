package kitty

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"dotfiles/cmds/internal/hyprd/hypr"
)

// Edit focuses a workspace nvim and opens file in it.
// It prefers the editor Kitty window, then any Kitty pane running nvim.
func (t *Selector) Edit(filePath string) (string, error) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("stat %s: %w", abs, err)
	}

	wsID, err := t.hypr.ActiveWorkspace()
	if err != nil {
		return "", err
	}

	editor, err := t.findEditor(wsID)
	if err != nil {
		return "", err
	}

	win, client, pane, err := t.findNvim(wsID, editor)
	if err != nil {
		return "", err
	}

	if err := t.hypr.FocusWindow(win.Address); err != nil {
		return "", fmt.Errorf("focus editor: %w", err)
	}
	if err := client.FocusWindow(pane.ID); err != nil {
		return "", fmt.Errorf("focus nvim pane: %w", err)
	}
	if err := openInNvim(pane, abs); err != nil {
		return "", err
	}
	return fmt.Sprintf("edit: %s", abs), nil
}

func (t *Selector) findNvim(wsID int, preferred *hypr.Window) (*hypr.Window, *Client, Pane, error) {
	if preferred != nil {
		if client, pane, ok := nvimInWindow(preferred); ok {
			return preferred, client, pane, nil
		}
	}

	clients, err := t.hypr.Clients()
	if err != nil {
		return nil, nil, Pane{}, err
	}
	for i := range clients {
		win := &clients[i]
		if win.Workspace.ID != wsID || win.Class != "kitty" {
			continue
		}
		if preferred != nil && win.Address == preferred.Address {
			continue
		}
		if client, pane, ok := nvimInWindow(win); ok {
			return win, client, pane, nil
		}
	}
	return nil, nil, Pane{}, fmt.Errorf("no nvim on workspace %d", wsID)
}

func nvimInWindow(win *hypr.Window) (*Client, Pane, bool) {
	client := NewClient(win.Pid)
	state, err := client.FullState()
	if err != nil {
		return nil, Pane{}, false
	}
	for _, osWin := range state {
		for _, tab := range osWin.Tabs {
			for _, pane := range tab.Windows {
				if paneHasNvim(pane) {
					return client, pane, true
				}
			}
		}
	}
	return nil, Pane{}, false
}

func paneHasNvim(pane Pane) bool {
	if processIsNvim(pane.Cmdline) {
		return true
	}
	for _, proc := range pane.ForegroundProcesses {
		if processIsNvim(proc.Cmdline) {
			return true
		}
	}
	return false
}

func processIsNvim(cmd []string) bool {
	for _, part := range cmd {
		if filepath.Base(part) == "nvim" {
			return true
		}
	}
	return false
}

func openInNvim(pane Pane, file string) error {
	socket := nvimSocket(pane)
	if socket == "" {
		return fmt.Errorf("nvim server socket not found")
	}
	out, err := exec.Command("nvim", "--server", socket, "--remote-silent", file).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nvim --remote-silent: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func nvimSocket(pane Pane) string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join("/run/user", strconv.Itoa(os.Getuid()))
	}

	seen := map[int]bool{}
	var pids []int
	add := func(pid int) {
		if pid <= 0 || seen[pid] {
			return
		}
		seen[pid] = true
		pids = append(pids, pid)
	}

	for _, proc := range pane.ForegroundProcesses {
		add(proc.PID)
	}
	for _, pid := range append([]int(nil), pids...) {
		for _, child := range childPIDs(pid) {
			add(child)
		}
	}

	for _, pid := range pids {
		socket := filepath.Join(runtimeDir, fmt.Sprintf("nvim.%d.0", pid))
		if _, err := os.Stat(socket); err == nil {
			return socket
		}
	}
	return ""
}

func childPIDs(pid int) []int {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/children", pid, pid))
	if err != nil {
		return nil
	}
	fields := strings.Fields(string(data))
	pids := make([]int, 0, len(fields))
	for _, field := range fields {
		child, err := strconv.Atoi(field)
		if err != nil {
			continue
		}
		pids = append(pids, child)
	}
	return pids
}
