package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load returns the combined daemon configuration using the split YAML files.
func Load() *Config {
	return &Config{
		Eww:    LoadEww(),
		Hypr:   LoadHypr(),
		Newtab: LoadNewtab(),
	}
}

// LoadEww loads the ewwd daemon config from configs/ewwd.yaml.
func LoadEww() EwwConfig {
	cfg := DefaultEww()
	if err := loadYAMLFile(ConfigPath("ewwd"), &cfg); err != nil {
		logConfigError("ewwd", err)
	}
	return cfg
}

// LoadHypr loads the hyprd daemon config from configs/hyprd.yaml.
func LoadHypr() HyprConfig {
	cfg := DefaultHypr()
	if err := loadYAMLFile(ConfigPath("hyprd"), &cfg); err != nil {
		logConfigError("hyprd", err)
	}
	cfg.Notify.UrgencySounds = lowercaseKeys(cfg.Notify.UrgencySounds)
	cfg.Notify.AppSounds = lowercaseKeys(cfg.Notify.AppSounds)
	cfg.Notify.SilentApps = lowercaseSlice(cfg.Notify.SilentApps)
	return cfg
}

// LoadNewtab loads the newtab config and overlays configs/newtab.local.yaml if present.
func LoadNewtab() NewtabConfig {
	cfg := DefaultNewtab()
	if err := loadYAMLFile(ConfigPath("newtab"), &cfg); err != nil {
		logConfigError("newtab", err)
	}
	if err := loadYAMLFile(LocalConfigPath("newtab"), &cfg); err != nil && !errors.Is(err, os.ErrNotExist) {
		logConfigError("newtab local", err)
	}
	return cfg
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
