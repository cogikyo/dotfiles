package browser

// profile.go discovers Firefox profile roots from profiles.ini/installs.ini and resolves profile selectors.

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/config"
)

type firefoxProfile struct {
	Root       string
	Name       string
	InstallKey string
}

type iniFile map[string]map[string]string

// discoverFirefoxProfile resolves a profile: raw path > profiles.ini name > installs.ini default > Default=1.
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
		if isDir(candidate) {
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
					Name: cmp.Or(profiles.get(section, "Name"), filepath.Base(profilePath)),
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
			Name: cmp.Or(profiles.get(section, "Name"), filepath.Base(profilePath)),
		}, nil
	}

	return firefoxProfile{}, fmt.Errorf("could not determine default Firefox profile")
}

// ManagedProfileForSession returns a runtime Firefox profile owned by one layout session.
//
// Profiles live under XDG_STATE_HOME/hyprd/firefox-profiles, falling back to ~/.local/state/hyprd/firefox-profiles.
// The first restore clones the snapshot source profile so exact restores keep browser state.
func ManagedProfileForSession(sessionName string, cfg config.BrowserConfig) (firefoxProfile, error) {
	slug, err := slugifySnapshotName(sessionName)
	if err != nil {
		return firefoxProfile{}, err
	}
	source, err := sourceProfileForBrowserConfig(cfg)
	if err != nil {
		return firefoxProfile{}, err
	}
	root, err := managedFirefoxProfilesRoot()
	if err != nil {
		return firefoxProfile{}, err
	}
	profile := firefoxProfile{
		Root: filepath.Join(root, slug),
		Name: "hyprd-" + slug,
	}
	if isDir(profile.Root) {
		if err := ensureExternalLinksOpenInTabs(profile); err != nil {
			return firefoxProfile{}, err
		}
		return profile, nil
	}
	if err := cloneFirefoxProfile(source.Root, profile.Root); err != nil {
		return firefoxProfile{}, err
	}
	if err := ensureExternalLinksOpenInTabs(profile); err != nil {
		return firefoxProfile{}, err
	}
	return profile, nil
}

func ensureExternalLinksOpenInTabs(profile firefoxProfile) error {
	// Managed profiles are cloned outside the repo-managed user.js path, so keep Firefox remoting tab-safe here too.
	for key, value := range map[string]string{
		"browser.link.open_newwindow":                   "3",
		"browser.link.open_newwindow.override.external": "3",
		"browser.link.open_newwindow.restriction":       "0",
	} {
		if err := setFirefoxPref(profile, key, value); err != nil {
			return err
		}
	}
	return nil
}

func sourceProfileForBrowserConfig(cfg config.BrowserConfig) (firefoxProfile, error) {
	if cfg.Profile != "" {
		return discoverFirefoxProfile(cfg.Profile)
	}
	if cfg.Snapshot != "" {
		dir, err := resolveSnapshotDir(cfg.Snapshot)
		if err != nil {
			return firefoxProfile{}, err
		}
		return restoreProfileForSnapshot(dir, "")
	}
	return discoverFirefoxProfile("")
}

func managedFirefoxProfilesRoot() (string, error) {
	if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" {
		return filepath.Join(stateHome, "hyprd", "firefox-profiles"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "hyprd", "firefox-profiles"), nil
}

func cloneFirefoxProfile(source, target string) error {
	if filepath.Clean(source) == filepath.Clean(target) {
		return nil
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		name := entry.Name()
		if shouldSkipProfileCloneEntry(name) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dest := filepath.Join(target, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()
		if entry.IsDir() {
			return os.MkdirAll(dest, mode.Perm())
		}
		if mode.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, dest)
		}
		if !mode.IsRegular() {
			return nil
		}
		return copyProfileFile(path, dest, mode.Perm())
	})
}

func shouldSkipProfileCloneEntry(name string) bool {
	switch name {
	case ".parentlock", "lock", "parent.lock", "sessionstore.jsonlz4", "sessionCheckpoints.json":
		return true
	case "sessionstore-backups", "startupCache", "cache2", "thumbnails", "shader-cache", "crashes", "minidumps":
		return true
	}
	return strings.HasSuffix(name, ".tmp") || strings.HasSuffix(name, ".sqlite-wal") || strings.HasSuffix(name, ".sqlite-shm")
}

func copyProfileFile(source, target string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
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
			return cmp.Or(profiles.get(section, "Name"), filepath.Base(target))
		}
	}
	return filepath.Base(target)
}

// setFirefoxPref upserts a user_pref line in the profile's prefs.js.
//
// Firefox prefs are last-write-wins, but keeping one line avoids prefs.js churn.
func setFirefoxPref(profile firefoxProfile, key, value string) error {
	prefsPath := filepath.Join(profile.Root, "prefs.js")
	line := fmt.Sprintf("user_pref(\"%s\", %s);", key, value)

	data, err := os.ReadFile(prefsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read pref %s: %w", key, err)
	}

	prefix := fmt.Sprintf("user_pref(\"%s\",", key)
	var lines []string
	replaced := false
	for raw := range strings.SplitSeq(strings.TrimRight(string(data), "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(raw), prefix) {
			if replaced {
				continue
			}
			lines = append(lines, line)
			replaced = true
			continue
		}
		if raw != "" || len(data) > 0 {
			lines = append(lines, raw)
		}
	}
	if !replaced {
		lines = append(lines, line)
	}

	contents := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(prefsPath, []byte(contents), 0o644); err != nil {
		return fmt.Errorf("set pref %s: %w", key, err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
