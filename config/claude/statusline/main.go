package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─────────────────────────────────────────────────
// ANSI Colors
// ─────────────────────────────────────────────────
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

// ─────────────────────────────────────────────────
// Icons
// ─────────────────────────────────────────────────
const (
	sep          = " ╼╾ "
	iconModel    = "󰯉 "
	gitBranch    = " "
	gitAhead     = "⮭"
	gitBehind    = "⮯"
	gitStaged    = ""
	gitModified  = " "
	gitUntracked = " "
	gitDeleted   = "󰚃 "
	gitStashed   = "󰸧 "
	gitRenamed   = "󰑕 "
	gitConflict  = " "
	barContext   = "㊋"
	barFilled    = "◉"
	barEmpty     = "○"
)

var barProgress = []string{"󰪞", "󰪟", "󰪠", "󰪡", "󰪢", "󰪣", "󰪤", "󰪥", "󰪦", "󰪧"}

// ─────────────────────────────────────────────────
// Input Types (from Claude Code JSON)
// ─────────────────────────────────────────────────

type Input struct {
	Workspace struct {
		CurrentDir string `json:"current_dir"`
	} `json:"workspace"`
	ContextWindow struct {
		ContextWindowSize int `json:"context_window_size"`
		CurrentUsage      *struct {
			InputTokens         int `json:"input_tokens"`
			CacheCreationTokens int `json:"cache_creation_input_tokens"`
			CacheReadTokens     int `json:"cache_read_input_tokens"`
		} `json:"current_usage"`
	} `json:"context_window"`
}

// ─────────────────────────────────────────────────
// Git Types
// ─────────────────────────────────────────────────

type GitStatus struct {
	Branch     string
	Ahead      int
	Behind     int
	Staged     int
	Modified   int
	Untracked  int
	Deleted    int
	Stashed    int
	Renamed    int
	Conflicted int
}

// ─────────────────────────────────────────────────
// Usage API Types
// ─────────────────────────────────────────────────

type UsageResponse struct {
	FiveHour *UsagePeriod `json:"five_hour"`
	SevenDay *UsagePeriod `json:"seven_day"`
}

type UsagePeriod struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at"`
}

type CachedUsage struct {
	FetchedAt int64         `json:"fetched_at"`
	Response  UsageResponse `json:"response"`
}

// ─────────────────────────────────────────────────
// Cache Configuration
// ─────────────────────────────────────────────────

const cacheTTL = 60 * time.Second

func getCacheDir() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "claude")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "claude")
}

func getCachePath() string {
	return filepath.Join(getCacheDir(), "usage-cache.json")
}

// ─────────────────────────────────────────────────
// Credentials
// ─────────────────────────────────────────────────

type Credentials struct {
	ClaudeAiOauth *struct {
		AccessToken string `json:"accessToken"`
	} `json:"claudeAiOauth"`
}

func getCredsPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		p := filepath.Join(xdg, "claude", ".credentials.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	home, _ := os.UserHomeDir()
	xdgPath := filepath.Join(home, ".config", "claude", ".credentials.json")
	if _, err := os.Stat(xdgPath); err == nil {
		return xdgPath
	}
	return filepath.Join(home, ".claude", ".credentials.json")
}

func getAccessToken() string {
	data, err := os.ReadFile(getCredsPath())
	if err != nil {
		return ""
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return ""
	}
	if creds.ClaudeAiOauth == nil {
		return ""
	}
	return creds.ClaudeAiOauth.AccessToken
}

// ─────────────────────────────────────────────────
// Color Helpers
// ─────────────────────────────────────────────────

func pctColor(pct int) string {
	switch {
	case pct < 10:
		return blue
	case pct < 20:
		return brBlue
	case pct < 30:
		return green
	case pct < 40:
		return brGreen
	case pct < 50:
		return brYello
	case pct < 60:
		return yellow
	case pct < 70:
		return red
	case pct < 90:
		return brRed
	default:
		return pink
	}
}

func progressIcon(pct int) string {
	idx := min(max(pct/10, 0), 9)
	return barProgress[idx]
}

func buildBar(filled int) string {
	var b strings.Builder
	for range filled {
		b.WriteString(barFilled)
	}
	for range 10 - filled {
		b.WriteString(barEmpty)
	}
	return b.String()
}

// ─────────────────────────────────────────────────
// Git Status
// ─────────────────────────────────────────────────
func getGitStatus(dir string) *GitStatus {
	// Check if directory is a git repo
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return nil
	}

	// Get porcelain v2 output with branch info and stash
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
			// Format: "# branch.ab +N -M"
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
			continue // Skip other headers

		default:
			// Entry lines: type XY ...
			if len(line) < 4 {
				continue
			}
			entryType := line[0]
			xy := line[2:4]

			switch entryType {
			case '1': // Tracked file changes
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
			case '2': // Renamed
				gs.Renamed++
			case 'u': // Unmerged (conflict)
				gs.Conflicted++
			case '?': // Untracked
				gs.Untracked++
			}
		}
	}

	return gs
}

// ─────────────────────────────────────────────────
// Usage API with Caching
// ─────────────────────────────────────────────────
func loadCache() (*CachedUsage, bool) {
	data, err := os.ReadFile(getCachePath())
	if err != nil {
		return nil, false
	}
	var cached CachedUsage
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, false
	}
	// Check TTL
	if time.Since(time.Unix(cached.FetchedAt, 0)) > cacheTTL {
		return nil, false
	}
	return &cached, true
}

func saveCache(resp *UsageResponse) {
	cached := CachedUsage{
		FetchedAt: time.Now().Unix(),
		Response:  *resp,
	}
	data, err := json.Marshal(cached)
	if err != nil {
		return
	}
	// Ensure cache directory exists
	os.MkdirAll(getCacheDir(), 0755)
	os.WriteFile(getCachePath(), data, 0644)
}

func fetchUsageFromAPI() *UsageResponse {
	token := getAccessToken()
	if token == "" {
		return nil
	}

	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("GET", "https://api.anthropic.com/api/oauth/usage", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var usage UsageResponse
	if err := json.Unmarshal(body, &usage); err != nil {
		return nil
	}

	return &usage
}

func getUsageLimits() *UsageResponse {
	// Try cache first
	if cached, ok := loadCache(); ok {
		return &cached.Response
	}
	// Fetch from API
	resp := fetchUsageFromAPI()
	if resp != nil {
		saveCache(resp)
	}
	return resp
}

// ─────────────────────────────────────────────────
// Date Formatting
// ─────────────────────────────────────────────────
func formatResetTime(isoTime string) string {
	if isoTime == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, isoTime)
	if err != nil {
		return ""
	}
	// Format: "3pm" style (hour + am/pm)
	hour := t.Local().Hour()
	suffix := "am"
	if hour >= 12 {
		suffix = "pm"
		if hour > 12 {
			hour -= 12
		}
	}
	if hour == 0 {
		hour = 12
	}
	return fmt.Sprintf(" (%d%s)", hour, suffix)
}

func formatResetDate(isoTime string) string {
	if isoTime == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, isoTime)
	if err != nil {
		return ""
	}
	// Format: "1/24" style (month/day)
	return fmt.Sprintf(" (%d/%d)", t.Local().Month(), t.Local().Day())
}

// ─────────────────────────────────────────────────
// Output Formatting
// ─────────────────────────────────────────────────
func formatGitStat(color, icon string, count int) string {
	if count <= 0 {
		return ""
	}
	return fmt.Sprintf(" %s%s%d%s", color, icon, count, reset)
}

func main() {
	// Read JSON input from stdin
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

	// Parallel fetch: git status and usage API
	var wg sync.WaitGroup
	var gitStatus *GitStatus
	var usageResp *UsageResponse

	wg.Add(2)
	go func() {
		defer wg.Done()
		gitStatus = getGitStatus(dir)
	}()
	go func() {
		defer wg.Done()
		usageResp = getUsageLimits()
	}()
	wg.Wait()

	// Build output
	var out strings.Builder

	// Directory (with home replaced by empty - matches bash behavior)
	displayDir := dir
	if after, found := strings.CutPrefix(dir, home+"/"); found {
		displayDir = after
	} else if dir == home {
		displayDir = ""
	}
	out.WriteString(red + iconModel + bold + red + displayDir + reset)

	// Git info
	if gitStatus != nil && gitStatus.Branch != "" {
		out.WriteString(yellow + " " + gitBranch + gitStatus.Branch + reset)
		out.WriteString(formatGitStat(green, gitAhead, gitStatus.Ahead))
		out.WriteString(formatGitStat(brRed, gitBehind, gitStatus.Behind))
		out.WriteString(formatGitStat(yellow, gitStaged, gitStatus.Staged))
		out.WriteString(formatGitStat(cyan, gitModified, gitStatus.Modified))
		out.WriteString(formatGitStat(brYello, gitUntracked, gitStatus.Untracked))
		out.WriteString(formatGitStat(red, gitDeleted, gitStatus.Deleted))
		out.WriteString(formatGitStat(gray, gitStashed, gitStatus.Stashed))
		out.WriteString(formatGitStat(magenta, gitRenamed, gitStatus.Renamed))
		out.WriteString(formatGitStat(pink, gitConflict, gitStatus.Conflicted))
	}

	// Context window bar
	if input.ContextWindow.CurrentUsage != nil && input.ContextWindow.ContextWindowSize > 0 {
		usage := input.ContextWindow.CurrentUsage
		total := usage.InputTokens + usage.CacheCreationTokens + usage.CacheReadTokens
		size := input.ContextWindow.ContextWindowSize
		pct := total * 100 / size
		color := pctColor(pct)
		bar := buildBar(pct / 10)
		out.WriteString(brBlue + sep + reset)
		fmt.Fprintf(&out, "%s%s%s %d%%%s", color, barContext, bar, pct, reset)
	}

	// Usage limits
	if usageResp != nil {
		hasOutput := false

		if usageResp.FiveHour != nil {
			pct := int(usageResp.FiveHour.Utilization)
			color := pctColor(pct)
			icon := progressIcon(pct)
			resetTime := formatResetTime(usageResp.FiveHour.ResetsAt)
			out.WriteString(brBlue + sep + reset)
			fmt.Fprintf(&out, "%s%s %d%%%s%s", color, icon, pct, resetTime, reset)
			hasOutput = true
		}

		if usageResp.SevenDay != nil {
			pct := int(usageResp.SevenDay.Utilization)
			color := pctColor(pct)
			icon := progressIcon(pct)
			resetDate := formatResetDate(usageResp.SevenDay.ResetsAt)
			if hasOutput {
				out.WriteString(blue + sep + reset)
			} else {
				out.WriteString(brBlue + sep + reset)
			}
			fmt.Fprintf(&out, "%s%s %d%%%s%s", color, icon, pct, resetDate, reset)
		}
	}

	// Final reset and output
	fmt.Print(out.String() + reset)
}
