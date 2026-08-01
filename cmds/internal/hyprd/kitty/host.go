package kitty

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"dotfiles/cmds/internal/config"
)

// errRequirementUnsatisfied means the probe ran successfully and the requirement is absent.
// Transport, execution, and parsing failures must not use this sentinel.
var errRequirementUnsatisfied = errors.New("requirement not satisfied")

var hostAliases = []string{"abbott", "costello", "neumann"}

const (
	userVarHost         = "hyprd_host"
	userVarCWD          = "hyprd_cwd"
	userVarPane         = "hyprd_pane"
	userVarTxn          = "hyprd_txn"
	userVarObservedHost = "hyprd_observed_host"
)

type hostTab struct {
	source      Tab
	definition  config.TabDef
	name        string
	logicalCWDs []string
	targetCWDs  []string
	layoutName  string
	layoutOpts  map[string]string // complete normalized source layout_opts (verify truth)
	layoutGoto  string            // valid goto-layout spec derived from source + enabled_layouts
	focusedPane int
	stageID     int
	stagePanes  []int
}

type hostPlan struct {
	profile    *config.TabProfile
	controller string
	source     string
	target     string
	window     OSWindow
	tabs       []hostTab
	focusedTab int
	txn        string
}

// hostOrigin names the exact Kitty OS window a host switch must mutate.
//
// The chooser kitten captures both values inside the Kitty process before dispatch, so a focus change between the chooser and the switch cannot retarget another window.
// A zero origin means "resolve the currently focused Kitty OS window".
type hostOrigin struct {
	pid      int // Kitty process PID; also selects its remote-control socket
	windowID int // Kitty OS window id
}

// hostTarget is the resolved and validated Kitty OS window a switch will mutate.
type hostTarget struct {
	origin hostOrigin
	kitty  *Client
	window OSWindow
}

const hostUsage = "usage: tabs host <abbott|costello|neumann> [--kitty-pid <pid> --os-window <id>]"

func (t *Manager) host(args []string) (result string, err error) {
	alias, origin, err := parseHostArgs(args)
	if err != nil {
		return "", err
	}

	target, err := t.resolveHostTarget(origin)
	if err != nil {
		return "", err
	}
	kitty, window := target.kitty, target.window

	if !t.beginHostSwitch(target.origin) {
		err = fmt.Errorf("a host switch is already running for this Kitty OS window")
		t.notifyHost(kitty, "Host switch blocked", err.Error(), true)
		return "", err
	}
	defer t.endHostSwitch(target.origin)

	plan, err := t.prepareHostPlan(window, alias)
	if err != nil {
		t.notifyHost(kitty, "Host switch failed", err.Error(), true)
		return "", err
	}
	if plan.source == plan.target {
		msg := fmt.Sprintf("Already on %s; no tabs changed", plan.source)
		t.notifyHost(kitty, "Host switch unchanged", msg, false)
		return msg, nil
	}

	// Complete strict preflight before any Kitty remote-control mutation (including notify).
	if err := t.preflightHostPlan(plan); err != nil {
		err = fmt.Errorf("preflight %s: %w", plan.target, err)
		t.notifyHost(kitty, "Host switch failed", err.Error(), true)
		return "", err
	}
	t.notifyHost(kitty, "Switching Kitty host", fmt.Sprintf("%s → %s", plan.source, plan.target), false)

	staged := false
	defer func() {
		if err == nil || !staged {
			return
		}
		if rollbackErr := rollbackHostTabs(kitty, plan); rollbackErr != nil {
			err = fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
			t.notifyHost(kitty, "Host switch rollback failed", err.Error(), true)
			return
		}
		t.notifyHost(kitty, "Host switch rolled back", fmt.Sprintf("%v; source tabs were preserved", err), true)
	}()

	staged = true
	if err = stageHostTabs(kitty, plan); err != nil {
		return "", err
	}
	if err = verifyHostTabs(kitty, plan); err != nil {
		return "", fmt.Errorf("verify staged tabs: %w", err)
	}

	focus := &plan.tabs[plan.focusedTab]
	if err = kitty.focusTabByNumericID(focus.stageID); err != nil {
		return "", fmt.Errorf("focus replacement tab: %w", err)
	}
	if focus.focusedPane >= 0 && focus.focusedPane < len(focus.stagePanes) {
		if err = kitty.FocusWindow(focus.stagePanes[focus.focusedPane]); err != nil {
			return "", fmt.Errorf("focus replacement pane: %w", err)
		}
	}

	sourceIDs := make([]int, len(plan.tabs))
	for i := range plan.tabs {
		sourceIDs[i] = plan.tabs[i].source.ID
	}
	if err = kitty.closeTabsByNumericIDs(sourceIDs); err != nil {
		return "", fmt.Errorf("close source tabs: %w", err)
	}
	staged = false

	result = fmt.Sprintf("tabs host: %s → %s (%d tabs)", plan.source, plan.target, len(plan.tabs))
	t.notifyHost(kitty, "Kitty host switched", result, false)
	return result, nil
}

// parseHostArgs accepts one host alias plus an optional explicit OS-window identity.
// The two identity flags are useless apart, so they must be given together.
func parseHostArgs(args []string) (string, hostOrigin, error) {
	var (
		alias  string
		origin hostOrigin
	)
	for i := 0; i < len(args); i++ {
		flag := args[i]
		switch flag {
		case "--kitty-pid", "--os-window":
			if i+1 >= len(args) {
				return "", origin, fmt.Errorf("%s needs a value; %s", flag, hostUsage)
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil || value <= 0 {
				return "", origin, fmt.Errorf("%s needs a positive integer, got %q", flag, args[i+1])
			}
			if flag == "--kitty-pid" {
				origin.pid = value
			} else {
				origin.windowID = value
			}
			i++
		default:
			if alias != "" || !slices.Contains(hostAliases, flag) {
				return "", origin, fmt.Errorf("%s", hostUsage)
			}
			alias = flag
		}
	}
	if alias == "" {
		return "", origin, fmt.Errorf("%s", hostUsage)
	}
	if (origin.pid == 0) != (origin.windowID == 0) {
		return "", origin, fmt.Errorf("--kitty-pid and --os-window must be given together; %s", hostUsage)
	}
	return alias, origin, nil
}

// resolveHostTarget validates that the captured OS window still exists inside the captured Kitty process.
// Without an explicit identity it falls back to the focused Kitty window, which is what a manual `hyprd tabs host <alias>` expects.
func (t *Manager) resolveHostTarget(origin hostOrigin) (hostTarget, error) {
	if origin.pid == 0 {
		pid, err := t.activeKittyPID()
		if err != nil {
			return hostTarget{}, fmt.Errorf("host switch requires a focused Kitty window: %w", err)
		}
		origin.pid = pid
	}

	// The socket is per Kitty process, so any window found here belongs to that process.
	kitty := NewClient(origin.pid)
	windows, err := kitty.FullState()
	if err != nil {
		return hostTarget{}, fmt.Errorf("kitty state for pid %d: %w", origin.pid, err)
	}

	if origin.windowID == 0 {
		window, err := focusedOSWindow(windows)
		if err != nil {
			return hostTarget{}, err
		}
		origin.windowID = window.ID
		return hostTarget{origin: origin, kitty: kitty, window: window}, nil
	}

	window, ok := windowByID(windows, origin.windowID)
	if !ok {
		return hostTarget{}, fmt.Errorf("Kitty OS window %d no longer exists in Kitty process %d", origin.windowID, origin.pid)
	}
	return hostTarget{origin: origin, kitty: kitty, window: window}, nil
}

func (t *Manager) beginHostSwitch(origin hostOrigin) bool {
	t.hostMu.Lock()
	defer t.hostMu.Unlock()
	if t.hostSwitches[origin] {
		return false
	}
	t.hostSwitches[origin] = true
	return true
}

func (t *Manager) endHostSwitch(origin hostOrigin) {
	t.hostMu.Lock()
	delete(t.hostSwitches, origin)
	t.hostMu.Unlock()
}

func (t *Manager) prepareHostPlan(window OSWindow, target string) (*hostPlan, error) {
	controller, err := controllerHost()
	if err != nil {
		return nil, err
	}
	source, err := sourceHost(window, controller)
	if err != nil {
		return nil, err
	}
	_, profile, tabs, focused, err := strictHostProfile(t.state.GetConfig(), window, controller, source)
	if err != nil {
		return nil, err
	}
	if err := t.ensureCompleteHostProfile(profile, tabs, source); err != nil {
		return nil, err
	}
	txn, err := newHostTransaction(window.ID)
	if err != nil {
		return nil, err
	}
	return &hostPlan{profile: profile, controller: controller, source: source, target: target, window: window, tabs: tabs, focusedTab: focused, txn: txn}, nil
}

func strictHostProfile(cfg *config.HyprConfig, window OSWindow, controller, source string) (string, *config.TabProfile, []hostTab, int, error) {
	if cfg == nil || len(cfg.Tabs) == 0 || len(window.Tabs) == 0 {
		return "", nil, nil, -1, fmt.Errorf("focused window has no configured tab profile")
	}
	controllerHome, err := os.UserHomeDir()
	if err != nil {
		return "", nil, nil, -1, fmt.Errorf("controller HOME unavailable: %w", err)
	}
	if controllerHome == "" {
		return "", nil, nil, -1, fmt.Errorf("controller HOME unavailable")
	}
	type match struct {
		name    string
		profile config.TabProfile
		tabs    []hostTab
		focused int
	}
	var matches []match
	var metadataErr error
	for name, profile := range cfg.Tabs {
		seen := make(map[string]bool)
		candidate := make([]hostTab, 0, len(window.Tabs))
		focused := -1
		valid := true
		for _, tab := range window.Tabs {
			if len(tab.Windows) == 0 {
				valid = false
				break
			}
			id := tab.Windows[0].Env["KITTY_TAB_ID"]
			tabName := profileTabNameFromID(window.ID, profile.Prefix, id)
			definition := findProfileTab(&profile, tabName)
			if definition == nil || seen[tabName] || len(tab.Windows) != 1+len(definition.Panes) {
				valid = false
				break
			}
			for _, pane := range tab.Windows {
				if pane.Env["KITTY_TAB_ID"] != id {
					valid = false
					break
				}
			}
			if !valid {
				break
			}
			panes, paneErr := orderedProfilePanes(tab.Windows)
			if paneErr != nil {
				metadataErr = fmt.Errorf("tab %s: %w", tabName, paneErr)
				valid = false
				break
			}
			seen[tabName] = true
			layoutName, layoutOpts := sourceTabLayout(tab, *definition)
			entry := hostTab{
				source:      tab,
				definition:  *definition,
				name:        tabName,
				layoutName:  layoutName,
				layoutOpts:  layoutOpts,
				layoutGoto:  selectGotoLayout(layoutName, layoutOpts, tab.EnabledLayouts),
				focusedPane: -1,
			}
			for paneIndex, pane := range panes {
				logical, cwdErr := sourceLogicalCWD(pane, configuredPaneCWD(*definition, paneIndex), controllerHome, source == controller)
				if cwdErr != nil {
					metadataErr = fmt.Errorf("tab %s pane %d: %w", tabName, paneIndex, cwdErr)
					valid = false
					break
				}
				entry.logicalCWDs = append(entry.logicalCWDs, logical)
			}
			if !valid {
				break
			}
			paneFocus, paneErr := uniqueSelectedPane(panes)
			if paneErr != nil {
				return "", nil, nil, -1, fmt.Errorf("tab %s: %w", tabName, paneErr)
			}
			entry.focusedPane = paneFocus
			candidate = append(candidate, entry)
		}
		tabFocus, tabErr := uniqueSelectedTab(window.Tabs)
		if tabErr != nil {
			valid = false
		} else {
			// Map OS-window tab index onto the candidate slice (same order/length when valid).
			focused = tabFocus
		}
		if valid {
			matches = append(matches, match{name: name, profile: profile, tabs: candidate, focused: focused})
		}
	}
	if len(matches) != 1 {
		if metadataErr != nil {
			return "", nil, nil, -1, metadataErr
		}
		return "", nil, nil, -1, fmt.Errorf("focused window is unknown, ambiguous, or contains arbitrary/mixed profile tabs")
	}
	if matches[0].focused < 0 || matches[0].focused >= len(matches[0].tabs) {
		return "", nil, nil, -1, fmt.Errorf("focused profile tab is ambiguous")
	}
	return matches[0].name, &matches[0].profile, matches[0].tabs, matches[0].focused, nil
}

func profilePaneIndex(pane Pane, count int) (int, error) {
	raw, ok := pane.UserVars[userVarPane]
	if !ok || raw == "" {
		return -1, fmt.Errorf("missing %s metadata; refresh the tab before switching hosts", userVarPane)
	}
	index, err := strconv.Atoi(raw)
	if err != nil || index < 0 || index >= count || strconv.Itoa(index) != raw {
		return -1, fmt.Errorf("invalid %s value %q", userVarPane, raw)
	}
	return index, nil
}

func orderedProfilePanes(panes []Pane) ([]Pane, error) {
	if len(panes) == 1 {
		if _, ok := panes[0].UserVars[userVarPane]; !ok {
			return slices.Clone(panes), nil
		}
	}
	ordered := make([]Pane, len(panes))
	seen := make([]bool, len(panes))
	for _, pane := range panes {
		index, err := profilePaneIndex(pane, len(panes))
		if err != nil {
			return nil, err
		}
		if seen[index] {
			return nil, fmt.Errorf("duplicate %s index %d", userVarPane, index)
		}
		seen[index] = true
		ordered[index] = pane
	}
	for index, ok := range seen {
		if !ok {
			return nil, fmt.Errorf("missing %s index %d", userVarPane, index)
		}
	}
	return ordered, nil
}

func configuredPaneCWD(definition config.TabDef, paneIndex int) string {
	if paneIndex == 0 || paneIndex > len(definition.Panes) || definition.Panes[paneIndex-1].CWD == "" {
		return definition.CWD
	}
	return definition.Panes[paneIndex-1].CWD
}

func sourceLogicalCWD(pane Pane, configured, controllerHome string, local bool) (string, error) {
	if logical, ok := pane.UserVars[userVarCWD]; ok {
		return normalizeLogicalCWD(logical)
	}
	if _, reported := pane.UserVars[userVarObservedHost]; reported {
		return "", fmt.Errorf("prompt removed %s because the current directory is outside HOME; return under HOME before switching", userVarCWD)
	}
	if local {
		if pane.CWD == "" {
			return "", fmt.Errorf("missing %s and local process CWD", userVarCWD)
		}
		return logicalCWDFromPath(pane.CWD, controllerHome)
	}
	if configured != "" {
		return logicalCWDFromPath(config.ExpandPath(configured), controllerHome)
	}
	if project := pane.Env["PROJECT_PATH"]; project != "" {
		return logicalCWDFromPath(config.ExpandPath(project), controllerHome)
	}
	return "", fmt.Errorf("missing %s; remote CWD cannot be recovered safely from profile CWD or PROJECT_PATH, so refresh this window on its source host", userVarCWD)
}

func normalizeLogicalCWD(logical string) (string, error) {
	if logical == "." {
		return logical, nil
	}
	if logical == "" || strings.HasPrefix(logical, "/") || strings.Contains(logical, "\\") {
		return "", fmt.Errorf("invalid home-relative %s %q", userVarCWD, logical)
	}
	clean := path.Clean(logical)
	if clean != logical || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid home-relative %s %q", userVarCWD, logical)
	}
	return clean, nil
}

func logicalCWDFromPath(cwd, home string) (string, error) {
	if !filepath.IsAbs(cwd) {
		return "", fmt.Errorf("CWD %q is not absolute", cwd)
	}
	rel, err := filepath.Rel(home, filepath.Clean(cwd))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("CWD %q is outside controller HOME %q; no mapping is configured", cwd, home)
	}
	return normalizeLogicalCWD(filepath.ToSlash(rel))
}

// ensureCompleteHostProfile rejects profile fragments and unknown extras.
// Tabs with requires may be absent only when source-side requires checks fail.
func (t *Manager) ensureCompleteHostProfile(profile *config.TabProfile, present []hostTab, source string) error {
	if profile == nil {
		return fmt.Errorf("focused window has no configured tab profile")
	}
	have := make(map[string]bool, len(present))
	for _, tab := range present {
		if have[tab.name] {
			return fmt.Errorf("managed window contains duplicate profile tab %q", tab.name)
		}
		have[tab.name] = true
	}
	defaultLogical := "."
	if len(present) > 0 && len(present[0].logicalCWDs) > 0 {
		defaultLogical = present[0].logicalCWDs[0]
	}
	for _, def := range profile.Tabs {
		if have[def.Name] {
			continue
		}
		if def.Requires == "" {
			return fmt.Errorf("managed window missing required profile tab %q", def.Name)
		}
		logical, err := configuredLogicalCWD(def.CWD, defaultLogical)
		if err != nil {
			return fmt.Errorf("profile tab %q CWD: %w", def.Name, err)
		}
		cwd, err := hostPath(source, logical)
		if err != nil {
			return fmt.Errorf("profile tab %q CWD: %w", def.Name, err)
		}
		satisfied, err := hostRequiresSatisfied(source, def.Requires, cwd)
		if err != nil {
			return fmt.Errorf("profile tab %q requires %q: %w", def.Name, def.Requires, err)
		}
		if satisfied {
			return fmt.Errorf("managed window missing required profile tab %q", def.Name)
		}
	}
	return nil
}

func hostRequiresSatisfied(source, requires, cwd string) (bool, error) {
	if requires == "" {
		return true, nil
	}
	controller, err := controllerHost()
	if err != nil {
		return false, err
	}
	if err := checkHostRequirement(controller, source, requires, cwd); err != nil {
		// Only a successful negative probe may omit a conditional tab.
		if errors.Is(err, errRequirementUnsatisfied) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// uniqueSelectedTab requires exactly one tab marked selected; does not fall back or overwrite.
func uniqueSelectedTab(tabs []Tab) (int, error) {
	selected := -1
	for i, tab := range tabs {
		if !tabSelected(tab) {
			continue
		}
		if selected >= 0 {
			return -1, fmt.Errorf("focused profile tab is ambiguous")
		}
		selected = i
	}
	if selected < 0 {
		return -1, fmt.Errorf("focused profile tab is ambiguous")
	}
	return selected, nil
}

// uniqueSelectedPane requires a single selected pane when focus is required.
// A lone pane with no selection markers still resolves to index 0.
func uniqueSelectedPane(panes []Pane) (int, error) {
	selected := -1
	for i, pane := range panes {
		if !paneSelected(pane) {
			continue
		}
		if selected >= 0 {
			return -1, fmt.Errorf("focused pane is ambiguous")
		}
		selected = i
	}
	if selected >= 0 {
		return selected, nil
	}
	if len(panes) == 1 {
		return 0, nil
	}
	return -1, fmt.Errorf("focused pane is ambiguous")
}

// sourceTabLayout prefers live Kitty layout + layout_opts so manual geometry survives.
// Configured definition layout is only a fallback when the source tab has no layout name.
func sourceTabLayout(tab Tab, definition config.TabDef) (string, map[string]string) {
	if tab.Layout != "" {
		return tab.Layout, normalizeLayoutOpts(tab.LayoutOpts)
	}
	return parseLayoutSpec(definition.Layout)
}

// parseLayoutSpec parses a Kitty layout definition.
// Options after ':' are semicolon-separated key=value pairs (Kitty 0.48.1).
func parseLayoutSpec(spec string) (string, map[string]string) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", nil
	}
	name, rest, ok := strings.Cut(spec, ":")
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	if !ok || strings.TrimSpace(rest) == "" {
		return name, map[string]string{}
	}
	opts := make(map[string]string)
	for part := range strings.SplitSeq(rest, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, found := strings.Cut(part, "=")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !found || key == "" || value == "" {
			continue
		}
		opts[key] = value
	}
	return name, opts
}

func normalizeLayoutOpts(raw map[string]any) map[string]string {
	if len(raw) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(raw))
	for key, value := range raw {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		text, ok := layoutOptString(value)
		if !ok {
			continue
		}
		out[key] = text
	}
	return out
}

func layoutOptString(value any) (string, bool) {
	switch v := value.(type) {
	case nil:
		return "", false
	case string:
		return v, true
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "", false
		}
		if v == math.Trunc(v) && v >= math.MinInt64 && v <= math.MaxInt64 {
			return strconv.FormatInt(int64(v), 10), true
		}
		return strconv.FormatFloat(v, 'g', -1, 64), true
	case json.Number:
		return v.String(), true
	case int:
		return strconv.Itoa(v), true
	case int8:
		return strconv.FormatInt(int64(v), 10), true
	case int16:
		return strconv.FormatInt(int64(v), 10), true
	case int32:
		return strconv.FormatInt(int64(v), 10), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case uint:
		return strconv.FormatUint(uint64(v), 10), true
	case uint8:
		return strconv.FormatUint(uint64(v), 10), true
	case uint16:
		return strconv.FormatUint(uint64(v), 10), true
	case uint32:
		return strconv.FormatUint(uint64(v), 10), true
	case uint64:
		return strconv.FormatUint(v, 10), true
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		if text == "" {
			return "", false
		}
		return text, true
	}
}

// gotoLayoutSpec builds a deterministic Kitty layout spec.
// Option order is sorted by key; options are semicolon-separated (Kitty 0.48.1).
func gotoLayoutSpec(name string, opts map[string]string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if len(opts) == 0 {
		return name
	}
	keys := slices.Sorted(maps.Keys(opts))
	var b strings.Builder
	b.WriteString(name)
	b.WriteByte(':')
	for i, key := range keys {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(opts[key])
	}
	return b.String()
}

// selectGotoLayout chooses a Kitty-accepted goto-layout argument for the live source layout.
// Kitty matches against enabled_layouts (exact or unique prefix); a full layout_opts dump is
// often not itself an enabled name, so pick the consistent enabled entry when available.
// Verification still demands the complete normalized layout_opts from the source tab.
func selectGotoLayout(name string, opts map[string]string, enabled []string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	full := gotoLayoutSpec(name, opts)
	if len(enabled) == 0 {
		return full
	}

	var consistent []string
	for _, candidate := range enabled {
		candName, candOpts := parseLayoutSpec(candidate)
		if candName != name {
			continue
		}
		if layoutOptsContain(opts, candOpts) {
			consistent = append(consistent, candidate)
		}
	}
	if len(consistent) == 1 {
		return consistent[0]
	}
	if len(consistent) > 1 {
		// Prefer the most specific enabled definition (most declared options).
		best := consistent[0]
		_, bestOpts := parseLayoutSpec(best)
		for _, candidate := range consistent[1:] {
			_, candOpts := parseLayoutSpec(candidate)
			if len(candOpts) > len(bestOpts) {
				best = candidate
				bestOpts = candOpts
			}
		}
		return best
	}
	// No enabled entry fits; still emit the deterministic full spec.
	return full
}

func layoutOptsContain(live, want map[string]string) bool {
	for key, value := range want {
		if live[key] != value {
			return false
		}
	}
	return true
}

func layoutMatches(tab Tab, name string, opts map[string]string) bool {
	if name == "" {
		return true
	}
	if tab.Layout != name {
		return false
	}
	return maps.Equal(normalizeLayoutOpts(tab.LayoutOpts), opts)
}

func controllerHost() (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("controller hostname: %w", err)
	}
	host, _, _ = strings.Cut(strings.ToLower(host), ".")
	if !slices.Contains(hostAliases, host) {
		return "", fmt.Errorf("controller hostname %q is not a configured host alias", host)
	}
	return host, nil
}

func sourceHost(window OSWindow, controller string) (string, error) {
	hosts := make(map[string]bool)
	for _, tab := range window.Tabs {
		for _, pane := range tab.Windows {
			declared := pane.UserVars[userVarHost]
			legacyMarker := pane.Env["HYPRD_TAB_HOST"]
			observed := strings.ToLower(strings.TrimSpace(pane.UserVars[userVarObservedHost]))
			parsed, sshManaged, err := sshAlias(pane)
			if err != nil {
				return "", err
			}
			if observed != "" && !slices.Contains(hostAliases, observed) {
				return "", fmt.Errorf("pane reports unknown observed host %q", observed)
			}

			if declared == "" {
				declared = legacyMarker
			}
			if declared == "" {
				switch {
				case !sshManaged:
					declared = controller
				case parsed != "" && managedFlatSSH(pane, parsed):
					// Legacy remote pane without user vars: only migrate when the stable
					// top-level cmdline proves local kitten-ssh orchestration to one alias.
					declared = parsed
				default:
					return "", fmt.Errorf("pane is missing %s and is not a managed flat kitten ssh (top-level cmdline must be kitten ssh to a configured alias); refresh it before switching", userVarHost)
				}
			}
			if !slices.Contains(hostAliases, declared) {
				return "", fmt.Errorf("pane has unknown %s %q", userVarHost, declared)
			}
			if observed != "" && observed != declared {
				return "", fmt.Errorf("pane declares host %q but its prompt reports %q; leave the nested SSH session before switching", declared, observed)
			}
			if declared == controller {
				if sshManaged {
					return "", fmt.Errorf("pane declares local host %q but is running SSH; leave the manual SSH session before switching", declared)
				}
			} else if parsed != declared || !managedFlatSSH(pane, declared) {
				return "", fmt.Errorf("pane declares host %q but is not a managed flat kitten ssh to that destination (top-level cmdline must be kitten ssh; foreground may be plain ssh)", declared)
			}
			hosts[declared] = true
		}
	}
	if len(hosts) != 1 {
		return "", fmt.Errorf("focused managed window contains mixed or ambiguous hosts")
	}
	for host := range hosts {
		return host, nil
	}
	panic("unreachable")
}

// managedFlatSSH recognizes a local Kitty-orchestrated flat SSH session to alias.
//
// A managed pane must have a top-level pane.Cmdline of kitten ssh … alias or legacy kitty +kitten ssh … alias.
// Every SSH-shaped foreground process, if any, must target that same alias; plain ssh is fine.
//
// A local zsh where the user typed `ssh alias` stays unmanaged because its stable top-level cmdline is zsh, not kitten.
func managedFlatSSH(pane Pane, alias string) bool {
	if !orchestratedKittenSSH(pane.Cmdline, alias) {
		return false
	}
	for _, process := range pane.ForegroundProcesses {
		if !cmdlineIsSSH(process.Cmdline) {
			continue
		}
		destination, ok := sshCmdlineDestination(process.Cmdline)
		if !ok || sshHostToken(destination) != alias {
			return false
		}
	}
	return true
}

// orchestratedKittenSSH reports whether cmdline is a supported local launch shape whose SSH destination token equals alias.
func orchestratedKittenSSH(cmdline []string, alias string) bool {
	_, binary, ok := sshCmdlinePrefix(cmdline)
	if !ok || (binary != "kitten" && binary != "kitty") {
		return false
	}
	destination, ok := sshCmdlineDestination(cmdline)
	return ok && sshHostToken(destination) == alias
}

func sshAlias(pane Pane) (string, bool, error) {
	aliases := make(map[string]bool)
	sshManaged := false
	unknownDest := false
	// Include top-level cmdline (stable launch argv) plus foreground processes, which are often plain ssh after kitten settles.
	cmdlines := make([][]string, 0, 1+len(pane.ForegroundProcesses))
	if len(pane.Cmdline) > 0 {
		cmdlines = append(cmdlines, pane.Cmdline)
	}
	for _, process := range pane.ForegroundProcesses {
		cmdlines = append(cmdlines, process.Cmdline)
	}
	for _, cmdline := range cmdlines {
		if !cmdlineIsSSH(cmdline) {
			continue
		}
		sshManaged = true
		dest, ok := sshCmdlineDestination(cmdline)
		if !ok {
			unknownDest = true
			continue
		}
		if slices.Contains(hostAliases, dest) {
			aliases[dest] = true
			continue
		}
		unknownDest = true
	}
	if !sshManaged {
		return "", false, nil
	}
	if unknownDest || len(aliases) != 1 {
		if len(aliases) > 1 {
			return "", true, fmt.Errorf("pane command contains multiple configured SSH aliases")
		}
		// Detected SSH, but destination is missing, unrecognized, or ambiguous.
		return "", true, nil
	}
	for alias := range aliases {
		return alias, true, nil
	}
	return "", true, nil
}

func cmdlineIsSSH(cmdline []string) bool {
	_, _, ok := sshCmdlinePrefix(cmdline)
	return ok
}

// sshCmdlinePrefix recognizes only real SSH invocation shapes: argv0 basename ssh, kitten ssh, or kitty +kitten ssh.
// It never scans unrelated later arguments for the token "ssh".
func sshCmdlinePrefix(cmdline []string) (start int, binary string, ok bool) {
	if len(cmdline) == 0 {
		return 0, "", false
	}
	base := filepath.Base(cmdline[0])
	switch base {
	case "ssh":
		return 1, "ssh", true
	case "kitten":
		if len(cmdline) > 1 && cmdline[1] == "ssh" {
			return 2, "kitten", true
		}
	case "kitty":
		for i := 1; i < len(cmdline); i++ {
			if cmdline[i] == "+kitten" && i+1 < len(cmdline) && cmdline[i+1] == "ssh" {
				return i + 2, "kitty", true
			}
		}
	}
	return 0, "", false
}

func sshCmdlineDestination(cmdline []string) (string, bool) {
	start, _, ok := sshCmdlinePrefix(cmdline)
	if !ok || start >= len(cmdline) {
		return "", false
	}
	for i := start; i < len(cmdline); i++ {
		arg := cmdline[i]
		if arg == "--" {
			if i+1 < len(cmdline) {
				return sshHostToken(cmdline[i+1]), true
			}
			return "", false
		}
		if strings.HasPrefix(arg, "-") {
			// ssh options that consume a separate argument
			switch strings.TrimLeft(arg, "-") {
			case "b", "c", "D", "E", "e", "F", "I", "i", "J", "L", "l", "m", "O", "o", "p", "Q", "R", "S", "W", "w":
				i++
			}
			continue
		}
		return sshHostToken(arg), true
	}
	return "", false
}

func sshHostToken(token string) string {
	// strip optional user@ prefix; host aliases are bare names
	if _, host, ok := strings.Cut(token, "@"); ok {
		return host
	}
	return token
}

func newHostTransaction(windowID int) (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("transaction id: %w", err)
	}
	return fmt.Sprintf("host-%d-%s", windowID, hex.EncodeToString(random[:])), nil
}

func (t *Manager) preflightHostPlan(plan *hostPlan) error {
	targetHome, err := hostHome(plan.target)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", plan.target, err)
	}

	directories := make(map[string]bool)
	executables := map[string]bool{"zsh": true}
	type requirement struct {
		name string
		cwd  string
	}
	var requirements []requirement
	for i := range plan.tabs {
		for _, logical := range plan.tabs[i].logicalCWDs {
			logical, err = normalizeLogicalCWD(logical)
			if err != nil {
				return err
			}
			mapped := filepath.Join(targetHome, filepath.FromSlash(logical))
			plan.tabs[i].targetCWDs = append(plan.tabs[i].targetCWDs, mapped)
			directories[mapped] = true
		}
		for _, executable := range plan.tabs[i].definition.Executables {
			executables[executable] = true
		}
		for paneIndex := range plan.tabs[i].definition.Panes {
			for _, executable := range plan.tabs[i].definition.Panes[paneIndex].Executables {
				executables[executable] = true
			}
		}
		if plan.tabs[i].definition.Requires != "" {
			requirements = append(requirements, requirement{name: plan.tabs[i].definition.Requires, cwd: plan.tabs[i].targetCWDs[0]})
		}
	}
	if _, err := exec.LookPath("kitten"); err != nil {
		return fmt.Errorf("local executable kitten: %w", err)
	}
	for directory := range directories {
		if err := checkHostDirectory(plan.controller, plan.target, directory); err != nil {
			return err
		}
	}
	for executable := range executables {
		if err := checkHostExecutable(plan.controller, plan.target, executable); err != nil {
			return err
		}
	}
	for _, requirement := range requirements {
		if err := checkHostRequirement(plan.controller, plan.target, requirement.name, requirement.cwd); err != nil {
			return err
		}
	}
	return nil
}

func hostHome(host string) (string, error) {
	controller, err := controllerHost()
	if err != nil {
		return "", err
	}
	if host == controller {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("local HOME unavailable: %w", err)
		}
		if home == "" {
			return "", fmt.Errorf("local HOME unavailable")
		}
		return home, nil
	}
	out, err := sshCommand(host, `printf '%s' "$HOME"`).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("probe HOME: %w: %s", err, strings.TrimSpace(string(out)))
	}
	home := strings.TrimSpace(string(out))
	if !strings.HasPrefix(home, "/") {
		return "", fmt.Errorf("invalid HOME %q", home)
	}
	return home, nil
}

func checkHostDirectory(controller, host, directory string) error {
	if host == controller {
		info, err := os.Stat(directory)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("required directory %q is unavailable on %s", directory, host)
		}
		return nil
	}
	out, err := sshCommand(host, "test -d "+shellQuote(directory)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("required directory %q is unavailable on %s: %s", directory, host, strings.TrimSpace(string(out)))
	}
	return nil
}

func checkHostExecutable(controller, host, executable string) error {
	if host == controller {
		if _, err := exec.LookPath(executable); err != nil {
			return fmt.Errorf("required executable %q is unavailable on %s", executable, host)
		}
		return nil
	}
	out, err := sshCommand(host, "command -v -- "+shellQuote(executable)+" >/dev/null").CombinedOutput()
	if err != nil {
		return fmt.Errorf("required executable %q is unavailable on %s: %s", executable, host, strings.TrimSpace(string(out)))
	}
	return nil
}

func checkHostRequirement(controller, host, requirement, directory string) error {
	if requirement == "" {
		return nil
	}
	var command string
	switch requirement {
	case "git":
		command = "git -C " + shellQuote(directory) + " rev-parse --git-dir >/dev/null"
	case "justfile":
		command = "test -f " + shellQuote(filepath.Join(directory, "justfile"))
	default:
		return fmt.Errorf("unsupported configured requirement %q", requirement)
	}
	if host == controller {
		return runLocalRequirement(requirement, directory, command)
	}
	return runRemoteRequirement(host, requirement, directory, command)
}

func runLocalRequirement(requirement, directory, command string) error {
	err := exec.Command("sh", "-c", command).Run()
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// Shell ran; non-zero status is a definitive "not satisfied".
		return fmt.Errorf("%w: requirement %q is not satisfied in %q", errRequirementUnsatisfied, requirement, directory)
	}
	return fmt.Errorf("probe requirement %q in %q: %w", requirement, directory, err)
}

func runRemoteRequirement(host, requirement, directory, command string) error {
	cmd := sshCommand(host, command)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	detail := strings.TrimSpace(string(out))
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		// ssh binary missing, kill signal without exit, etc.
		if detail != "" {
			return fmt.Errorf("probe requirement %q on %s: %w: %s", requirement, host, err, detail)
		}
		return fmt.Errorf("probe requirement %q on %s: %w", requirement, host, err)
	}
	// ssh(1): exit 255 means ssh itself failed (transport/auth/config).
	// Any other non-zero status is the remote command's exit status.
	if code := exitErr.ExitCode(); code == 255 || code < 0 {
		if detail != "" {
			return fmt.Errorf("probe requirement %q on %s: ssh failed (exit %d): %s", requirement, host, code, detail)
		}
		return fmt.Errorf("probe requirement %q on %s: ssh failed (exit %d)", requirement, host, code)
	}
	if detail != "" {
		return fmt.Errorf("%w: requirement %q is not satisfied in %q on %s: %s", errRequirementUnsatisfied, requirement, directory, host, detail)
	}
	return fmt.Errorf("%w: requirement %q is not satisfied in %q on %s", errRequirementUnsatisfied, requirement, directory, host)
}

func sshCommand(host, command string) *exec.Cmd {
	return exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=4", "--", host, command)
}

func stageHostTabs(kitty *Client, plan *hostPlan) error {
	anchor := plan.window.Tabs[0].Windows[0].ID
	for i := range plan.tabs {
		tab := &plan.tabs[i]
		runtimeID := fmt.Sprintf("%d-%s%s", plan.window.ID, plan.profile.Prefix, tab.name)
		args := []string{"--type=tab", "--keep-focus", "--match", fmt.Sprintf("window_id:%d", anchor), "--source-window", fmt.Sprintf("id:%d", anchor), "--copy-env", "--env", "KITTY_TAB_ID=" + runtimeID, "--env", "HYPRD_LAUNCH_COMMAND=", "--env", "HYPRD_HOST_TXN=", "--env", "HYPRD_TAB_HOST=", "--tab-title=" + tab.source.Title}
		args = append(args, paneMetadataArgs(plan.target, tab.logicalCWDs[0], 0, plan.txn)...)
		args = append(args, hostLaunchCommand(plan.controller, plan.target, tab.definition.Command, tab.targetCWDs[0])...)
		paneID, err := kitty.launchID(args...)
		if err != nil {
			return fmt.Errorf("stage tab %s: %w", tab.name, err)
		}
		tab.stagePanes = append(tab.stagePanes, paneID)
		state, err := kitty.FullState()
		if err != nil {
			return err
		}
		stageTab, ok := tabForPane(state, plan.window.ID, paneID)
		if !ok {
			return fmt.Errorf("staged tab %s did not appear in captured OS window", tab.name)
		}
		tab.stageID = stageTab.ID
		if tab.layoutGoto != "" {
			if err := kitty.gotoLayoutByNumericID(tab.stageID, tab.layoutGoto); err != nil {
				return fmt.Errorf("stage layout for %s: %w", tab.name, err)
			}
		}
		for paneIndex, pane := range tab.definition.Panes {
			profilePane := paneIndex + 1
			paneArgs := []string{"--keep-focus", "--copy-env", "--match", fmt.Sprintf("id:%d", tab.stageID), "--env", "KITTY_TAB_ID=" + runtimeID, "--env", "HYPRD_LAUNCH_COMMAND=", "--env", "HYPRD_HOST_TXN=", "--env", "HYPRD_TAB_HOST="}
			paneArgs = append(paneArgs, paneMetadataArgs(plan.target, tab.logicalCWDs[profilePane], profilePane, plan.txn)...)
			if pane.Location != "" {
				paneArgs = append(paneArgs, "--location="+pane.Location)
			}
			if pane.Bias != 0 {
				paneArgs = append(paneArgs, "--bias", fmt.Sprint(pane.Bias))
			}
			paneArgs = append(paneArgs, hostLaunchCommand(plan.controller, plan.target, pane.Command, tab.targetCWDs[profilePane])...)
			paneID, err := kitty.launchID(paneArgs...)
			if err != nil {
				return fmt.Errorf("stage pane %d for %s: %w", paneIndex+1, tab.name, err)
			}
			tab.stagePanes = append(tab.stagePanes, paneID)
		}
		if tab.layoutGoto != "" && len(tab.definition.Panes) > 0 {
			if err := kitty.gotoLayoutByNumericID(tab.stageID, tab.layoutGoto); err != nil {
				return fmt.Errorf("reapply layout for %s: %w", tab.name, err)
			}
		}
	}
	return nil
}

func paneMetadataArgs(host, logicalCWD string, paneIndex int, txn string) []string {
	return []string{
		"--var", userVarHost + "=" + host,
		"--var", userVarCWD + "=" + logicalCWD,
		"--var", userVarPane + "=" + strconv.Itoa(paneIndex),
		"--var", userVarTxn + "=" + txn,
	}
}

// hostLaunchCommand builds Kitty launch argv for a pane on host.
// Construction is always flat from the controller: local target keeps zsh; any remote target is `kitten ssh <target>` independent of the source host.
func hostLaunchCommand(controller, host, command, cwd string) []string {
	resolved := withResolvedPWD(command, cwd)
	if host == controller {
		args := []string{"--cwd=" + cwd}
		if command == "xplr" {
			return append(args, "zsh", "-c", `cd "$(xplr --print-pwd-as-result)" 2>/dev/null; exec zsh -l`)
		}
		if resolved != "" {
			args = append(args, "--env", "HYPRD_LAUNCH_COMMAND="+resolved)
		}
		return append(args, persistentZshCommand()...)
	}

	script := "cd -- " + shellQuote(cwd) + " || exit; export HYPRD_LAUNCH_COMMAND=''"
	if command == "xplr" {
		script += `; cd "$(xplr --print-pwd-as-result)" 2>/dev/null; exec zsh -l`
	} else {
		if resolved != "" {
			script += "; export HYPRD_LAUNCH_COMMAND=" + shellQuote(resolved)
		}
		script += "; exec zsh -l"
	}
	return []string{"kitten", "ssh", "-t", "-o", "BatchMode=yes", "-o", "ConnectTimeout=4", "--", host, "zsh", "-lc", shellQuote(script)}
}

func verifyHostTabs(kitty *Client, plan *hostPlan) error {
	deadline := time.Now().Add(4 * time.Second)
	for {
		windows, err := kitty.FullState()
		if err != nil {
			return err
		}
		window, ok := windowByID(windows, plan.window.ID)
		if !ok {
			return fmt.Errorf("captured OS window disappeared")
		}
		verified := true
		for i := range plan.tabs {
			tab := &plan.tabs[i]
			runtimeID := fmt.Sprintf("%d-%s%s", plan.window.ID, plan.profile.Prefix, tab.name)
			staged, ok := numericTab(window, tab.stageID)
			if !ok || staged.Title != tab.source.Title || len(staged.Windows) != len(tab.source.Windows) {
				verified = false
				break
			}
			// Complete source layout + layout_opts must match before source tabs close.
			if tab.layoutName != "" && !layoutMatches(staged, tab.layoutName, tab.layoutOpts) {
				verified = false
				break
			}
			ordered, paneErr := orderedProfilePanes(staged.Windows)
			if paneErr != nil {
				verified = false
				break
			}
			for paneIndex, pane := range ordered {
				observed := pane.UserVars[userVarObservedHost]
				if pane.Env["KITTY_TAB_ID"] != runtimeID ||
					pane.UserVars[userVarHost] != plan.target ||
					(observed != "" && observed != plan.target) ||
					pane.UserVars[userVarCWD] != tab.logicalCWDs[paneIndex] ||
					pane.UserVars[userVarPane] != strconv.Itoa(paneIndex) ||
					pane.UserVars[userVarTxn] != plan.txn ||
					pane.ID != tab.stagePanes[paneIndex] ||
					!paneRunsOnHost(pane, plan.target, plan.controller) {
					verified = false
					break
				}
			}
		}
		if verified {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("staged tabs, panes, or layouts did not reach the expected transaction state")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func paneRunsOnHost(pane Pane, host, controller string) bool {
	alias, managed, err := sshAlias(pane)
	if err != nil {
		return false
	}
	if host == controller {
		return !managed
	}
	// Plain foreground ssh is fine once top-level cmdline proves kitten orchestration and every SSH-shaped destination matches the declared target.
	return managed && alias == host && managedFlatSSH(pane, host)
}

func rollbackHostTabs(kitty *Client, plan *hostPlan) error {
	windows, err := kitty.FullState()
	if err != nil {
		return err
	}
	window, ok := windowByID(windows, plan.window.ID)
	if !ok {
		return fmt.Errorf("captured OS window disappeared")
	}
	var ids []int
	for _, tab := range window.Tabs {
		for _, pane := range tab.Windows {
			if pane.UserVars[userVarTxn] == plan.txn {
				ids = append(ids, tab.ID)
				break
			}
		}
	}
	return kitty.closeTabsByNumericIDs(ids)
}

func tabForPane(windows []OSWindow, windowID, paneID int) (Tab, bool) {
	window, ok := windowByID(windows, windowID)
	if !ok {
		return Tab{}, false
	}
	for _, tab := range window.Tabs {
		for _, pane := range tab.Windows {
			if pane.ID == paneID {
				return tab, true
			}
		}
	}
	return Tab{}, false
}

func windowByID(windows []OSWindow, id int) (OSWindow, bool) {
	for _, window := range windows {
		if window.ID == id {
			return window, true
		}
	}
	return OSWindow{}, false
}

func numericTab(window OSWindow, id int) (Tab, bool) {
	for _, tab := range window.Tabs {
		if tab.ID == id {
			return tab, true
		}
	}
	return Tab{}, false
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func (t *Manager) notifyHost(kitty *Client, title, body string, critical bool) {
	icon := "system-monitor"
	if critical {
		icon = "error"
	}
	args := []string{"--app-name=hyprd", "--identifier=hyprd-host-switch", "--icon=" + icon}
	if critical {
		args = append(args, "--urgency=critical")
	}
	args = append(args, title, body)
	_ = kitty.notify(args...)
}
