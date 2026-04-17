package browser

import (
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// firefoxBinary is the fallback launcher when three_body.browser.command is unset.
const firefoxBinary = "firefox-developer-edition"

var (
	// firefoxTitleSuffixes are stripped from Hypr window titles so they compare against session-store tab titles.
	firefoxTitleSuffixes = []string{
		" — Firefox Developer Edition",
		" — Mozilla Firefox",
	}
	// trivialBrowserURLs flag blank/home-only windows so snapshot heuristics skip them.
	// localhost:42069 is the newtab daemon.
	trivialBrowserURLs = map[string]struct{}{
		"":                         {},
		"about:blank":              {},
		"about:home":               {},
		"about:newtab":             {},
		"http://localhost:42069/":  {},
		"https://localhost:42069/": {},
	}
)

// firefoxRunningPIDs returns PIDs of running Firefox Developer Edition processes.
//
// pgrep exit code 1 (no matches) and a missing pgrep binary are both empty-slice, not errors.
func firefoxRunningPIDs() ([]int, error) {
	cmd := exec.Command("pgrep", "-f", "/usr/lib/firefox-developer-edition/firefox|firefox-developer-edition")
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		if errors.Is(err, exec.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var pids []int
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

// stopFirefox ensures no Firefox process is running before an exact restore rewrites sessionstore files.
//
// Without force it errors when PIDs exist. With force it SIGTERMs them and polls up to 15s for exit.
func stopFirefox(force bool) error {
	pids, err := firefoxRunningPIDs()
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		return nil
	}
	if !force {
		return fmt.Errorf("Firefox is running; rerun with --force for exact restore")
	}

	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		pids, err = firefoxRunningPIDs()
		if err != nil {
			return err
		}
		if len(pids) == 0 {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("Firefox did not exit cleanly after SIGTERM")
}

func (b *Browser) launchFirefoxProfile(profile firefoxProfile) error {
	cmd := append(slices.Clone(b.browserCommandParts()), "--new-instance", "--profile", profile.Root)
	return exec.Command(cmd[0], cmd[1:]...).Start()
}

func (b *Browser) browserCommandParts() []string {
	if b.state != nil {
		cfg := b.state.GetConfig()
		if cfg != nil {
			if browser, ok := cfg.ThreeBody["browser"]; ok {
				parts := strings.Fields(strings.TrimSpace(browser.Command))
				if len(parts) > 0 {
					return parts
				}
			}
		}
	}
	return []string{firefoxBinary}
}

func (b *Browser) currentFirefoxTitle() string {
	if b.hypr == nil {
		return ""
	}
	active, err := b.hypr.ActiveWindow()
	if err != nil || active == nil {
		return ""
	}
	if !strings.Contains(strings.ToLower(active.Class), "firefox") {
		return ""
	}
	return trimFirefoxTitle(active.Title)
}

func trimFirefoxTitle(title string) string {
	title = strings.TrimSpace(title)
	for _, suffix := range firefoxTitleSuffixes {
		title = strings.TrimSuffix(title, suffix)
	}
	return strings.TrimSpace(title)
}

// titlesMatch does a loose compare: exact match or one being a prefix of the other.
//
// Firefox truncates long titles with ellipses in some contexts, so prefix match covers the cut-off Hypr title case.
func titlesMatch(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}
