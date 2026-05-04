package config

// newtab.go defines newtab server settings and defaults.

// NewtabConfig configures the newtab HTTP server.
type NewtabConfig struct {
	Port         string `yaml:"port"`          // listen address, e.g. ":42069"
	FirefoxDB    string `yaml:"firefox_db"`    // $HOME-relative path to Firefox places.sqlite
	StaticDir    string `yaml:"static_dir"`    // $HOME-relative path to static asset root
	HistoryLimit int    `yaml:"history_limit"` // max recent-history rows surfaced to the page
}

// DefaultNewtab returns newtab defaults; FirefoxDB is resolved at runtime by scanning Firefox profiles.
func DefaultNewtab() NewtabConfig {
	return NewtabConfig{
		Port:         ":42069",
		StaticDir:    "dotfiles/cmds/cmd/newtab",
		HistoryLimit: 15,
	}
}
