// Claude Code custom statusline renderer.
//
// Reads JSON from stdin (via Claude Code's statusline hook) and prints an
// ANSI-colored, Nerd Font status string to stdout. Segments are separated
// by box-drawing delimiters (╼╾) and include: working directory, git branch
// and file-status counts, context window usage bar, and rate-limit gauges
// (5-hour / 7-day) from the Anthropic usage API. Git status and API calls
// run concurrently; API responses are cached to disk with a 60s TTL.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
	iconError    = "󰅜"
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
	FetchedAt int64          `json:"fetched_at"`
	Response  *UsageResponse `json:"response"`  // nil when API failed (negative cache)
	Error     string         `json:"error,omitempty"` // reason for failure
}

// Request stats for debugging rate limits
type RequestStats struct {
	FirstRequest int64 `json:"first_request"`
	Count        int   `json:"count"`
}

// ─────────────────────────────────────────────────
// Cache Configuration
// ─────────────────────────────────────────────────

const cacheTTL = 5 * time.Minute
const negCacheTTL = 60 * time.Second       // transient errors (timeout, etc)
const rateLimitCacheTTL = 10 * time.Minute // back off hard on 429

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

type OAuthData struct {
	AccessToken      string   `json:"accessToken"`
	RefreshToken     string   `json:"refreshToken"`
	ExpiresAt        int64    `json:"expiresAt"`
	Scopes           []string `json:"scopes"`
	SubscriptionType string   `json:"subscriptionType,omitempty"`
	RateLimitTier    string   `json:"rateLimitTier,omitempty"`
}

type Credentials struct {
	ClaudeAiOauth *OAuthData `json:"claudeAiOauth"`
}

// OAuth refresh constants
const (
	oauthTokenURL = "https://platform.claude.com/v1/oauth/token"
	oauthClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	// Refresh 5 minutes before actual expiry to avoid races
	expiryBuffer = 5 * time.Minute
)

type TokenRefreshRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
	Scope        string `json:"scope"`
}

type TokenRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
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

// findKeychainServices returns all keychain service names matching "Claude Code-credentials*".
func findKeychainServices() []string {
	cmd := exec.Command("security", "dump-keychain")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	const prefix = "Claude Code-credentials"
	var services []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// Match: "svce"<blob>="Claude Code-credentials..."
		if !strings.HasPrefix(line, `"svce"<blob>="`) {
			continue
		}
		// Extract value between quotes
		start := strings.Index(line, `="`) + 2
		end := strings.LastIndex(line, `"`)
		if start < 2 || end <= start {
			continue
		}
		svc := line[start:end]
		if strings.HasPrefix(svc, prefix) && !seen[svc] {
			seen[svc] = true
			services = append(services, svc)
		}
	}
	return services
}

func getAccessToken() string {
	if runtime.GOOS == "darwin" {
		return getAccessTokenMacOS()
	}
	// Linux: read from file
	data, err := os.ReadFile(getCredsPath())
	if err != nil {
		return ""
	}
	return parseAccessToken(data)
}

func isTokenExpired(expiresAt int64) bool {
	expiryTime := time.UnixMilli(expiresAt)
	return time.Now().Add(expiryBuffer).After(expiryTime)
}

func refreshToken(oauth *OAuthData) (*TokenRefreshResponse, error) {
	scope := strings.Join(oauth.Scopes, " ")
	if scope == "" {
		scope = "user:inference user:profile user:mcp_servers user:sessions:claude_code"
	}

	reqBody := TokenRefreshRequest{
		GrantType:    "refresh_token",
		RefreshToken: oauth.RefreshToken,
		ClientID:     oauthClientID,
		Scope:        scope,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("POST", oauthTokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("refresh failed: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokenResp TokenRefreshResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, err
	}
	return &tokenResp, nil
}

func updateKeychainEntry(svc string, creds *Credentials) {
	data, err := json.Marshal(creds)
	if err != nil {
		return
	}

	// Get the account name from the existing entry
	cmd := exec.Command("security", "find-generic-password", "-s", svc)
	out, err := cmd.Output()
	if err != nil {
		return
	}
	// Parse account name from output: "acct"<blob>="..."
	account := ""
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `"acct"<blob>="`) {
			start := strings.Index(line, `="`) + 2
			end := strings.LastIndex(line, `"`)
			if start >= 2 && end > start {
				account = line[start:end]
			}
		}
	}
	if account == "" {
		return
	}

	// Update the keychain entry
	cmd = exec.Command("security", "add-generic-password", "-U",
		"-a", account, "-s", svc, "-w", string(data))
	cmd.Run()
}

type keychainEntry struct {
	service string
	creds   Credentials
}

func getAccessTokenMacOS() string {
	services := findKeychainServices()
	if len(services) == 0 {
		return ""
	}

	// Collect all entries
	var entries []keychainEntry
	for _, svc := range services {
		cmd := exec.Command("security", "find-generic-password", "-s", svc, "-w")
		data, err := cmd.Output()
		if err != nil {
			continue
		}
		data = []byte(strings.TrimSpace(string(data)))
		var creds Credentials
		if err := json.Unmarshal(data, &creds); err != nil || creds.ClaudeAiOauth == nil {
			continue
		}
		entries = append(entries, keychainEntry{service: svc, creds: creds})
	}

	if len(entries) == 0 {
		return ""
	}

	// Find the entry with the latest expiry
	best := 0
	for i, e := range entries {
		if e.creds.ClaudeAiOauth.ExpiresAt > entries[best].creds.ClaudeAiOauth.ExpiresAt {
			best = i
		}
	}

	entry := &entries[best]
	oauth := entry.creds.ClaudeAiOauth

	// If not expired, use it directly
	if !isTokenExpired(oauth.ExpiresAt) {
		return oauth.AccessToken
	}

	// Token expired — try to refresh
	if oauth.RefreshToken == "" {
		return ""
	}

	tokenResp, err := refreshToken(oauth)
	if err != nil {
		return ""
	}

	// Update credentials and persist to keychain
	oauth.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		oauth.RefreshToken = tokenResp.RefreshToken
	}
	oauth.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UnixMilli()

	updateKeychainEntry(entry.service, &entry.creds)

	return oauth.AccessToken
}

func parseAccessToken(data []byte) string {
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return ""
	}
	if creds.ClaudeAiOauth == nil {
		return ""
	}
	if isTokenExpired(creds.ClaudeAiOauth.ExpiresAt) {
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
	age := time.Since(time.Unix(cached.FetchedAt, 0))
	ttl := cacheTTL
	if cached.Response == nil {
		if cached.Error == "rate limited" {
			ttl = rateLimitCacheTTL
		} else {
			ttl = negCacheTTL
		}
	}
	if age > ttl {
		return nil, false
	}
	return &cached, true
}

func getStatsPath() string {
	return filepath.Join(getCacheDir(), "usage-stats.json")
}

func recordRequest() RequestStats {
	var stats RequestStats
	data, err := os.ReadFile(getStatsPath())
	if err == nil {
		json.Unmarshal(data, &stats)
	}
	now := time.Now()
	// Reset daily: if first request was on a different day, start fresh
	if stats.FirstRequest != 0 {
		first := time.Unix(stats.FirstRequest, 0)
		y1, m1, d1 := first.Date()
		y2, m2, d2 := now.Date()
		if y1 != y2 || m1 != m2 || d1 != d2 {
			stats = RequestStats{}
		}
	}
	if stats.FirstRequest == 0 {
		stats.FirstRequest = now.Unix()
	}
	stats.Count++
	if out, err := json.Marshal(stats); err == nil {
		os.WriteFile(getStatsPath(), out, 0644)
	}
	return stats
}

func getRequestStats() RequestStats {
	var stats RequestStats
	data, err := os.ReadFile(getStatsPath())
	if err == nil {
		json.Unmarshal(data, &stats)
	}
	return stats
}

func formatStatsDuration(stats RequestStats) string {
	if stats.FirstRequest == 0 || stats.Count == 0 {
		return ""
	}
	elapsed := time.Since(time.Unix(stats.FirstRequest, 0))
	if elapsed < time.Minute {
		return fmt.Sprintf("%dreq/%ds", stats.Count, int(elapsed.Seconds()))
	}
	return fmt.Sprintf("%dreq/%dm", stats.Count, int(elapsed.Minutes()))
}

// saveCache persists a result (or nil for negative cache) to disk.
func saveCache(resp *UsageResponse, errMsg string) {
	cached := CachedUsage{
		FetchedAt: time.Now().Unix(),
		Response:  resp,
		Error:     errMsg,
	}
	data, err := json.Marshal(cached)
	if err != nil {
		return
	}
	os.MkdirAll(getCacheDir(), 0755)
	os.WriteFile(getCachePath(), data, 0644)
}

func fetchUsageFromAPI() (*UsageResponse, string) {
	token := getAccessToken()
	if token == "" {
		return nil, "no auth"
	}

	recordRequest()

	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("GET", "https://api.anthropic.com/api/oauth/usage", nil)
	if err != nil {
		return nil, ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "timeout"
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, "rate limited"
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, "auth expired"
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Sprintf("http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ""
	}

	var usage UsageResponse
	if err := json.Unmarshal(body, &usage); err != nil {
		return nil, ""
	}

	return &usage, ""
}

type usageResult struct {
	resp *UsageResponse
	err  string
}

func getLockPath() string {
	return filepath.Join(getCacheDir(), "usage-cache.lock")
}

func getUsageLimits() usageResult {
	// Try cache first — no lock needed for reads
	if cached, ok := loadCache(); ok {
		return usageResult{resp: cached.Response, err: cached.Error}
	}

	// Cache miss — acquire exclusive lock so only one instance fetches
	os.MkdirAll(getCacheDir(), 0755)
	lockFile, err := os.OpenFile(getLockPath(), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		// Can't lock, just fetch (better than no data)
		resp, errMsg := fetchUsageFromAPI()
		return usageResult{resp: resp, err: errMsg}
	}
	defer lockFile.Close()

	// Non-blocking try: if another instance holds the lock, return stale/empty
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		// Another instance is fetching — return empty rather than block the render
		return usageResult{}
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	// Re-check cache after acquiring lock (another instance may have just written it)
	if cached, ok := loadCache(); ok {
		return usageResult{resp: cached.Response, err: cached.Error}
	}

	// We hold the lock and cache is stale — fetch
	resp, errMsg := fetchUsageFromAPI()
	saveCache(resp, errMsg)
	return usageResult{resp: resp, err: errMsg}
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
	var usage usageResult

	wg.Add(2)
	go func() {
		defer wg.Done()
		gitStatus = getGitStatus(dir)
	}()
	go func() {
		defer wg.Done()
		usage = getUsageLimits()
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
		out.WriteString(" " + yellow + gitBranch + gitStatus.Branch + reset)
		out.WriteString(buildGitStats(gitStatus))
	}

	// Context window bar
	if input.ContextWindow.CurrentUsage != nil && input.ContextWindow.ContextWindowSize > 0 {
		usage := input.ContextWindow.CurrentUsage
		total := usage.InputTokens + usage.CacheCreationTokens + usage.CacheReadTokens
		size := input.ContextWindow.ContextWindowSize
		pct := total * 100 / size
		color := pctColor(pct)
		bar := buildBar(pct / 10)
		out.WriteString(gray + sep + reset)
		fmt.Fprintf(&out, "%s%s%s %d%%%s", color, barContext, bar, pct, reset)
	}

	// Usage limits
	if usage.resp != nil {
		if usage.resp.FiveHour != nil {
			pct := int(usage.resp.FiveHour.Utilization)
			color := pctColor(pct)
			icon := progressIcon(pct)
			resetTime := formatResetTime(usage.resp.FiveHour.ResetsAt)
			out.WriteString(gray + sep + reset)
			fmt.Fprintf(&out, "%s%s %d%%%s%s", color, icon, pct, resetTime, reset)
		}

		if usage.resp.SevenDay != nil {
			pct := int(usage.resp.SevenDay.Utilization)
			color := pctColor(pct)
			icon := progressIcon(pct)
			resetDate := formatResetDate(usage.resp.SevenDay.ResetsAt)
			out.WriteString(gray + sep + reset)
			fmt.Fprintf(&out, "%s%s %d%%%s%s", color, icon, pct, resetDate, reset)
		}
	} else if usage.err != "" {
		stats := getRequestStats()
		statStr := formatStatsDuration(stats)
		out.WriteString(gray + sep + reset)
		if statStr != "" {
			fmt.Fprintf(&out, "%s%s %s %s(%s)%s", brRed, iconError, usage.err, gray, statStr, reset)
		} else {
			fmt.Fprintf(&out, "%s%s %s%s", brRed, iconError, usage.err, reset)
		}
	}

	// Final reset and output
	fmt.Print(out.String() + reset)
}
