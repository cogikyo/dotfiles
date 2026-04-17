package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Paths returned by this file are $HOME-relative; loadYAMLFile joins them with os.UserHomeDir().
const configsDir = "dotfiles/daemons/configs"

// ConfigPath returns the $HOME-relative path to a daemon's YAML config.
func ConfigPath(name string) string {
	return filepath.Join(configsDir, name+".yaml")
}

// LocalConfigPath returns the $HOME-relative path to a daemon's gitignored local override.
func LocalConfigPath(name string) string {
	return filepath.Join(configsDir, name+".local.yaml")
}

// ExpandPath resolves a leading "~/" against the current user's home directory.
// Returns the input unchanged on missing prefix or home-lookup failure.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
