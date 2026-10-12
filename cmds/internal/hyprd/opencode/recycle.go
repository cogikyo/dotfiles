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
	closed, details := closeTUIs(snapshot, skip, force)
	return append(details, relaunch(closed)...)
}

func closeTUIs(snapshot Snapshot, skip map[paneKey]string, force bool) ([]Pane, []string) {
	var closed []Pane
	var details []string
	for _, pane := range snapshot.Panes {
		key := paneKey{KittyPID: pane.KittyPID, PaneID: pane.KittyPaneID}
		if reason := skip[key]; reason != "" {
			details = append(details, pane.label()+": skipped ("+reason+")")
			continue
		}
		if !force {
			if reason := currentPaneGate(pane); reason != "" {
				details = append(details, pane.label()+": skipped ("+reason+")")
				continue
			}
		}
		skipped, err := closeTUI(pane)
		switch {
		case err != nil:
			details = append(details, pane.label()+": failed ("+err.Error()+")")
		case skipped != "":
			details = append(details, pane.label()+": skipped ("+skipped+")")
		default:
			closed = append(closed, pane)
		}
	}
	return closed, details
}

func relaunch(panes []Pane) []string {
	details := make([]string, 0, len(panes))
	for _, pane := range panes {
		result, err := launchTUI(pane)
		if err != nil {
			result = "failed (" + err.Error() + ")"
		}
		details = append(details, pane.label()+": "+result)
	}
	return details
}

func (p Pane) label() string {
	return fmt.Sprintf("kitty %d pane %d", p.KittyPID, p.KittyPaneID)
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

func closeTUI(snapshot Pane) (string, error) {
	client := kitty.NewClient(snapshot.KittyPID)
	pane, found, err := client.Pane(snapshot.KittyPaneID)
	if err != nil {
		return "", err
	}
	if !found {
		return "pane closed", nil
	}

	if !loginZshReady(pane) {
		if !tuiIsForeground(snapshot, pane) {
			return "foreground changed", nil
		}
		if err := signal(snapshot.TUIPID, syscall.SIGTERM); err != nil {
			return "", fmt.Errorf("SIGTERM TUI %d: %w", snapshot.TUIPID, err)
		}
		if !waitTUIExit(client, snapshot, termWait) {
			current, found, err := client.Pane(snapshot.KittyPaneID)
			if err != nil {
				return "", fmt.Errorf("inspect TUI before SIGKILL: %w", err)
			}
			if !found || !tuiIsForeground(snapshot, current) {
				return "foreground changed before SIGKILL", nil
			}
			if err := signal(snapshot.TUIPID, syscall.SIGKILL); err != nil {
				return "", fmt.Errorf("SIGKILL TUI %d: %w", snapshot.TUIPID, err)
			}
		}
	}

	if !waitLoginZsh(client, snapshot.KittyPaneID, shellWait) {
		return "login zsh did not become ready", nil
	}
	return "", nil
}

func launchTUI(snapshot Pane) (string, error) {
	client := kitty.NewClient(snapshot.KittyPID)
	pane, found, err := client.Pane(snapshot.KittyPaneID)
	if err != nil {
		return "", err
	}
	if !found {
		return "skipped (pane closed)", nil
	}
	if !loginZshReady(pane) {
		return "skipped (foreground changed before relaunch)", nil
	}
	command := "c\n"
	result := "recycled (home)"
	if validSessionID(snapshot.SessionID) {
		command = "c --session " + snapshot.SessionID + "\n"
		result = "recycled " + snapshot.SessionID
	}
	if err := client.SendText(snapshot.KittyPaneID, command); err != nil {
		return "", err
	}
	return result, nil
}

func tuiIsForeground(snapshot Pane, pane kitty.Pane) bool {
	if snapshot.TUIPID <= 0 || snapshot.TUIPID == pane.PID {
		return false
	}
	for _, process := range pane.ForegroundProcesses {
		if process.PID == snapshot.TUIPID && isTUICommand(process.Cmdline) {
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

func waitTUIExit(client *kitty.Client, snapshot Pane, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		pane, found, err := client.Pane(snapshot.KittyPaneID)
		if err == nil && (!found || !tuiIsForeground(snapshot, pane)) {
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
