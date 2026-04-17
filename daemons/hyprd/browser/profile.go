package browser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/daemons/config"
)

// firefoxProfile locates a single profile on disk.
//
// InstallKey is the section header from installs.ini (Firefox's per-install hash tying a binary to a profile).
// Empty when the profile was resolved without consulting installs.ini.
type firefoxProfile struct {
	Root       string
	Name       string
	InstallKey string
}

type iniFile map[string]map[string]string

// discoverFirefoxProfile resolves a Firefox profile directory.
//
// Resolution order:
//  1. raw is a path that exists on disk — use it directly.
//  2. raw matches a profiles.ini Name, Path, or basename — use that.
//  3. Otherwise pick the default:
//     a. first installs.ini section with a Default= entry (per-install default), or
//     b. first profiles.ini Profile section with Default=1.
//
// installs.ini takes precedence because Firefox itself prefers it when multiple installs share a profile root.
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

// firefoxRoot returns the directory containing profiles.ini.
//
// The XDG path (~/.config/mozilla/firefox) wins over legacy ~/.mozilla/firefox; Developer Edition defaults to XDG.
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

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
