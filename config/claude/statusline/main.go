// Package main implements the Claude Code custom statusline renderer.
//
// Reads JSON from stdin (the Claude Code statusline hook) and prints an ANSI-colored Nerd Font string to stdout.
// Segments are delimited by ╼╾ and cover: working directory, git state, model/effort, context usage, and 5h/7d rate gauges.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ╭────────────────────────────────────────────────────────────────────────────╮
// │ ANSI colors and Nerd Font icons                                            │
// ╰────────────────────────────────────────────────────────────────────────────╯

const (
	bold    = "\033[1m"
	reset   = "\033[0m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	gray    = "\033[90m"
	brRed   = "\033[91m"
	brGreen = "\033[92m"
	brYello = "\033[93m"
	brBlue  = "\033[94m"
	pink    = "\033[95m"
)

const (
	sep           = " ╼╾ "
	iconModel     = "󰯉 "
	iconEffortLo  = "󰤟"
	iconEffortMd  = "󰤢"
	iconEffortHi  = "󰤥"
	iconEffortXHi = "󰤨"
	iconEffortMx  = ""
	iconEffortAu  = "󰙴"
	iconEffortUn  = "󰤫"
	gitBranch     = " "
	gitAhead      = "⮭"
	gitBehind     = "⮯"
	gitStaged     = ""
	gitModified   = " "
	gitUntracked  = " "
	gitDeleted    = "󰚃 "
	gitStashed    = "󰸧 "
	gitRenamed    = "󰑕 "
	gitConflict   = " "
	barContext    = "㊋"
	barFilled     = "◉"
	barEmpty      = "○"
	iconError     = "󰅜"
)

var barProgress = []string{"󰪞", "󰪟", "󰪠", "󰪡", "󰪢", "󰪣", "󰪤", "󰪥", "", ""}

// ╭────────────────────────────────────────────────────────────────────────────╮
// │ Types                                                                      │
// ╰────────────────────────────────────────────────────────────────────────────╯

// Input mirrors the statusline JSON payload Claude Code writes to stdin.
//
// Only fields the renderer consumes are declared; extras are ignored.
type Input struct {
	Workspace struct {
		CurrentDir string `json:"current_dir"`
	} `json:"workspace"`
	Model struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Effort struct {
		Level string `json:"level"`
	} `json:"effort"`
	ContextWindow struct {
		ContextWindowSize int `json:"context_window_size"`
		CurrentUsage      *struct {
			InputTokens         int `json:"input_tokens"`
			CacheCreationTokens int `json:"cache_creation_input_tokens"`
			CacheReadTokens     int `json:"cache_read_input_tokens"`
		} `json:"current_usage"`
	} `json:"context_window"`
	RateLimits *struct {
		FiveHour *struct {
			UsedPercentage float64 `json:"used_percentage"`
			ResetsAt       int64   `json:"resets_at"`
		} `json:"five_hour"`
		SevenDay *struct {
			UsedPercentage float64 `json:"used_percentage"`
			ResetsAt       int64   `json:"resets_at"`
		} `json:"seven_day"`
	} `json:"rate_limits"`
}

// GitStatus is the parsed result of `git status --porcelain=v2 --branch --show-stash`.
type GitStatus struct {
	Branch     string
	Ahead      int // commits ahead of upstream
	Behind     int // commits behind upstream
	Staged     int // index entries with any change vs HEAD
	Modified   int // worktree modifications (M/T in the Y column)
	Untracked  int
	Deleted    int
	Stashed    int
	Renamed    int
	Conflicted int // unmerged entries
}


// ╭────────────────────────────────────────────────────────────────────────────╮
// │ Percent-tier helpers and effort level                                      │
// ╰────────────────────────────────────────────────────────────────────────────╯

// pctColors ramps cool → warm → alarm across 9 tiers; see pctTier for boundaries.
var pctColors = []string{blue, brBlue, green, brGreen, brYello, yellow, red, brRed, pink}

// pctTier returns the index into pctColors for a given percent.
func pctTier(pct int) int {
	switch {
	case pct < 10:
		return 0
	case pct < 20:
		return 1
	case pct < 30:
		return 2
	case pct < 40:
		return 3
	case pct < 50:
		return 4
	case pct < 60:
		return 5
	case pct < 70:
		return 6
	case pct < 90:
		return 7
	default:
		return 8
	}
}

func pctColor(pct int) string {
	return pctColors[pctTier(pct)]
}

func progressIcon(pct int) string {
	idx := min(max(pct/10, 0), 9)
	return barProgress[idx]
}

func buildBar(filled, total int) string {
	var b strings.Builder
	for range filled {
		b.WriteString(barFilled)
	}
	for range total - filled {
		b.WriteString(barEmpty)
	}
	return b.String()
}

func effortIcon(level string) (string, string) {
	switch level {
	case "low":
		return iconEffortLo, brBlue
	case "medium":
		return iconEffortMd, blue
	case "high":
		return iconEffortHi, yellow
	case "xhigh":
		return iconEffortXHi, red
	case "max":
		return iconEffortMx, pink
	case "auto":
		return iconEffortAu, blue
	default:
		return iconEffortUn, gray
	}
}

// ╭────────────────────────────────────────────────────────────────────────────╮
// │ Git status                                                                 │
// ╰────────────────────────────────────────────────────────────────────────────╯

// getGitStatus returns a populated GitStatus for dir, or nil if dir isn't a git worktree or git fails.
//
// Uses porcelain=v2 for a version-stable parser and --no-optional-locks to avoid blocking on a concurrent index lock.
func getGitStatus(dir string) *GitStatus {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return nil
	}

	cmd = exec.Command("git", "-C", dir, "--no-optional-locks", "status", "--porcelain=v2", "--branch", "--show-stash")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	gs := &GitStatus{}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		switch {
		case strings.HasPrefix(line, "# branch.head "):
			gs.Branch = strings.TrimPrefix(line, "# branch.head ")

		case strings.HasPrefix(line, "# branch.ab "):
			// format: "# branch.ab +N -M"
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				if v, err := strconv.Atoi(strings.TrimPrefix(parts[2], "+")); err == nil {
					gs.Ahead = v
				}
				if v, err := strconv.Atoi(strings.TrimPrefix(parts[3], "-")); err == nil {
					gs.Behind = v
				}
			}

		case strings.HasPrefix(line, "# stash "):
			if v, err := strconv.Atoi(strings.TrimPrefix(line, "# stash ")); err == nil {
				gs.Stashed = v
			}

		case strings.HasPrefix(line, "#"):
			continue

		default:
			// entry lines: "<type> <XY> ..."
			if len(line) < 4 {
				continue
			}
			entryType := line[0]
			xy := line[2:4]

			switch entryType {
			case '1': // tracked file change
				x, y := xy[0], xy[1]
				if x != '.' && x != ' ' {
					gs.Staged++
				}
				if y == 'M' || y == 'T' {
					gs.Modified++
				}
				if y == 'D' {
					gs.Deleted++
				}
			case '2': // renamed/copied
				gs.Renamed++
			case 'u': // unmerged (conflict)
				gs.Conflicted++
			case '?': // untracked
				gs.Untracked++
			}
		}
	}

	return gs
}

// ╭────────────────────────────────────────────────────────────────────────────╮
// │ Rate-limit reset formatting                                                │
// ╰────────────────────────────────────────────────────────────────────────────╯

func formatResetTime(epoch int64) string {
	if epoch == 0 {
		return ""
	}
	t := time.Unix(epoch, 0).Local()
	return fmt.Sprintf(" (%02d:%02d)", t.Hour(), t.Minute())
}

func formatResetDate(epoch int64) string {
	if epoch == 0 {
		return ""
	}
	t := time.Unix(epoch, 0).Local()
	return fmt.Sprintf(" (%02d/%02d %02d:%02d)", t.Month(), t.Day(), t.Hour(), t.Minute())
}

// ╭────────────────────────────────────────────────────────────────────────────╮
// │ Output formatting and main                                                 │
// ╰────────────────────────────────────────────────────────────────────────────╯

// gitStateColor picks a branch color under a highest-signal-wins ordering.
//
// Order: staged yellow, unstaged cyan, diverged magenta, ahead green, behind red, clean blue.
func gitStateColor(gs *GitStatus) string {
	if gs == nil {
		return blue
	}
	staged := gs.Staged > 0
	unstaged := gs.Modified+gs.Untracked+gs.Deleted+gs.Renamed+gs.Conflicted > 0
	ahead := gs.Ahead > 0
	behind := gs.Behind > 0
	switch {
	case staged:
		return yellow
	case unstaged:
		return cyan
	case ahead && behind:
		return magenta
	case ahead:
		return green
	case behind:
		return red
	default:
		return blue
	}
}

func buildGitStats(gs *GitStatus) string {
	var stats strings.Builder
	if gs.Ahead > 0 {
		fmt.Fprintf(&stats, " %s%s%d", green, gitAhead, gs.Ahead)
	}
	if gs.Behind > 0 {
		fmt.Fprintf(&stats, " %s%s%d", brRed, gitBehind, gs.Behind)
	}
	if gs.Staged > 0 {
		fmt.Fprintf(&stats, " %s%s%d", yellow, gitStaged, gs.Staged)
	}
	if gs.Modified > 0 {
		fmt.Fprintf(&stats, " %s%s%d", cyan, gitModified, gs.Modified)
	}
	if gs.Untracked > 0 {
		fmt.Fprintf(&stats, " %s%s%d", brYello, gitUntracked, gs.Untracked)
	}
	if gs.Deleted > 0 {
		fmt.Fprintf(&stats, " %s%s%d", red, gitDeleted, gs.Deleted)
	}
	if gs.Stashed > 0 {
		fmt.Fprintf(&stats, " %s%s%d", gray, gitStashed, gs.Stashed)
	}
	if gs.Renamed > 0 {
		fmt.Fprintf(&stats, " %s%s%d", magenta, gitRenamed, gs.Renamed)
	}
	if gs.Conflicted > 0 {
		fmt.Fprintf(&stats, " %s%s%d", pink, gitConflict, gs.Conflicted)
	}
	if stats.Len() > 0 {
		stats.WriteString(reset)
	}
	return stats.String()
}

func main() {
	inputData, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}

	var input Input
	if err := json.Unmarshal(inputData, &input); err != nil {
		return
	}

	dir := input.Workspace.CurrentDir
	home, _ := os.UserHomeDir()
	gitStatus := getGitStatus(dir)

	var out strings.Builder

	// ├─ directory ──────────────────────────────────────────────────────────────┤
	// Strip $HOME prefix so repo dirs read as "dotfiles/…" instead of "/home/…/dotfiles/…".
	displayDir := dir
	if after, found := strings.CutPrefix(dir, home+"/"); found {
		displayDir = after
	} else if dir == home {
		displayDir = ""
	}
	out.WriteString(red + iconModel + bold + red + displayDir + reset)

	// ├─ git branch and file-status counts ──────────────────────────────────────┤
	if gitStatus != nil && gitStatus.Branch != "" {
		branchColor := gitStateColor(gitStatus)
		out.WriteString(" " + branchColor + gitBranch + gitStatus.Branch + reset)
		out.WriteString(buildGitStats(gitStatus))
	}

	// ├─ model name + effort level ──────────────────────────────────────────────┤
	if input.Model.DisplayName != "" {
		level := input.Effort.Level
		icon, color := effortIcon(level)
		out.WriteString(gray + sep + reset)
		fmt.Fprintf(&out, "%s%s %s%s", color, icon, input.Model.DisplayName, reset)
	}

	// ├─ context window bar ─────────────────────────────────────────────────────┤
	// Percent reflects true usage against the model's full window (1M/2M/…).
	// Color and fill scale against a 250k "comfort budget" so the bar tracks where we want to stay, not the hard cap.
	if input.ContextWindow.CurrentUsage != nil && input.ContextWindow.ContextWindowSize > 0 {
		usage := input.ContextWindow.CurrentUsage
		total := usage.InputTokens + usage.CacheCreationTokens + usage.CacheReadTokens
		size := input.ContextWindow.ContextWindowSize
		pct := total * 100 / size
		colorRef := min(size, 250_000)
		colorPct := min(total*100/colorRef, 100)
		tier := pctTier(colorPct)
		color := pctColors[tier]
		bar := buildBar(tier+1, len(pctColors))
		out.WriteString(gray + sep + reset)
		fmt.Fprintf(&out, "%s%s%s %d%%%s", color, barContext, bar, pct, reset)
	}

	// ├─ rate-limit gauges (5-hour / 7-day) ─────────────────────────────────────┤
	if input.RateLimits != nil {
		if fh := input.RateLimits.FiveHour; fh != nil {
			pct := int(fh.UsedPercentage)
			color := pctColor(pct)
			icon := progressIcon(pct)
			out.WriteString(gray + sep + reset)
			fmt.Fprintf(&out, "%s%s %d%%%s%s", color, icon, pct, formatResetTime(fh.ResetsAt), reset)
		}
		if sd := input.RateLimits.SevenDay; sd != nil {
			pct := int(sd.UsedPercentage)
			color := pctColor(pct)
			icon := progressIcon(pct)
			out.WriteString(gray + sep + reset)
			fmt.Fprintf(&out, "%s%s %d%%%s%s", color, icon, pct, formatResetDate(sd.ResetsAt), reset)
		}
	}

	fmt.Print(out.String() + reset)
}
