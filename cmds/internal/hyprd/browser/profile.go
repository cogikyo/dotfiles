package browser

// profile.go discovers Firefox profile roots from profiles.ini/installs.ini and resolves profile selectors.

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"io/fs"
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
	Main       bool
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
			Main:       raw == "",
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
			Main: raw == "",
		}, nil
	}

	return firefoxProfile{}, fmt.Errorf("could not determine default Firefox profile")
}

const mainFirefoxSnapshot = "coms"

func profileForSnapshot(snapshot string, create bool) (firefoxProfile, error) {
	main, err := discoverFirefoxProfile("")
	if err != nil {
		return firefoxProfile{}, err
	}
	slug, err := slugifySnapshotName(snapshot)
	if err != nil {
		return firefoxProfile{}, err
	}
	if slug == mainFirefoxSnapshot {
		main.Main = true
		return main, nil
	}
	return layoutFirefoxProfile(snapshot, main, create)
}

func layoutFirefoxProfile(snapshot string, seed firefoxProfile, create bool) (firefoxProfile, error) {
	slug, err := slugifySnapshotName(snapshot)
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
	if !create || isDir(profile.Root) {
		return profile, nil
	}
	if err := seedFirefoxProfile(seed, profile.Root); err != nil {
		return firefoxProfile{}, err
	}
	return profile, nil
}

func managedFirefoxProfilesRoot() (string, error) {
	if dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); dataHome != "" {
		return filepath.Join(config.ExpandPath(dataHome), "hyprd", "firefox-profiles"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "hyprd", "firefox-profiles"), nil
}

func seedFirefoxProfile(seed firefoxProfile, target string) error {
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, ".seed-*")
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(tmp)
		}
	}()
	if err := copyFirefoxProfile(seed.Root, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		return err
	}
	committed = true
	return nil
}

func refreshFirefoxProfile(seed firefoxProfile, target firefoxProfile) error {
	parent := filepath.Dir(target.Root)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, ".refresh-*")
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(tmp)
		}
	}()
	if err := copyFirefoxProfile(seed.Root, tmp); err != nil {
		return err
	}
	old := target.Root + ".old"
	_ = os.RemoveAll(old)
	if isDir(target.Root) {
		if err := os.Rename(target.Root, old); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, target.Root); err != nil {
		if isDir(old) {
			_ = os.Rename(old, target.Root)
		}
		return err
	}
	_ = os.RemoveAll(old)
	committed = true
	return nil
}

func copyFirefoxProfile(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if skipFirefoxProfilePath(rel, entry) {
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
		switch {
		case mode.IsDir():
			return os.MkdirAll(dest, mode.Perm())
		case mode.Type()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, dest)
		case mode.IsRegular():
			return copyRegularFile(path, dest, mode.Perm())
		default:
			return nil
		}
	})
}

func skipFirefoxProfilePath(rel string, entry fs.DirEntry) bool {
	name := entry.Name()
	switch name {
	case "parent.lock", "lock", ".parentlock", "cache2", "startupCache", "hyprd-restore-backups", "sessionstore-backups":
		return true
	}
	return strings.HasPrefix(rel, "minidumps") || strings.HasPrefix(rel, "crashes")
}

func copyRegularFile(source, target string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
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
