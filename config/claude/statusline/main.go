// Package main implements the Claude Code custom statusline renderer.
//
// Reads JSON from stdin (the Claude Code statusline hook) and prints an ANSI-colored Nerd Font string to stdout.
// Segments are delimited by ╼╾ and cover: working directory, git state, context usage, and 5h/7d rate gauges.
// Git status and the usage-API call run concurrently; API responses cache to disk with a 5-minute positive TTL.
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
	SessionID     string `json:"session_id"`
	ContextWindow struct {
		ContextWindowSize int `json:"context_window_size"`
		CurrentUsage      *struct {
			InputTokens         int `json:"input_tokens"`
			CacheCreationTokens int `json:"cache_creation_input_tokens"`
			CacheReadTokens     int `json:"cache_read_input_tokens"`
		} `json:"current_usage"`
	} `json:"context_window"`
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

// UsageResponse is the shape returned by the Anthropic OAuth usage endpoint.
//
// Either period may be nil if the account has no data for that window.
type UsageResponse struct {
	FiveHour *UsagePeriod `json:"five_hour"`
	SevenDay *UsagePeriod `json:"seven_day"`
}

// UsagePeriod is a single rate-limit window.
type UsagePeriod struct {
	Utilization float64 `json:"utilization"` // percent, 0-100+
	ResetsAt    string  `json:"resets_at"`   // RFC3339
}

// CachedUsage is the on-disk cache envelope for the usage API.
//
// A nil Response with a non-empty Error is a negative cache entry.
// Expiry is tiered: cacheTTL for success, rateLimitCacheTTL for 429s, negCacheTTL for other errors.
type CachedUsage struct {
	FetchedAt int64          `json:"fetched_at"`
	Response  *UsageResponse `json:"response"`
	Error     string         `json:"error,omitempty"`
}

// RequestStats tracks how many usage-API calls have been made today.
//
// Surfaced next to error messages so repeated failures are visible.
type RequestStats struct {
	FirstRequest int64 `json:"first_request"` // unix seconds of first request this day
	Count        int   `json:"count"`
}

// ╭────────────────────────────────────────────────────────────────────────────╮
// │ Cache and credentials                                                      │
// ╰────────────────────────────────────────────────────────────────────────────╯

// Disk-cache TTLs balance liveness against hitting the API on every prompt render.
const (
	cacheTTL          = 5 * time.Minute
	negCacheTTL       = 60 * time.Second // transient errors (timeout, etc.)
	rateLimitCacheTTL = 10 * time.Minute // back off hard on 429
)

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

// OAuthData is the persisted Claude OAuth credential blob.
//
// Matches the shape Claude Code writes to the credentials file and the macOS keychain entry.
type OAuthData struct {
	AccessToken      string   `json:"accessToken"`
	RefreshToken     string   `json:"refreshToken"`
	ExpiresAt        int64    `json:"expiresAt"` // unix milliseconds
	Scopes           []string `json:"scopes"`
	SubscriptionType string   `json:"subscriptionType,omitempty"`
	RateLimitTier    string   `json:"rateLimitTier,omitempty"`
}

// Credentials is the top-level JSON wrapper stored at the credentials path.
type Credentials struct {
	ClaudeAiOauth *OAuthData `json:"claudeAiOauth"`
}

const (
	oauthTokenURL = "https://platform.claude.com/v1/oauth/token"
	oauthClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	// Refresh slightly before actual expiry to avoid racing a request against the cutover.
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

// findKeychainServices returns every macOS keychain service name matching "Claude Code-credentials*".
//
// Multiple entries can exist (e.g. per profile); getAccessTokenMacOS picks the one with the latest expiry.
// Returns nil if `security dump-keychain` fails or no entry matches.
func findKeychainServices() []string {
	cmd := exec.Command("security", "dump-keychain")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	const prefix = "Claude Code-credentials"
	var services []string
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		// dump-keychain format: `"svce"<blob>="<service name>"`
		if !strings.HasPrefix(line, `"svce"<blob>="`) {
			continue
		}
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

// getAccessToken returns a usable OAuth access token, or "" if none is available.
//
// macOS reads the keychain and refreshes on expiry; Linux reads the credentials file without refreshing.
func getAccessToken() string {
	if runtime.GOOS == "darwin" {
		return getAccessTokenMacOS()
	}
	data, err := os.ReadFile(getCredsPath())
	if err != nil {
		return ""
	}
	return parseAccessToken(data)
}

// isTokenExpired reports whether expiresAt (unix millis) is within expiryBuffer of now.
func isTokenExpired(expiresAt int64) bool {
	expiryTime := time.UnixMilli(expiresAt)
	return time.Now().Add(expiryBuffer).After(expiryTime)
}

// refreshToken exchanges the OAuth refresh token for a fresh access token.
//
// 5s HTTP timeout so a hanging auth server can't stall the render.
// Returns an error on transport failures or non-200 responses; the caller persists the new token.
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

// updateKeychainEntry writes refreshed credentials back to the keychain for svc.
//
// The account name is recovered from the existing entry because `security add-generic-password -U` requires -a and -s.
// Silently no-ops on any error to keep the renderer best-effort.
func updateKeychainEntry(svc string, creds *Credentials) {
	data, err := json.Marshal(creds)
	if err != nil {
		return
	}

	cmd := exec.Command("security", "find-generic-password", "-s", svc)
	out, err := cmd.Output()
	if err != nil {
		return
	}
	// find-generic-password format: `"acct"<blob>="<account>"`
	account := ""
	for line := range strings.SplitSeq(string(out), "\n") {
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

	cmd = exec.Command("security", "add-generic-password", "-U",
		"-a", account, "-s", svc, "-w", string(data))
	cmd.Run()
}

type keychainEntry struct {
	service string
	creds   Credentials
}

// getAccessTokenMacOS reads every Claude keychain entry and returns the access token from the latest-expiring one.
//
// Refreshes in place and writes the new token back to the keychain when expired.
// Returns "" when no entry exists, none parse, or refresh fails.
func getAccessTokenMacOS() string {
	services := findKeychainServices()
	if len(services) == 0 {
		return ""
	}

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

	// Pick the entry with the furthest-out expiry — most likely still valid.
	best := 0
	for i, e := range entries {
		if e.creds.ClaudeAiOauth.ExpiresAt > entries[best].creds.ClaudeAiOauth.ExpiresAt {
			best = i
		}
	}

	entry := &entries[best]
	oauth := entry.creds.ClaudeAiOauth

	if !isTokenExpired(oauth.ExpiresAt) {
		return oauth.AccessToken
	}

	if oauth.RefreshToken == "" {
		return ""
	}

	tokenResp, err := refreshToken(oauth)
	if err != nil {
		return ""
	}

	// Some refresh responses omit refresh_token; keep the existing one in that case.
	oauth.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		oauth.RefreshToken = tokenResp.RefreshToken
	}
	oauth.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UnixMilli()

	updateKeychainEntry(entry.service, &entry.creds)

	return oauth.AccessToken
}

// parseAccessToken extracts a non-expired access token from the Linux credentials file contents.
//
// Returns "" if the blob is malformed, empty, or within expiryBuffer of expiration.
// Linux has no refresh path here — the caller must re-auth out of band.
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

func claudeConfigDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "claude")
}

// scanCmdlineForEffort reads /proc/<pid>/cmdline and returns the --effort value.
//
// Handles both "--effort X" and "--effort=X" forms.
// The exe name must contain "claude" so an unrelated parent (e.g. a shell) can't match by accident.
// Returns "" when the file is unreadable, the exe doesn't match, or the flag is absent.
// Linux-only: /proc/<pid>/cmdline doesn't exist on macOS.
func scanCmdlineForEffort(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return ""
	}
	args := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	if len(args) == 0 {
		return ""
	}
	exe := filepath.Base(args[0])
	if !strings.Contains(exe, "claude") {
		return ""
	}
	for i, a := range args {
		if a == "--effort" && i+1 < len(args) {
			return args[i+1]
		}
		if after, ok := strings.CutPrefix(a, "--effort="); ok {
			return after
		}
	}
	return ""
}

// pidForSession locates the claude PID owning sessionID by scanning $CLAUDE_CONFIG_DIR/sessions.
//
// Fallback path for when os.Getppid() isn't the claude process (e.g. under a helper wrapper).
// Returns 0 when sessionID is empty, the glob fails, or no session file matches.
func pidForSession(sessionID string) int {
	if sessionID == "" {
		return 0
	}
	matches, err := filepath.Glob(filepath.Join(claudeConfigDir(), "sessions", "*.json"))
	if err != nil {
		return 0
	}
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		var s struct {
			PID       int    `json:"pid"`
			SessionID string `json:"sessionId"`
		}
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		if s.SessionID == sessionID && s.PID > 0 {
			return s.PID
		}
	}
	return 0
}

// readSettingsEffort returns the effortLevel key from $CLAUDE_CONFIG_DIR/settings.json.
//
// Last-resort default when neither the parent nor the session process exposes --effort on its cmdline.
// Returns "" on any read or parse failure.
func readSettingsEffort() string {
	data, err := os.ReadFile(filepath.Join(claudeConfigDir(), "settings.json"))
	if err != nil {
		return ""
	}
	var s struct {
		EffortLevel string `json:"effortLevel"`
	}
	if json.Unmarshal(data, &s) != nil {
		return ""
	}
	return s.EffortLevel
}

// getEffortLevel resolves the effort level from parent cmdline, then session-owning claude cmdline, then settings.json.
//
// Returns "" if no source yields a value.
func getEffortLevel(sessionID string) string {
	if lvl := scanCmdlineForEffort(os.Getppid()); lvl != "" {
		return lvl
	}
	if pid := pidForSession(sessionID); pid > 0 {
		if lvl := scanCmdlineForEffort(pid); lvl != "" {
			return lvl
		}
	}
	if lvl := readSettingsEffort(); lvl != "" {
		return lvl
	}
	return ""
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
// │ Usage API with caching                                                     │
// ╰────────────────────────────────────────────────────────────────────────────╯

// loadCache reads the on-disk cache and returns it if still within its TTL.
//
// TTL varies by result kind: cacheTTL for success, rateLimitCacheTTL for 429s, negCacheTTL otherwise.
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

// recordRequest increments the daily request counter and persists it.
//
// The counter resets at local midnight so error-badge stats stay interpretable ("4req/12m" today).
func recordRequest() RequestStats {
	var stats RequestStats
	data, err := os.ReadFile(getStatsPath())
	if err == nil {
		json.Unmarshal(data, &stats)
	}
	now := time.Now()
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

// saveCache persists a response (or a negative-cache entry with errMsg) to disk.
//
// Write errors are swallowed — a missing cache just triggers another fetch next render.
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

// fetchUsageFromAPI calls the Anthropic OAuth usage endpoint.
//
// Returns the parsed response or a short diagnostic: "no auth", "timeout", "rate limited", "auth expired", "http NNN".
// The error string is user-facing — it's what the statusline renders next to iconError.
// 2s client timeout so a slow API can't stall the render.
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

// getUsageLimits returns usage data, preferring the disk cache.
//
// Cache miss path: take a non-blocking flock so at most one render hits the API; others return empty.
// After acquiring the lock, re-check the cache in case a sibling just populated it.
func getUsageLimits() usageResult {
	if cached, ok := loadCache(); ok {
		return usageResult{resp: cached.Response, err: cached.Error}
	}

	os.MkdirAll(getCacheDir(), 0755)
	lockFile, err := os.OpenFile(getLockPath(), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		// Lock unavailable — fetch anyway; stale data beats no data.
		resp, errMsg := fetchUsageFromAPI()
		return usageResult{resp: resp, err: errMsg}
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		// Another render is fetching; yield rather than block prompt drawing.
		return usageResult{}
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	// Double-check: a sibling may have written the cache while we waited.
	if cached, ok := loadCache(); ok {
		return usageResult{resp: cached.Response, err: cached.Error}
	}

	resp, errMsg := fetchUsageFromAPI()
	saveCache(resp, errMsg)
	return usageResult{resp: resp, err: errMsg}
}

// formatResetTime renders an RFC3339 timestamp as " (3pm)" in local time for the 5-hour window reset.
//
// Returns "" if parsing fails.
func formatResetTime(isoTime string) string {
	if isoTime == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, isoTime)
	if err != nil {
		return ""
	}
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

// formatResetDate renders an RFC3339 timestamp as " (M/D)" in local time for the 7-day window reset.
//
// Returns "" if parsing fails.
func formatResetDate(isoTime string) string {
	if isoTime == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, isoTime)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(" (%d/%d)", t.Local().Month(), t.Local().Day())
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

	// Fan out git and usage lookups — both are I/O-bound and independent.
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
		name := input.Model.DisplayName
		// DisplayName often includes a parenthetical (e.g. "Sonnet 4.5 (1M)"); trim it.
		if i := strings.Index(name, " ("); i >= 0 {
			name = name[:i]
		}
		level := getEffortLevel(input.SessionID)
		icon, color := effortIcon(level)
		out.WriteString(gray + sep + reset)
		fmt.Fprintf(&out, "%s%s %s%s", color, icon, name, reset)
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

	fmt.Print(out.String() + reset)
}
