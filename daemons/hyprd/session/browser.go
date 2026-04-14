package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"

	"gopkg.in/yaml.v3"
)

const (
	browserUsage         = "usage: browser {windows|snapshot|show|hypr|restore} ..."
	browserWindowsUsage  = "usage: browser windows [--all] [--profile <name|path>]"
	browserSnapshotUsage = "usage: browser snapshot <name> [active|largest|index] [--profile <name|path>]"
	browserShowUsage     = "usage: browser show <name> [snapshot]"
	browserHyprUsage     = "usage: browser hypr <name> [snapshot]"
	browserRestoreUsage  = "usage: browser restore <name> [snapshot] [--mode urls|exact] [--profile <name|path>] [--force] [--dry-run]"
	firefoxBinary        = "firefox-developer-edition"
)

var (
	firefoxTitleSuffixes = []string{
		" — Firefox Developer Edition",
		" — Mozilla Firefox",
	}
	trivialBrowserURLs = map[string]struct{}{
		"":                         {},
		"about:blank":              {},
		"about:home":               {},
		"about:newtab":             {},
		"http://localhost:42069/":  {},
		"https://localhost:42069/": {},
	}
	snapshotNamePattern       = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	defaultSessionCheckpoints = []byte("{\"profile-after-change\":true,\"final-ui-startup\":true,\"sessionstore-windows-restored\":true}\n")
)

type Browser struct {
	hypr  *hypr.Client
	state *state.State
}

type firefoxProfile struct {
	Root       string
	Name       string
	InstallKey string
}

type firefoxSessionStore struct {
	Source  string
	Raw     firefoxSessionEnvelope
	Windows []firefoxWindow
}

type firefoxSessionEnvelope struct {
	Version        json.RawMessage   `json:"version,omitempty"`
	Session        json.RawMessage   `json:"session,omitempty"`
	Global         json.RawMessage   `json:"global,omitempty"`
	MaxSplitViewID json.RawMessage   `json:"maxSplitViewId,omitempty"`
	SavedGroups    json.RawMessage   `json:"savedGroups,omitempty"`
	Windows        []json.RawMessage `json:"windows"`
}

type firefoxWindow struct {
	Selected int            `json:"selected"`
	Tabs     []firefoxTab   `json:"tabs"`
	Groups   []firefoxGroup `json:"groups"`
}

type firefoxTab struct {
	Index        int            `json:"index"`
	Entries      []firefoxEntry `json:"entries"`
	Pinned       bool           `json:"pinned"`
	Hidden       bool           `json:"hidden"`
	GroupID      string         `json:"groupId"`
	Group        string         `json:"group"`
	LastAccessed int64          `json:"lastAccessed"`
}

type firefoxEntry struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type firefoxGroup struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Collapsed bool   `json:"collapsed"`
}

type browserWindowSummary struct {
	SelectedTab          int                  `yaml:"selected_tab"`
	TabCount             int                  `yaml:"tab_count"`
	GroupCount           int                  `yaml:"group_count"`
	SelectedTitle        string               `yaml:"selected_title,omitempty"`
	SelectedURL          string               `yaml:"selected_url,omitempty"`
	HyprOrderMatchesTabs bool                 `yaml:"hypr_order_matches_tabs"`
	Browser              config.BrowserConfig `yaml:"-"`
	Tabs                 []browserTabSummary  `yaml:"-"`
}

type browserTabSummary struct {
	Position   int    `yaml:"position"`
	Title      string `yaml:"title,omitempty"`
	URL        string `yaml:"url,omitempty"`
	Pinned     bool   `yaml:"pinned,omitempty"`
	Hidden     bool   `yaml:"hidden,omitempty"`
	GroupID    string `yaml:"group_id,omitempty"`
	Group      string `yaml:"group,omitempty"`
	GroupColor string `yaml:"group_color,omitempty"`
}

type browserSnapshotSummary struct {
	Name      string                `yaml:"name"`
	CreatedAt string                `yaml:"created_at"`
	Profile   browserProfileSummary `yaml:"profile"`
	Source    browserSourceSummary  `yaml:"source"`
	Window    browserSnapshotWindow `yaml:"window"`
	Browser   config.BrowserConfig  `yaml:"browser"`
	Tabs      []browserTabSummary   `yaml:"tabs,omitempty"`
}

type browserProfileSummary struct {
	Name       string `yaml:"name"`
	Path       string `yaml:"path"`
	InstallKey string `yaml:"install_key,omitempty"`
}

type browserSourceSummary struct {
	SessionFile string `yaml:"session_file"`
	WindowIndex int    `yaml:"window_index"`
}

type browserSnapshotWindow struct {
	SelectedTab          int    `yaml:"selected_tab"`
	TabCount             int    `yaml:"tab_count"`
	GroupCount           int    `yaml:"group_count"`
	SelectedTitle        string `yaml:"selected_title,omitempty"`
	SelectedURL          string `yaml:"selected_url,omitempty"`
	HyprOrderMatchesTabs bool   `yaml:"hypr_order_matches_tabs"`
}

type iniFile map[string]map[string]string

func NewBrowser(h *hypr.Client, s *state.State) *Browser {
	return &Browser{hypr: h, state: s}
}

func (b *Browser) Execute(args string) (string, error) {
	parts := strings.Fields(args)
	if len(parts) == 0 {
		return "", fmt.Errorf(browserUsage)
	}

	switch parts[0] {
	case "windows":
		return b.executeWindows(parts[1:])
	case "snapshot":
		return b.executeSnapshot(parts[1:])
	case "show":
		return b.executeShow(parts[1:])
	case "hypr":
		return b.executeHypr(parts[1:])
	case "restore":
		return b.executeRestore(parts[1:])
	default:
		return "", fmt.Errorf(browserUsage)
	}
}

func (b *Browser) ResolveLaunchConfig(cfg config.BrowserConfig) (config.BrowserConfig, error) {
	if cfg.Snapshot == "" {
		return cfg, nil
	}

	snapshotCfg, err := b.SnapshotConfig(cfg.Snapshot, "")
	if err != nil {
		if len(cfg.AllURLs()) > 0 {
			return cfg, nil
		}
		return config.BrowserConfig{}, err
	}
	snapshotCfg.Snapshot = cfg.Snapshot
	return snapshotCfg, nil
}

func (b *Browser) SnapshotConfig(name, snapshotID string) (config.BrowserConfig, error) {
	_, store, err := b.loadSnapshotSession(name, snapshotID)
	if err != nil {
		return config.BrowserConfig{}, err
	}
	if len(store.Windows) == 0 {
		return config.BrowserConfig{}, fmt.Errorf("snapshot %q has no windows", name)
	}
	return summarizeFirefoxWindow(store.Windows[0]).Browser, nil
}

func (b *Browser) UsesExactRestore(cfg config.BrowserConfig) bool {
	return browserMode(cfg) == "exact"
}

func (b *Browser) RestoreConfiguredSnapshot(cfg config.BrowserConfig, dryRun bool) (string, error) {
	if !b.UsesExactRestore(cfg) {
		return "", fmt.Errorf("browser restore mode %q is not exact", browserMode(cfg))
	}
	if cfg.Snapshot == "" {
		return "", fmt.Errorf("browser exact restore requires snapshot")
	}

	dir, store, err := b.loadSnapshotSession(cfg.Snapshot, "")
	if err != nil {
		return "", err
	}
	profile, err := discoverFirefoxProfile(cfg.Profile)
	if err != nil {
		return "", err
	}
	return b.restoreSnapshotExact(cfg.Snapshot, dir, store, profile, cfg.Force, dryRun)
}

func (b *Browser) executeWindows(args []string) (string, error) {
	var profileArg string
	all := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--all":
			all = true
		case arg == "--profile":
			if i+1 >= len(args) {
				return "", fmt.Errorf(browserWindowsUsage)
			}
			i++
			profileArg = args[i]
		case strings.HasPrefix(arg, "--profile="):
			profileArg = strings.TrimPrefix(arg, "--profile=")
		default:
			return "", fmt.Errorf(browserWindowsUsage)
		}
	}

	profile, err := discoverFirefoxProfile(profileArg)
	if err != nil {
		return "", err
	}
	store, err := b.loadFirefoxSession(profile)
	if err != nil {
		return "", err
	}

	activeTitle := b.currentFirefoxTitle()
	var lines []string
	lines = append(lines, fmt.Sprintf("profile: %s (%s)", profile.Name, profile.Root))
	lines = append(lines, fmt.Sprintf("session: %s", store.Source))
	if activeTitle != "" {
		lines = append(lines, fmt.Sprintf("active_hypr_title: %s", activeTitle))
	}

	for i, window := range store.Windows {
		if !all && !windowIsInteresting(window) {
			continue
		}
		summary := summarizeFirefoxWindow(window)
		mark := " "
		if titlesMatch(activeTitle, summary.SelectedTitle) {
			mark = "*"
		}

		var tags []string
		if windowIsInteresting(window) {
			tags = append(tags, "interesting")
		}
		if summary.GroupCount > 0 {
			tags = append(tags, fmt.Sprintf("groups=%d", summary.GroupCount))
		}
		tagSuffix := ""
		if len(tags) > 0 {
			tagSuffix = " " + strings.Join(tags, " ")
		}

		lines = append(lines, fmt.Sprintf(
			"%s %02d tabs=%d selected=%d title=%q url=%q%s",
			mark, i+1, summary.TabCount, summary.SelectedTab, summary.SelectedTitle, summary.SelectedURL, tagSuffix,
		))
	}

	return strings.Join(lines, "\n"), nil
}

func (b *Browser) executeSnapshot(args []string) (string, error) {
	var profileArg string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--profile":
			if i+1 >= len(args) {
				return "", fmt.Errorf(browserSnapshotUsage)
			}
			i++
			profileArg = args[i]
		case strings.HasPrefix(arg, "--profile="):
			profileArg = strings.TrimPrefix(arg, "--profile=")
		case strings.HasPrefix(arg, "--"):
			return "", fmt.Errorf(browserSnapshotUsage)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) < 1 || len(positional) > 2 {
		return "", fmt.Errorf(browserSnapshotUsage)
	}

	name := positional[0]
	selector := "active"
	if len(positional) == 2 {
		selector = positional[1]
	}

	profile, err := discoverFirefoxProfile(profileArg)
	if err != nil {
		return "", err
	}
	store, err := b.loadFirefoxSession(profile)
	if err != nil {
		return "", err
	}

	windowIndex, err := b.resolveWindowIndex(store, selector)
	if err != nil {
		return "", err
	}
	return b.writeSnapshot(name, profile, windowIndex, store)
}

func (b *Browser) executeShow(args []string) (string, error) {
	if len(args) < 1 || len(args) > 2 {
		return "", fmt.Errorf(browserShowUsage)
	}
	dir, err := resolveSnapshotDir(args[0], optionalArg(args, 1))
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "snapshot.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\n"), nil
}

func (b *Browser) executeHypr(args []string) (string, error) {
	if len(args) < 1 || len(args) > 2 {
		return "", fmt.Errorf(browserHyprUsage)
	}
	cfg, err := b.SnapshotConfig(args[0], optionalArg(args, 1))
	if err != nil {
		return "", err
	}
	doc := map[string]config.BrowserConfig{"browser": cfg}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (b *Browser) executeRestore(args []string) (string, error) {
	var (
		mode       = "urls"
		profileArg string
		force      bool
		dryRun     bool
		positional []string
	)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--force":
			force = true
		case arg == "--dry-run":
			dryRun = true
		case arg == "--mode":
			if i+1 >= len(args) {
				return "", fmt.Errorf(browserRestoreUsage)
			}
			i++
			mode = args[i]
		case strings.HasPrefix(arg, "--mode="):
			mode = strings.TrimPrefix(arg, "--mode=")
		case arg == "--profile":
			if i+1 >= len(args) {
				return "", fmt.Errorf(browserRestoreUsage)
			}
			i++
			profileArg = args[i]
		case strings.HasPrefix(arg, "--profile="):
			profileArg = strings.TrimPrefix(arg, "--profile=")
		case strings.HasPrefix(arg, "--"):
			return "", fmt.Errorf(browserRestoreUsage)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) < 1 || len(positional) > 2 {
		return "", fmt.Errorf(browserRestoreUsage)
	}
	if mode != "urls" && mode != "exact" {
		return "", fmt.Errorf(browserRestoreUsage)
	}

	name := positional[0]
	snapshotID := optionalArg(positional, 1)
	dir, store, err := b.loadSnapshotSession(name, snapshotID)
	if err != nil {
		return "", err
	}

	if mode == "urls" {
		return b.restoreSnapshotURLs(store, dryRun)
	}

	profile, err := discoverFirefoxProfile(profileArg)
	if err != nil {
		return "", err
	}
	return b.restoreSnapshotExact(name, dir, store, profile, force, dryRun)
}

func (b *Browser) loadFirefoxSession(profile firefoxProfile) (*firefoxSessionStore, error) {
	source, err := firefoxSessionSourceFile(profile)
	if err != nil {
		return nil, err
	}
	payload, err := decodeMozillaLZ4File(source)
	if err != nil {
		return nil, err
	}
	return parseFirefoxSession(payload, source)
}

func (b *Browser) loadSnapshotSession(name, snapshotID string) (string, *firefoxSessionStore, error) {
	dir, err := resolveSnapshotDir(name, snapshotID)
	if err != nil {
		return "", nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "session.json"))
	if err != nil {
		return "", nil, err
	}
	store, err := parseFirefoxSession(data, filepath.Join(dir, "session.json"))
	if err != nil {
		return "", nil, err
	}
	return dir, store, nil
}

func parseFirefoxSession(payload []byte, source string) (*firefoxSessionStore, error) {
	var raw firefoxSessionEnvelope
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", source, err)
	}

	store := &firefoxSessionStore{Source: source, Raw: raw}
	store.Windows = make([]firefoxWindow, 0, len(raw.Windows))
	for i, windowRaw := range raw.Windows {
		var window firefoxWindow
		if err := json.Unmarshal(windowRaw, &window); err != nil {
			return nil, fmt.Errorf("parse %s window %d: %w", source, i+1, err)
		}
		store.Windows = append(store.Windows, window)
	}
	return store, nil
}

func firefoxSessionSourceFile(profile firefoxProfile) (string, error) {
	candidates := []string{
		filepath.Join(profile.Root, "sessionstore-backups", "recovery.jsonlz4"),
		filepath.Join(profile.Root, "sessionstore-backups", "recovery.baklz4"),
		filepath.Join(profile.Root, "sessionstore.jsonlz4"),
		filepath.Join(profile.Root, "sessionstore-backups", "previous.jsonlz4"),
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no sessionstore file found in %s", profile.Root)
}

func discoverFirefoxProfile(raw string) (firefoxProfile, error) {
	root, err := firefoxRoot()
	if err != nil {
		return firefoxProfile{}, err
	}

	profiles, err := readINI(filepath.Join(root, "profiles.ini"))
	if err != nil {
		return firefoxProfile{}, err
	}

	installs := iniFile{}
	installsPath := filepath.Join(root, "installs.ini")
	if fileExists(installsPath) {
		installs, err = readINI(installsPath)
		if err != nil {
			return firefoxProfile{}, err
		}
	}

	if raw != "" {
		candidate := config.ExpandPath(raw)
		if fileExists(candidate) {
			return firefoxProfile{
				Root: filepath.Clean(candidate),
				Name: filepath.Base(candidate),
			}, nil
		}

		for _, section := range profiles.sections() {
			if !strings.HasPrefix(section, "Profile") {
				continue
			}
			profilePath := resolveFirefoxProfilePath(root, profiles.get(section, "Path"), profiles.get(section, "IsRelative") != "0")
			if raw == profiles.get(section, "Name") || raw == profiles.get(section, "Path") || raw == filepath.Base(profilePath) {
				return firefoxProfile{
					Root: profilePath,
					Name: firstNonEmpty(profiles.get(section, "Name"), filepath.Base(profilePath)),
				}, nil
			}
		}
		return firefoxProfile{}, fmt.Errorf("profile %q not found", raw)
	}

	for _, section := range installs.sections() {
		defaultPath := installs.get(section, "Default")
		if defaultPath == "" {
			continue
		}
		profilePath := filepath.Join(root, defaultPath)
		return firefoxProfile{
			Root:       profilePath,
			Name:       profileNameForPath(profiles, root, profilePath),
			InstallKey: section,
		}, nil
	}

	for _, section := range profiles.sections() {
		if !strings.HasPrefix(section, "Profile") || profiles.get(section, "Default") != "1" {
			continue
		}
		profilePath := resolveFirefoxProfilePath(root, profiles.get(section, "Path"), profiles.get(section, "IsRelative") != "0")
		return firefoxProfile{
			Root: profilePath,
			Name: firstNonEmpty(profiles.get(section, "Name"), filepath.Base(profilePath)),
		}, nil
	}

	return firefoxProfile{}, fmt.Errorf("could not determine default Firefox profile")
}

func firefoxRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	for _, candidate := range []string{
		filepath.Join(home, ".config", "mozilla", "firefox"),
		filepath.Join(home, ".mozilla", "firefox"),
	} {
		if fileExists(filepath.Join(candidate, "profiles.ini")) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find Firefox profile root")
}

func readINI(path string) (iniFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	out := make(iniFile)
	current := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = strings.TrimSpace(line[1 : len(line)-1])
			if _, ok := out[current]; !ok {
				out[current] = map[string]string{}
			}
			continue
		}
		if current == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[current][strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return out, nil
}

func (ini iniFile) sections() []string {
	var sections []string
	for section := range ini {
		sections = append(sections, section)
	}
	slices.Sort(sections)
	return sections
}

func (ini iniFile) get(section, key string) string {
	if values, ok := ini[section]; ok {
		return values[key]
	}
	return ""
}

func resolveFirefoxProfilePath(root, value string, relative bool) string {
	if value == "" {
		return root
	}
	if relative {
		return filepath.Join(root, value)
	}
	return config.ExpandPath(value)
}

func profileNameForPath(profiles iniFile, root, target string) string {
	target = filepath.Clean(target)
	for _, section := range profiles.sections() {
		if !strings.HasPrefix(section, "Profile") {
			continue
		}
		path := resolveFirefoxProfilePath(root, profiles.get(section, "Path"), profiles.get(section, "IsRelative") != "0")
		if filepath.Clean(path) == target {
			return firstNonEmpty(profiles.get(section, "Name"), filepath.Base(target))
		}
	}
	return filepath.Base(target)
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

func (b *Browser) resolveWindowIndex(store *firefoxSessionStore, selector string) (int, error) {
	if len(store.Windows) == 0 {
		return 0, fmt.Errorf("session file has no windows")
	}

	if selector != "" && selector != "active" {
		if selector == "largest" {
			return pickBestWindow(store.Windows, allWindowIndexes(store.Windows)), nil
		}
		if idx, err := strconv.Atoi(selector); err == nil {
			idx--
			if idx < 0 || idx >= len(store.Windows) {
				return 0, fmt.Errorf("window index %d is out of range", idx+1)
			}
			return idx, nil
		}
		return 0, fmt.Errorf("unknown window selector %q", selector)
	}

	activeTitle := b.currentFirefoxTitle()
	if activeTitle != "" {
		for i, window := range store.Windows {
			if titlesMatch(activeTitle, selectedTabTitle(window)) {
				return i, nil
			}
		}
	}

	pool := interestingWindowIndexes(store.Windows)
	if len(pool) == 0 {
		pool = allWindowIndexes(store.Windows)
	}
	return pickBestWindow(store.Windows, pool), nil
}

func allWindowIndexes(windows []firefoxWindow) []int {
	out := make([]int, 0, len(windows))
	for i := range windows {
		out = append(out, i)
	}
	return out
}

func interestingWindowIndexes(windows []firefoxWindow) []int {
	var out []int
	for i, window := range windows {
		if windowIsInteresting(window) {
			out = append(out, i)
		}
	}
	return out
}

func pickBestWindow(windows []firefoxWindow, pool []int) int {
	best := pool[0]
	for _, idx := range pool[1:] {
		bestTabs, candTabs := len(windows[best].Tabs), len(windows[idx].Tabs)
		bestGroups, candGroups := len(windows[best].Groups), len(windows[idx].Groups)
		bestAccess, candAccess := windowLastAccessed(windows[best]), windowLastAccessed(windows[idx])

		switch {
		case candGroups > bestGroups:
			best = idx
		case candGroups == bestGroups && candTabs > bestTabs:
			best = idx
		case candGroups == bestGroups && candTabs == bestTabs && candAccess > bestAccess:
			best = idx
		}
	}
	return best
}

func windowIsInteresting(window firefoxWindow) bool {
	if len(window.Tabs) > 1 || len(window.Groups) > 0 {
		return true
	}
	for _, tab := range window.Tabs {
		if tab.Pinned {
			return true
		}
		if _, ok := trivialBrowserURLs[selectedEntry(tab).URL]; !ok {
			return true
		}
	}
	return false
}

func windowLastAccessed(window firefoxWindow) int64 {
	var best int64
	for _, tab := range window.Tabs {
		if tab.LastAccessed > best {
			best = tab.LastAccessed
		}
	}
	return best
}

func selectedEntry(tab firefoxTab) firefoxEntry {
	if len(tab.Entries) == 0 {
		return firefoxEntry{}
	}
	index := tab.Index - 1
	if index < 0 {
		index = 0
	}
	if index >= len(tab.Entries) {
		index = len(tab.Entries) - 1
	}
	return tab.Entries[index]
}

func selectedTabTitle(window firefoxWindow) string {
	tab := selectedWindowTab(window)
	if tab == nil {
		return ""
	}
	return selectedEntry(*tab).Title
}

func selectedTabURL(window firefoxWindow) string {
	tab := selectedWindowTab(window)
	if tab == nil {
		return ""
	}
	return selectedEntry(*tab).URL
}

func selectedWindowTab(window firefoxWindow) *firefoxTab {
	if len(window.Tabs) == 0 {
		return nil
	}
	index := window.Selected - 1
	if index < 0 {
		index = 0
	}
	if index >= len(window.Tabs) {
		index = len(window.Tabs) - 1
	}
	return &window.Tabs[index]
}

func titlesMatch(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

func summarizeFirefoxWindow(window firefoxWindow) browserWindowSummary {
	groupByID := make(map[string]firefoxGroup, len(window.Groups))
	for _, group := range window.Groups {
		groupByID[group.ID] = group
	}

	var summaryTabs []browserTabSummary
	for i, tab := range window.Tabs {
		entry := selectedEntry(tab)
		groupID := tabGroupID(tab)
		group := groupByID[groupID]
		summaryTabs = append(summaryTabs, browserTabSummary{
			Position:   i + 1,
			Title:      entry.Title,
			URL:        entry.URL,
			Pinned:     tab.Pinned,
			Hidden:     tab.Hidden,
			GroupID:    groupID,
			Group:      groupName(group, groupID),
			GroupColor: group.Color,
		})
	}

	browserCfg := config.BrowserConfig{}
	for _, tab := range summaryTabs {
		if tab.Hidden || tab.URL == "" {
			continue
		}
		switch {
		case tab.Pinned:
			browserCfg.Pinned = append(browserCfg.Pinned, tab.URL)
		case tab.GroupID == "":
			browserCfg.URLs = append(browserCfg.URLs, tab.URL)
		}
	}

	seenGroups := map[string]struct{}{}
	for _, group := range window.Groups {
		if group.ID == "" {
			continue
		}
		seenGroups[group.ID] = struct{}{}
		groupCfg := config.BrowserGroup{
			Name:      groupName(group, group.ID),
			Color:     group.Color,
			Collapsed: group.Collapsed,
		}
		for _, tab := range summaryTabs {
			if !tab.Hidden && tab.URL != "" && tab.GroupID == group.ID {
				groupCfg.URLs = append(groupCfg.URLs, tab.URL)
			}
		}
		browserCfg.Groups = append(browserCfg.Groups, groupCfg)
	}

	for _, tab := range summaryTabs {
		if tab.GroupID == "" {
			continue
		}
		if _, ok := seenGroups[tab.GroupID]; ok {
			continue
		}
		seenGroups[tab.GroupID] = struct{}{}
		groupCfg := config.BrowserGroup{Name: firstNonEmpty(tab.Group, tab.GroupID)}
		for _, candidate := range summaryTabs {
			if !candidate.Hidden && candidate.URL != "" && candidate.GroupID == tab.GroupID {
				groupCfg.URLs = append(groupCfg.URLs, candidate.URL)
			}
		}
		browserCfg.Groups = append(browserCfg.Groups, groupCfg)
	}

	var liveOrder []string
	for _, tab := range summaryTabs {
		if !tab.Hidden && tab.URL != "" {
			liveOrder = append(liveOrder, tab.URL)
		}
	}

	return browserWindowSummary{
		SelectedTab:          max(window.Selected, 1),
		TabCount:             len(window.Tabs),
		GroupCount:           len(window.Groups),
		SelectedTitle:        selectedTabTitle(window),
		SelectedURL:          selectedTabURL(window),
		HyprOrderMatchesTabs: slices.Equal(browserCfg.AllURLs(), liveOrder),
		Browser:              browserCfg,
		Tabs:                 summaryTabs,
	}
}

func groupName(group firefoxGroup, fallback string) string {
	if group.Name != "" {
		return group.Name
	}
	return fallback
}

func tabGroupID(tab firefoxTab) string {
	if tab.GroupID != "" {
		return tab.GroupID
	}
	return tab.Group
}

func (b *Browser) writeSnapshot(name string, profile firefoxProfile, windowIndex int, store *firefoxSessionStore) (string, error) {
	slug, err := slugifySnapshotName(name)
	if err != nil {
		return "", err
	}

	root, err := browserStateRoot()
	if err != nil {
		return "", err
	}
	baseDir := filepath.Join(root, slug)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", err
	}

	snapshotID := time.Now().Format("20060102-150405")
	targetDir := filepath.Join(baseDir, snapshotID)
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		return "", err
	}

	windowSummary := summarizeFirefoxWindow(store.Windows[windowIndex])
	summary := browserSnapshotSummary{
		Name:      slug,
		CreatedAt: time.Now().Format(time.RFC3339),
		Profile: browserProfileSummary{
			Name:       profile.Name,
			Path:       profile.Root,
			InstallKey: profile.InstallKey,
		},
		Source: browserSourceSummary{
			SessionFile: store.Source,
			WindowIndex: windowIndex + 1,
		},
		Window: browserSnapshotWindow{
			SelectedTab:          windowSummary.SelectedTab,
			TabCount:             windowSummary.TabCount,
			GroupCount:           windowSummary.GroupCount,
			SelectedTitle:        windowSummary.SelectedTitle,
			SelectedURL:          windowSummary.SelectedURL,
			HyprOrderMatchesTabs: windowSummary.HyprOrderMatchesTabs,
		},
		Browser: windowSummary.Browser,
		Tabs:    windowSummary.Tabs,
	}

	summaryData, err := yaml.Marshal(summary)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(targetDir, "snapshot.yaml"), summaryData, 0o644); err != nil {
		return "", err
	}

	sessionData, err := snapshotSessionJSON(store.Raw, windowIndex)
	if err != nil {
		return "", err
	}
	sessionData = append(sessionData, '\n')
	if err := os.WriteFile(filepath.Join(targetDir, "session.json"), sessionData, 0o644); err != nil {
		return "", err
	}

	if err := updateLatestSnapshotLink(baseDir, snapshotID); err != nil {
		return "", err
	}
	return targetDir, nil
}

func snapshotSessionJSON(raw firefoxSessionEnvelope, windowIndex int) ([]byte, error) {
	if windowIndex < 0 || windowIndex >= len(raw.Windows) {
		return nil, fmt.Errorf("window index %d is out of range", windowIndex+1)
	}

	doc := map[string]any{
		"windows":        []json.RawMessage{raw.Windows[windowIndex]},
		"selectedWindow": 1,
		"_closedWindows": []any{},
	}
	if len(raw.Version) > 0 {
		doc["version"] = json.RawMessage(raw.Version)
	}
	if len(raw.Session) > 0 {
		doc["session"] = json.RawMessage(raw.Session)
	}
	if len(raw.Global) > 0 {
		doc["global"] = json.RawMessage(raw.Global)
	}
	if len(raw.MaxSplitViewID) > 0 {
		doc["maxSplitViewId"] = json.RawMessage(raw.MaxSplitViewID)
	}
	if len(raw.SavedGroups) > 0 {
		doc["savedGroups"] = json.RawMessage(raw.SavedGroups)
	}
	return json.MarshalIndent(doc, "", "  ")
}

func updateLatestSnapshotLink(baseDir, snapshotID string) error {
	latest := filepath.Join(baseDir, "latest")
	tmp := filepath.Join(baseDir, ".latest.tmp")
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := os.Symlink(snapshotID, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, latest); err != nil {
		return err
	}
	return nil
}

func resolveSnapshotDir(name, snapshotID string) (string, error) {
	slug, err := slugifySnapshotName(name)
	if err != nil {
		return "", err
	}

	for _, root := range snapshotRoots() {
		baseDir := filepath.Join(root, slug)
		if snapshotID != "" {
			candidate := filepath.Join(baseDir, snapshotID)
			if isDir(candidate) {
				return candidate, nil
			}
			continue
		}

		dir, err := latestSnapshotDir(baseDir)
		if err == nil {
			return dir, nil
		}
	}

	if snapshotID == "" {
		return "", fmt.Errorf("no snapshot found for %q", slug)
	}
	return "", fmt.Errorf("snapshot %q not found for %q", snapshotID, slug)
}

func latestSnapshotDir(baseDir string) (string, error) {
	latest := filepath.Join(baseDir, "latest")
	if info, err := os.Lstat(latest); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(latest)
			if err != nil {
				return "", err
			}
			if isDir(resolved) {
				return resolved, nil
			}
		}
		if info.IsDir() {
			return latest, nil
		}
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", err
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "latest" {
			dirs = append(dirs, entry.Name())
		}
	}
	if len(dirs) == 0 {
		return "", fmt.Errorf("no snapshot directories in %s", baseDir)
	}
	slices.Sort(dirs)
	return filepath.Join(baseDir, dirs[len(dirs)-1]), nil
}

func browserStateRoot() (string, error) {
	if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" {
		return filepath.Join(stateHome, "hyprd", "browser-sessions"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "hyprd", "browser-sessions"), nil
}

func legacyBrowserStateRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "firefox-sessions"), nil
}

func snapshotRoots() []string {
	var roots []string
	if root, err := browserStateRoot(); err == nil {
		roots = append(roots, root)
	}
	if root, err := legacyBrowserStateRoot(); err == nil {
		roots = append(roots, root)
	}
	return roots
}

func slugifySnapshotName(name string) (string, error) {
	slug := strings.Trim(snapshotNamePattern.ReplaceAllString(strings.TrimSpace(name), "-"), "-")
	if slug == "" {
		return "", fmt.Errorf("snapshot name must contain at least one visible character")
	}
	return slug, nil
}

func (b *Browser) restoreSnapshotURLs(store *firefoxSessionStore, dryRun bool) (string, error) {
	if len(store.Windows) == 0 {
		return "", fmt.Errorf("snapshot has no windows")
	}

	var tabs []browserTabSummary
	for _, tab := range summarizeFirefoxWindow(store.Windows[0]).Tabs {
		if !tab.Hidden && tab.URL != "" {
			tabs = append(tabs, tab)
		}
	}
	if len(tabs) == 0 {
		return "", fmt.Errorf("snapshot has no visible tabs to restore")
	}

	commands := make([][]string, 0, len(tabs))
	first := append(slices.Clone(b.browserCommandParts()), "--new-window", tabs[0].URL)
	commands = append(commands, first)
	for _, tab := range tabs[1:] {
		commands = append(commands, append(slices.Clone(b.browserCommandParts()), "--new-tab", tab.URL))
	}

	if dryRun {
		var lines []string
		for _, cmd := range commands {
			lines = append(lines, shellQuoteCommand(cmd))
		}
		return strings.Join(lines, "\n"), nil
	}

	for _, cmd := range commands {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			return "", err
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Sprintf("restored urls: %d tabs", len(tabs)), nil
}

func (b *Browser) restoreSnapshotExact(name, snapshotDir string, store *firefoxSessionStore, profile firefoxProfile, force, dryRun bool) (string, error) {
	target := filepath.Join(profile.Root, "sessionstore.jsonlz4")
	backupDir, err := restoreBackupDir()
	if err != nil {
		return "", err
	}

	if dryRun {
		lines := []string{
			fmt.Sprintf("would stop Firefox (force=%t)", force),
			fmt.Sprintf("would back up session files into %s", backupDir),
			fmt.Sprintf("would write %s", target),
			fmt.Sprintf("would write %s", filepath.Join(profile.Root, "sessionstore-backups", "recovery.jsonlz4")),
			fmt.Sprintf("would launch %s", shellQuoteCommand(append(b.browserCommandParts(), "--new-instance", "--profile", profile.Root))),
		}
		return strings.Join(lines, "\n"), nil
	}

	if err := stopFirefox(force); err != nil {
		return "", err
	}
	backupDir, err = backupFirefoxSessionFiles(profile)
	if err != nil {
		return "", err
	}

	payload, err := os.ReadFile(filepath.Join(snapshotDir, "session.json"))
	if err != nil {
		return "", err
	}

	if err := encodeMozillaLZ4File(target, payload); err != nil {
		return "", err
	}

	backupsDir := filepath.Join(profile.Root, "sessionstore-backups")
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		return "", err
	}
	for _, name := range []string{"recovery.jsonlz4", "recovery.baklz4"} {
		if err := encodeMozillaLZ4File(filepath.Join(backupsDir, name), payload); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(filepath.Join(profile.Root, "sessionCheckpoints.json"), defaultSessionCheckpoints, 0o644); err != nil {
		return "", err
	}

	if err := b.launchFirefoxProfile(profile); err != nil {
		return "", err
	}
	return fmt.Sprintf("restored %s into %s\nbackup: %s", name, profile.Root, backupDir), nil
}

func restoreBackupDir() (string, error) {
	root, err := browserStateRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "_restore-backups", time.Now().Format("20060102-150405")), nil
}

func backupFirefoxSessionFiles(profile firefoxProfile) (string, error) {
	backupDir, err := restoreBackupDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}

	for _, name := range []string{"sessionstore.jsonlz4", "sessionCheckpoints.json"} {
		source := filepath.Join(profile.Root, name)
		if fileExists(source) {
			if err := os.Rename(source, filepath.Join(backupDir, name)); err != nil {
				return "", err
			}
		}
	}

	backupsDir := filepath.Join(profile.Root, "sessionstore-backups")
	if isDir(backupsDir) {
		if err := os.Rename(backupsDir, filepath.Join(backupDir, "sessionstore-backups")); err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		return "", err
	}
	return backupDir, nil
}

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
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
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

func shellQuoteCommand(parts []string) string {
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = strconv.Quote(part)
	}
	return strings.Join(quoted, " ")
}

func browserMode(cfg config.BrowserConfig) string {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		return "urls"
	}
	return mode
}

func optionalArg(args []string, idx int) string {
	if idx >= len(args) {
		return ""
	}
	return args[idx]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
