package home

import (
	"path/filepath"

	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/setup"
)

var macConfigs = []string{"atuin", "btop", "git", "nvim", "shell", "starship.toml", "xplr", "zsh"}

func Mac(r paths.Root) setup.Stage {
	return setup.Stage{Name: "home", Items: []setup.Item{
		linkCheck("home-links", r.Dotfiles, "", func() ([]link, error) {
			out := []link{
				{r.Config("zsh", "zshrc"), filepath.Join(r.Home, ".zshrc")},
				{r.Config("zsh", "zshenv"), filepath.Join(r.Home, ".zshenv")},
				{r.Config("lazygit"), filepath.Join(r.Home, "Library", "Application Support", "lazygit")},
			}
			for _, name := range macConfigs {
				out = append(out, link{r.Config(name), filepath.Join(r.Home, ".config", name)})
			}
			return out, nil
		}),
	}}
}
