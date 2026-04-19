package config

// load.go loads daemon YAML files and falls back to compiled defaults on error.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load returns the combined configuration for every daemon.
//
// Errors are logged to stderr and never fatal.
func Load() *Config {
	return &Config{
		Eww:    LoadEww(),
		Hypr:   LoadHypr(),
		Newtab: LoadNewtab(),
	}
}

// LoadEww loads ewwd config from configs/ewwd.yaml.
func LoadEww() EwwConfig {
	cfg := DefaultEww()
	if err := loadYAMLFile(ConfigPath("ewwd"), &cfg); err != nil {
		logConfigError("ewwd", err)
	}
	return cfg
}

// LoadHypr loads hyprd config from configs/hyprd.yaml.
//
// Sound and app maps are lowercased for case-insensitive libnotify matching.
func LoadHypr() HyprConfig {
	cfg := DefaultHypr()
	if err := loadYAMLFile(ConfigPath("hyprd"), &cfg); err != nil {
		logConfigError("hyprd", err)
	}
	cfg.Notify.UrgencySounds = lowercaseKeys(cfg.Notify.UrgencySounds)
	cfg.Notify.AppSounds = lowercaseKeys(cfg.Notify.AppSounds)
	cfg.Notify.FocusApps = lowercaseKeys(cfg.Notify.FocusApps)
	cfg.Notify.ActionFocusApps = lowercaseFocusTargets(cfg.Notify.ActionFocusApps)
	cfg.Notify.SilentApps = lowercaseSlice(cfg.Notify.SilentApps)
	return cfg
}

// LoadNewtab returns newtab defaults (no YAML; Firefox DB is resolved at runtime).
func LoadNewtab() NewtabConfig {
	return DefaultNewtab()
}

func loadYAMLFile(relPath string, dst any) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(home, relPath))
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, dst)
}

func logConfigError(section string, err error) {
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	fmt.Fprintf(os.Stderr, "daemons: %s config error: %v\n", section, err)
}

func lowercaseKeys(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[strings.ToLower(k)] = v
	}
	return out
}

func lowercaseSlice(s []string) []string {
	out := make([]string, len(s))
	for i, v := range s {
		out[i] = strings.ToLower(v)
	}
	return out
}

func lowercaseFocusTargets(m map[string]NotifyFocusTarget) map[string]NotifyFocusTarget {
	out := make(map[string]NotifyFocusTarget, len(m))
	for k, v := range m {
		out[strings.ToLower(k)] = v
	}
	return out
}
