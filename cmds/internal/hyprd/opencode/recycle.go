package opencode

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"dotfiles/cmds/internal/hyprd/kitty"
)

const (
	termWait  = 5 * time.Second
	shellWait = 5 * time.Second
)

func recycle(snapshot Snapshot, skip map[paneKey]string, force bool) []string {
	results := make([]string, 0, len(snapshot.Panes))
	for _, pane := range snapshot.Panes {
		key := paneKey{KittyPID: pane.KittyPID, PaneID: pane.KittyPaneID}
		label := fmt.Sprintf("kitty %d pane %d", pane.KittyPID, pane.KittyPaneID)
		if reason := skip[key]; reason != "" {
			results = append(results, label+": skipped ("+reason+")")
			continue
		}
		if !force {
			if reason := currentPaneGate(pane); reason != "" {
				results = append(results, label+": skipped ("+reason+")")
				continue
			}
		}
		result, err := recyclePane(pane)
		if err != nil {
			results = append(results, label+": failed ("+err.Error()+")")
			continue
		}
		results = append(results, label+": "+result)
	}
	return results
}

func currentPaneGate(target Pane) string {
	if target.PromptDirty != nil && *target.PromptDirty {
		return "draft present"
	}
	panes, inventoryErr := kitty.Inventory()
	for _, pane := range panes {
		if pane.KittyPID == target.KittyPID && pane.Pane.ID == target.KittyPaneID {
			if pane.Focused {
				return "focused pane may contain a draft"
			}
			return ""
		}
	}
	if inventoryErr != nil {
		return "foreground unknown"
	}
	return ""
}

func recyclePane(snapshot Pane) (string, error) {
	client := kitty.NewClient(snapshot.KittyPID)
	pane, found, err := client.Pane(snapshot.KittyPaneID)
	if err != nil {
		return "", err
	}
	if !found {
		return "skipped (pane closed)", nil
	}

	if !loginZshReady(pane) {
		if !snapshotAttachIsForeground(snapshot, pane) {
			return "skipped (foreground changed)", nil
		}
		if err := signal(snapshot.AttachPID, syscall.SIGTERM); err != nil {
			return "", fmt.Errorf("SIGTERM attach %d: %w", snapshot.AttachPID, err)
		}
		if !waitAttachExit(client, snapshot, termWait) {
			current, found, err := client.Pane(snapshot.KittyPaneID)
			if err != nil {
				return "", fmt.Errorf("inspect attach before SIGKILL: %w", err)
			}
			if !found || !snapshotAttachIsForeground(snapshot, current) {
				return "skipped (foreground changed before SIGKILL)", nil
			}
			if err := signal(snapshot.AttachPID, syscall.SIGKILL); err != nil {
				return "", fmt.Errorf("SIGKILL attach %d: %w", snapshot.AttachPID, err)
			}
		}
	}

	if !waitLoginZsh(client, snapshot.KittyPaneID, shellWait) {
		return "skipped (login zsh did not become ready)", nil
	}
	command := "c\n"
	result := "recycled (unpinned picker)"
	if validSessionID(snapshot.SessionID) {
		command = "c --session " + snapshot.SessionID + "\n"
		result = "recycled " + snapshot.SessionID
	}
	if err := client.SendText(snapshot.KittyPaneID, command); err != nil {
		return "", err
	}
	return result, nil
}

func snapshotAttachIsForeground(snapshot Pane, pane kitty.Pane) bool {
	if snapshot.AttachPID <= 0 || snapshot.AttachPID == pane.PID {
		return false
	}
	for _, process := range pane.ForegroundProcesses {
		if process.PID == snapshot.AttachPID && isAttachCommand(process.Cmdline) {
			return true
		}
	}
	return false
}

func waitLoginZsh(client *kitty.Client, paneID int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		pane, found, err := client.Pane(paneID)
		if err == nil && found && loginZshReady(pane) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func loginZshReady(pane kitty.Pane) bool {
	if pane.PID <= 0 || !loginZshCommand(pane.Cmdline) || len(pane.ForegroundProcesses) != 1 {
		return false
	}
	process := pane.ForegroundProcesses[0]
	return process.PID == pane.PID && len(process.Cmdline) > 0 && zshName(process.Cmdline[0])
}

func loginZshCommand(args []string) bool {
	if len(args) == 0 || !zshName(args[0]) {
		return false
	}
	if strings.HasPrefix(filepath.Base(args[0]), "-") {
		return true
	}
	return slices.Contains(args[1:], "-l")
}

func zshName(command string) bool {
	name := filepath.Base(command)
	return name == "zsh" || name == "-zsh"
}

func signal(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	err := syscall.Kill(pid, sig)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func waitAttachExit(client *kitty.Client, snapshot Pane, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		pane, found, err := client.Pane(snapshot.KittyPaneID)
		if err == nil && (!found || !snapshotAttachIsForeground(snapshot, pane)) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func paneGates(snapshot Snapshot) (block, skip map[paneKey]string) {
	block = make(map[paneKey]string)
	skip = make(map[paneKey]string)
	panes, inventoryErr := kitty.Inventory()
	live := make(map[paneKey]kitty.LocatedPane, len(panes))
	for _, pane := range panes {
		live[paneKey{KittyPID: pane.KittyPID, PaneID: pane.Pane.ID}] = pane
	}
	for _, target := range snapshot.Panes {
		key := paneKey{KittyPID: target.KittyPID, PaneID: target.KittyPaneID}
		pane, found := live[key]
		if !found {
			if inventoryErr != nil {
				block[key] = "foreground unknown"
				continue
			}
			skip[key] = "pane closed"
			continue
		}
		if target.PromptDirty != nil && *target.PromptDirty {
			block[key] = "draft present"
			continue
		}
		if pane.Focused {
			block[key] = "focused pane may contain a draft"
		}
	}
	return block, skip
}
