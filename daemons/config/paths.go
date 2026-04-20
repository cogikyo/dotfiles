package config

// paths.go resolves daemon config file locations and expands "~/" paths.

import (
	"os"
	"path/filepath"
	"strings"
)

const configsDir = "dotfiles/daemons/config"

// ConfigPath returns the $HOME-relative path to a daemon's YAML config.
func ConfigPath(name string) string {
	return filepath.Join(configsDir, name+".yaml")
}

// LocalConfigPath returns the $HOME-relative path to a daemon's gitignored local override.
func LocalConfigPath(name string) string {
	return filepath.Join(configsDir, name+".local.yaml")
}

// ExpandPath resolves a leading "~/" against the current user's home directory.
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
