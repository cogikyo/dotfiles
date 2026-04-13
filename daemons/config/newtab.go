package config

// NewtabConfig defines settings for the newtab HTTP server.
type NewtabConfig struct {
	Port         string `yaml:"port"`
	FirefoxDB    string `yaml:"firefox_db"`
	StaticDir    string `yaml:"static_dir"`
	HistoryLimit int    `yaml:"history_limit"`
}

// DefaultNewtab returns sensible defaults for the newtab server.
func DefaultNewtab() NewtabConfig {
	return NewtabConfig{
		Port:         ":42069",
		FirefoxDB:    ".mozilla/firefox/sdfm8kqz.dev-edition-default/places.sqlite",
		StaticDir:    "dotfiles/daemons/newtab",
		HistoryLimit: 15,
	}
}
