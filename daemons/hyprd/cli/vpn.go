package cli

import (
	"dotfiles/daemons/config"
	"dotfiles/daemons/hyprd/vpn"
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
