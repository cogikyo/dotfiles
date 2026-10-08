package home

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/setup"
)

func Firefox(r paths.Root, run execx.Runner) setup.Stage {
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
			return nil, setup.Manual("vagari Firefox CSS missing; run dctl setup repos")
		}
		var out []link
		for _, src := range css {
			out = append(out, link{src, filepath.Join(profile, "chrome", filepath.Base(src))})
		}
		return out, nil
	}
	profile := setup.Item{
		Name: "firefox-profile",
		Check: func(context.Context) error {
			if _, err := FirefoxProfile(r.Home); err != nil {
				return errors.New("Firefox Developer Edition profile missing")
			}
			return nil
		},
		Fix: func(ctx context.Context) error {
			if _, err := FirefoxProfile(r.Home); err == nil {
				return nil
			}
			if running() {
				return setup.Manual("quit Firefox, then run dctl setup firefox")
			}
			dir, err := os.MkdirTemp("", "dctl-firefox-")
			if err != nil {
				return err
			}
			defer os.RemoveAll(dir)
			return execx.Reason(run, "create the Developer Edition profile").Run(ctx, "", "firefox-developer-edition", "--headless", "--screenshot", filepath.Join(dir, "first-run.png"), "about:blank")
		},
	}
	return setup.Stage{Name: "firefox", Items: []setup.Item{
		profile,
		linkCheck("firefox-user-js", r.Dotfiles, "restart Firefox after setup", userJS),
		linkCheck("firefox-chrome", r.Dotfiles, "restart Firefox after setup", chrome),
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
	return "", setup.Manual("Firefox Developer Edition profile not found; run dctl setup firefox")
}

func running() bool {
	comms, _ := filepath.Glob("/proc/[0-9]*/comm")
	for _, comm := range comms {
		if data, err := os.ReadFile(comm); err == nil && strings.TrimSpace(string(data)) == "firefox" {
			return true
		}
	}
	return false
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
