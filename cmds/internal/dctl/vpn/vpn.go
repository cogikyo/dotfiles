package vpn

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
)

func Group(r paths.Root, run execx.Runner) doctor.Group {
	return doctor.Group{Name: "vpn", Online: true, Checks: []doctor.Check{{
		Name: "vpn-imported",
		Check: func(ctx context.Context) error {
			data, err := os.ReadFile(filepath.Join(r.Dotfiles, "cmds", "config", "hyprd.yaml"))
			if err != nil {
				return err
			}
			var cfg struct {
				VPN config.VPNConfig `yaml:"vpn"`
			}
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return fmt.Errorf("parse hyprd.yaml: %w", err)
			}
			if len(cfg.VPN.Connections) == 0 {
				return nil
			}
			if _, err := exec.LookPath("nmcli"); err != nil {
				return doctor.Block("nmcli not found; install networkmanager from base.lst")
			}
			out, err := run.Output(ctx, "", "nmcli", "-t", "-f", "NAME", "connection", "show")
			if err != nil {
				return doctor.Block("nmcli connection show: %v", err)
			}
			have := strings.Split(out, "\n")
			var missing []string
			for _, name := range slices.Sorted(maps.Keys(cfg.VPN.Connections)) {
				if !slices.Contains(have, name) {
					missing = append(missing, name)
				}
			}
			if len(missing) > 0 {
				return fmt.Errorf("not in NetworkManager: %s; run hyprd vpn install after secrets", strings.Join(missing, ", "))
			}
			return nil
		},
	}}}
}
