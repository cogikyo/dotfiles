package vpn

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/secrets"
	"dotfiles/cmds/internal/dctl/setup"
	"dotfiles/cmds/internal/dctl/ui"
)

func Stage(u *ui.UI, r paths.Root, run execx.Runner) setup.Stage {
	return setup.Stage{Name: "vpn", Items: []setup.Item{{
		Name: "vpn-imported",
		Check: func(ctx context.Context) error {
			_, missing, err := absent(ctx, r, run)
			if err != nil || len(missing) == 0 {
				return err
			}
			return fmt.Errorf("not in NetworkManager: %s", strings.Join(missing, ", "))
		},
		Fix: func(ctx context.Context) (err error) {
			conns, missing, err := absent(ctx, r, run)
			if err != nil || len(missing) == 0 {
				return err
			}
			entries, err := secrets.Manifest(r)
			if err != nil {
				return err
			}
			var names, fresh []string
			for _, name := range missing {
				profile := conns[name].Path(name)
				i := slices.IndexFunc(entries, func(e secrets.Entry) bool { return paths.ExpandHome(r.Home, e.Path()) == profile })
				if i < 0 {
					return fmt.Errorf("vpn %s: no secret in secrets/manifest targets %s", name, profile)
				}
				names = append(names, entries[i].Name)
				if _, err := os.Lstat(profile); errors.Is(err, fs.ErrNotExist) {
					fresh = append(fresh, profile)
				}
			}
			defer func() {
				for _, profile := range fresh {
					if rerr := os.Remove(profile); rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
						err = errors.Join(err, rerr)
					}
				}
			}()
			if err := secrets.Decrypt(u, r, names); err != nil {
				return err
			}
			for _, name := range missing {
				if err := run.Run(ctx, "", "hyprd", "vpn", "install", name); err != nil {
					return err
				}
			}
			return nil
		},
	}}}
}

func absent(ctx context.Context, r paths.Root, run execx.Runner) (map[string]config.VPNConnection, []string, error) {
	data, err := os.ReadFile(filepath.Join(r.Dotfiles, "cmds", "config", "hyprd.yaml"))
	if err != nil {
		return nil, nil, err
	}
	var cfg struct {
		VPN config.VPNConfig `yaml:"vpn"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, nil, fmt.Errorf("parse hyprd.yaml: %w", err)
	}
	conns := cfg.VPN.Connections
	if len(conns) == 0 {
		return conns, nil, nil
	}
	if _, err := exec.LookPath("nmcli"); err != nil {
		return nil, nil, setup.Manual("nmcli not found; install networkmanager from base.lst")
	}
	out, err := run.Output(ctx, "", "nmcli", "-t", "-f", "NAME", "connection", "show")
	if err != nil {
		return nil, nil, setup.Manual("nmcli connection show: %v", err)
	}
	have := strings.Split(out, "\n")
	var missing []string
	for _, name := range slices.Sorted(maps.Keys(conns)) {
		if !slices.Contains(have, name) {
			missing = append(missing, name)
		}
	}
	return conns, missing, nil
}
