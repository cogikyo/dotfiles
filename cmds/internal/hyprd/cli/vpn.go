package cli

// vpn.go bridges hyprd's CLI-only VPN verb to the NetworkManager-backed vpn package.

import (
	"dotfiles/cmds/internal/config"
	"dotfiles/cmds/internal/hyprd/vpn"
	"fmt"
	"os"
	"strings"
)

// VPN dispatches VPN commands directly so install/export can use prompts and sudo.
func VPN() {
	cfg := config.LoadHypr()
	cmd := vpn.New(&cfg.VPN)
	result, err := cmd.Execute(strings.Join(os.Args[2:], " "))
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd vpn: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result)
}
