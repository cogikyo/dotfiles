package home

import (
	"os"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/paths"
)

func Firefox(r paths.Root) doctor.Group {
	userJS := func() ([]link, error) {
		profile, err := FirefoxProfile(r.Home)
		if err != nil {
			return nil, err
		}
		return []link{{r.Config("firefox", "user.js"), filepath.Join(profile, "user.js")}}, nil
	}
	chrome := func() ([]link, error) {
		profile, err := FirefoxProfile(r.Home)
		if err != nil {
			return nil, err
		}
		css, err := filepath.Glob(filepath.Join(r.Home, "vagari", "firefox", "css", "*"))
		if err != nil || len(css) == 0 {
			return nil, doctor.Block("vagari Firefox CSS missing; run dctl repos sync")
		}
		var out []link
		for _, src := range css {
			out = append(out, link{src, filepath.Join(profile, "chrome", filepath.Base(src))})
		}
		return out, nil
	}
	return doctor.Group{Name: "firefox", Checks: []doctor.Check{
		linkCheck("firefox-user-js", r.Dotfiles, "restart Firefox after fixing", userJS),
		linkCheck("firefox-chrome", r.Dotfiles, "restart Firefox after fixing", chrome),
	}}
}

func FirefoxProfile(home string) (string, error) {
	for _, dir := range []string{filepath.Join(home, ".mozilla", "firefox"), filepath.Join(home, ".config", "mozilla", "firefox")} {
		data, err := os.ReadFile(filepath.Join(dir, "profiles.ini"))
		if err != nil {
			continue
		}
		if path := devProfile(string(data)); path != "" {
			if !filepath.IsAbs(path) {
				path = filepath.Join(dir, path)
			}
			if st, err := os.Stat(path); err == nil && st.IsDir() {
				return path, nil
			}
		}
	}
	return "", doctor.Block("Firefox Developer Edition profile not found; launch it once first")
}

func devProfile(ini string) string {
	name, path := "", ""
	for line := range strings.Lines(ini + "\n[") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			if name == "dev-edition-default" && path != "" {
				return path
			}
			name, path = "", ""
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		switch strings.TrimSpace(key) {
		case "Name":
			name = strings.TrimSpace(value)
		case "Path":
			path = strings.TrimSpace(value)
		}
	}
	return ""
}
