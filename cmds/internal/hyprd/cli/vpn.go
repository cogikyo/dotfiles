package cli

import (
	"context"
	"errors"
	"os"
	"strings"

	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/vpn"
	"dotfiles/cmds/internal/ui"
)

// VPN dispatches VPN commands directly so install/export can use prompts and sudo.
func VPN() {
	cfg := config.LoadHypr()
	u := ui.New(ui.Options{Context: context.Background()})
	cmd := vpn.New(&cfg.VPN)
	if u.Can() {
		cmd.Secret = u.Secret
	}
	args := strings.Join(os.Args[2:], " ")
	u.Open("hyprd vpn %s", args)
	u.Trap()
	result, err := cmd.Execute(args)
	switch {
	case errors.Is(err, ui.ErrCanceled):
		u.Close(ui.Warn, "canceled")
		os.Exit(130)
	case err != nil:
		u.Error("%v", err)
		os.Exit(1)
	}
	for l := range strings.Lines(strings.TrimSpace(result)) {
		u.Info("%s", l)
	}
	u.Close(ui.OK, "done")
}
