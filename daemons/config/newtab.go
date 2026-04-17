package config

// NewtabConfig configures the newtab HTTP server.
type NewtabConfig struct {
	Port         string `yaml:"port"`         // listen address, including leading colon (e.g. ":42069")
	FirefoxDB    string `yaml:"firefox_db"`   // $HOME-relative path to Firefox places.sqlite
	StaticDir    string `yaml:"static_dir"`   // $HOME-relative path to static asset root
	HistoryLimit int    `yaml:"history_limit"` // max recent-history rows surfaced to the page
}

// DefaultNewtab returns sensible defaults for the newtab server.
//
// FirefoxDB is left empty; callers resolve it by scanning Firefox profile roots at startup.
func DefaultNewtab() NewtabConfig {
	return NewtabConfig{
		Port:         ":42069",
		StaticDir:    "dotfiles/daemons/newtab",
		HistoryLimit: 15,
	}
}
