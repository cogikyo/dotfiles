package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/kitty"
	opencodepkg "dotfiles/cmds/internal/hyprd/opencode"
	"dotfiles/cmds/internal/hyprd/wm"
)

const (
	openUsage       = "usage: open <kitty-pid> <kitty-window-id> <session-id> <directory>"
	workspacesUsage = "usage: workspaces <kitty-pid> [<directory>[\\t<directory>...]]"
	openTimeout     = 50 * time.Second
)

type openRequest struct {
	caller  kitty.PaneKey
	session string
	dir     string
}

func parseOpen(arg string) (openRequest, error) {
	fields := strings.SplitN(arg, " ", 4)
	if len(fields) < 4 {
		return openRequest{}, errors.New(openUsage)
	}
	pid, pidErr := strconv.Atoi(fields[0])
	pane, paneErr := strconv.Atoi(fields[1])
	if pidErr != nil || paneErr != nil || pid <= 0 || pane <= 0 {
		return openRequest{}, fmt.Errorf("invalid kitty pane %s %s", fields[0], fields[1])
	}
	if !opencodepkg.ValidSessionID(fields[2]) {
		return openRequest{}, fmt.Errorf("invalid session id %q", fields[2])
	}
	if !filepath.IsAbs(fields[3]) {
		return openRequest{}, fmt.Errorf("directory is not absolute: %s", fields[3])
	}
	return openRequest{
		caller:  kitty.PaneKey{KittyPID: pid, PaneID: pane},
		session: fields[2],
		dir:     filepath.Clean(fields[3]),
	}, nil
}

func (d *Daemon) handleOpen(arg string) string {
	req, err := parseOpen(arg)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	ctx, cancel := context.WithTimeoutCause(context.Background(), openTimeout, fmt.Errorf("open did not finish within %s", openTimeout))
	defer cancel()
	release, err := d.opencode.Reserve(ctx, "session open")
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	defer release()
	reply, err := d.open(ctx, req)
	if err != nil && ctx.Err() != nil {
		err = context.Cause(ctx)
	}
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return reply
}

func (d *Daemon) open(ctx context.Context, req openRequest) (string, error) {
	if pane, ok := opencodepkg.Showing(d.views, req.session, req.caller); ok {
		if err := d.showPane(ctx, pane.KittyPID, pane.PaneID, ""); err != nil {
			return "", err
		}
		return "focused", nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cfg := d.config.Load()
	ws := openWorkspace(cfg, req.dir, home)
	if ws <= 0 {
		return "", fmt.Errorf("no workspace for %s; set open.workspace", req.dir)
	}
	if ws == d.hypr.WorkspaceForPID(req.caller.KittyPID) {
		return "here", nil
	}

	term, pid, ok := d.agentsKitty(ctx, ws)
	if !ok {
		if term, pid, err = d.raiseAgents(ctx, ws); err != nil {
			return "", err
		}
	}
	env, err := term.ProcessEnv()
	if err != nil {
		return "", err
	}
	name := cmp.Or(env["HYPRD_TAB_PROFILE"], agentsBody)
	profile, ok := cfg.Tabs[name]
	if !ok {
		return "", fmt.Errorf("unknown tab profile %s", name)
	}

	manager, cwd := kitty.NewManager(d.hypr, d.state), cmp.Or(env["PROJECT_PATH"], home)
	tab, err := opencodepkg.Choose(ctx, term, profile, func(tab string) error {
		_, err := manager.EnsureTab(ctx, pid, name, tab, cwd)
		return err
	})
	if err != nil {
		return "", err
	}
	publish := func(event opencodepkg.Navigate) { d.server.Subs.Notify("navigate", event) }
	if err := opencodepkg.Open(ctx, pid, tab, req.session, publish); err != nil {
		return "", err
	}
	if err := d.showPane(ctx, pid, tab.Pane.ID, tab.Pane.Env["KITTY_TAB_ID"]); err != nil {
		return "", err
	}
	return fmt.Sprintf("opened %d %s", ws, tab.Name), nil
}

func parseWorkspaces(arg string) (int, []string, error) {
	head, rest, _ := strings.Cut(arg, " ")
	pid, err := strconv.Atoi(head)
	if err != nil || pid <= 0 {
		return 0, nil, errors.New(workspacesUsage)
	}
	if rest == "" {
		return pid, nil, nil
	}
	dirs := strings.Split(rest, "\t")
	for i, dir := range dirs {
		if !filepath.IsAbs(dir) {
			return 0, nil, fmt.Errorf("directory is not absolute: %s", dir)
		}
		dirs[i] = filepath.Clean(dir)
	}
	return pid, dirs, nil
}

func (d *Daemon) handleWorkspaces(arg string) string {
	pid, dirs, err := parseWorkspaces(arg)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	here, err := d.hypr.LookupWorkspace(pid)
	if err != nil {
		return fmt.Sprintf("error: find kitty workspace: %v", err)
	}
	return workspaces(d.config.Load(), home, here, dirs)
}

func workspaces(cfg *config.HyprConfig, home string, here int, dirs []string) string {
	fields := []string{strconv.Itoa(here)}
	for _, dir := range dirs {
		fields = append(fields, strconv.Itoa(openWorkspace(cfg, dir, home)))
	}
	return strings.Join(fields, " ")
}

func openWorkspace(cfg *config.HyprConfig, dir, home string) int {
	return cmp.Or(cfg.Sessions.Workspace(dir, home), cfg.Open.Workspace)
}

func (d *Daemon) showPane(ctx context.Context, pid, pane int, tab string) error {
	d.state.LockLayout()
	defer d.state.UnlockLayout()
	return wm.NewThreeBody(d.hypr, d.state).ShowPane(ctx, pid, pane, tab)
}
