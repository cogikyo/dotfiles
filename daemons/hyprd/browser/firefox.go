package browser

// firefox.go handles Firefox process lifecycle checks and title matching against session-store window metadata.

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

const firefoxBinary = "firefox-developer-edition"

var (
	// firefoxTitleSuffixes are stripped to normalize Hypr window titles for session-store comparison.
	firefoxTitleSuffixes = []string{
		" — Firefox Developer Edition",
		" — Mozilla Firefox",
	}
	// trivialBrowserURLs are ignored by snapshot heuristics when picking "interesting" windows.
	trivialBrowserURLs = map[string]struct{}{
		"":                         {},
		"about:blank":              {},
		"about:home":               {},
		"about:newtab":             {},
		"http://localhost:42069/":  {},
		"https://localhost:42069/": {},
	}
)

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

// stopFirefox ensures no Firefox is running; with force it SIGTERMs and polls up to 15s.
func stopFirefox(force bool) error {
	pids, err := firefoxRunningPIDs()
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		return nil
	}
	if !force {
		return fmt.Errorf("firefox is running; rerun with --force for exact restore")
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

// titlesMatch returns true on exact or prefix match (Firefox truncates long titles in the session store).
func titlesMatch(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}
