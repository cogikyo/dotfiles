package config

import (
	"os"
	"path/filepath"
	"strings"
)

const configsDir = "dotfiles/daemons/configs"

// ConfigPath returns the relative config path for a daemon YAML file.
func ConfigPath(name string) string {
	return filepath.Join(configsDir, name+".yaml")
}

// LocalConfigPath returns the relative path for a daemon-specific local override.
func LocalConfigPath(name string) string {
	return filepath.Join(configsDir, name+".local.yaml")
}

// ExpandPath converts "~/..." into an absolute path under the current home.
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
