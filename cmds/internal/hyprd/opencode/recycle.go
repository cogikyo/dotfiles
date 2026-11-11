package opencode

import (
	"context"
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

func recycle(views *kitty.Tracker, snapshot Snapshot, minAge time.Duration) []string {
	closed, details := closeTUIs(views, snapshot, nil, false, minAge)
	return append(details, relaunch(closed)...)
}

func closeTUIs(views *kitty.Tracker, snapshot Snapshot, skip map[kitty.PaneKey]string, force bool, minAge time.Duration) ([]Pane, []string) {
	var closed []Pane
	var details []string
	for _, pane := range snapshot.Panes {
		if reason := skip[pane.key()]; reason != "" {
			details = append(details, pane.label()+": skipped ("+reason+")")
			continue
		}
		if !force {
			live, reason := recheck(views, pane, minAge)
			if reason != "" {
				details = append(details, pane.label()+": skipped ("+reason+")")
				continue
			}
			pane = live
		}
		skipped, err := closeTUI(views, pane, !force)
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

func recheck(views *kitty.Tracker, pane Pane, minAge time.Duration) (Pane, string) {
	if reason := pane.gate(time.Now(), minAge); reason != "" {
		return Pane{}, reason
	}
	live, reason := current(views, pane)
	if reason == "" {
		reason = live.gate(time.Now(), minAge)
	}
	return live, reason
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

func closeTUI(views *kitty.Tracker, snapshot Pane, gated bool) (string, error) {
	client := kitty.NewClient(snapshot.KittyPID)
	located, found, err := client.Locate(context.Background(), snapshot.KittyPaneID)
	if err != nil {
		return "", err
	}
	if !found {
		return "pane closed", nil
	}
	pane := located.Pane
	if !loginZshCommand(pane.Cmdline) {
		return "no login shell to relaunch into", nil
	}

	if !loginZshReady(pane) {
		if !tuiIsForeground(snapshot, pane) {
			return "foreground changed", nil
		}
		// Recheck view before each signal; the snapshot can outlive a tab switch.
		if reason := hiddenNow(views, located, gated); reason != "" {
			return reason, nil
		}
		if err := signal(snapshot.TUIPID, syscall.SIGTERM); err != nil {
			return "", fmt.Errorf("SIGTERM TUI %d: %w", snapshot.TUIPID, err)
		}
		if !waitTUIExit(client, snapshot, termWait) {
			current, found, err := client.Locate(context.Background(), snapshot.KittyPaneID)
			if err != nil {
				return "", fmt.Errorf("inspect TUI before SIGKILL: %w", err)
			}
			if !found || !tuiIsForeground(snapshot, current.Pane) {
				return "foreground changed before SIGKILL", nil
			}
			if reason := hiddenNow(views, current, gated); reason != "" {
				return reason + " before SIGKILL", nil
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

func hiddenNow(views *kitty.Tracker, pane kitty.LocatedPane, gated bool) string {
	if !gated {
		return ""
	}
	shown, known := views.Shown(pane)
	switch {
	case !known:
		return "view unknown"
	case shown:
		return "in view"
	}
	return ""
}

func launchTUI(snapshot Pane) (string, error) {
	client := kitty.NewClient(snapshot.KittyPID)
	pane, found, err := client.Pane(context.Background(), snapshot.KittyPaneID)
	if err != nil {
		return "", err
	}
	if !found {
		return "skipped (pane closed)", nil
	}
	if !loginZshReady(pane) {
		return "skipped (foreground changed before relaunch)", nil
	}
	result := "recycled (home)"
	if ValidSessionID(snapshot.SessionID) {
		result = "recycled " + snapshot.SessionID
	}
	if err := launch(context.Background(), client, snapshot.KittyPaneID, snapshot.SessionID); err != nil {
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
		pane, found, err := client.Pane(context.Background(), paneID)
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
		pane, found, err := client.Pane(context.Background(), snapshot.KittyPaneID)
		if err == nil && (!found || !tuiIsForeground(snapshot, pane)) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func paneGates(snapshot Snapshot) (block, skip map[kitty.PaneKey]string) {
	block = make(map[kitty.PaneKey]string)
	skip = make(map[kitty.PaneKey]string)
	now := time.Now()
	for _, pane := range snapshot.Panes {
		reason := pane.gate(now, 0)
		switch {
		case reason == "":
		case !pane.Shell:
			skip[pane.key()] = reason
		default:
			block[pane.key()] = reason
		}
	}
	return block, skip
}
