package setup

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dotfiles/cmds/internal/ui"
)

var hosts = []string{"archlinux.org", "aur.archlinux.org", "github.com"}

func Network(u *ui.UI) Stage {
	return Stage{Name: "network", Items: []Item{{
		Name:  "network-online",
		Check: online,
		Fix: func(ctx context.Context) error {
			for {
				err := online(ctx)
				if err == nil || ctx.Err() != nil || !u.Can() {
					return err
				}
				i, serr := u.Select(err.Error(), []string{"Retry", "Skip online steps"}, 0)
				switch {
				case serr != nil:
					return serr
				case i == 1:
					return err
				}
			}
		},
	}}}
}

func online(ctx context.Context) error {
	if !carrier() {
		return errors.New("Ethernet not connected")
	}
	for _, host := range hosts {
		lookup, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := net.DefaultResolver.LookupHost(lookup, host)
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return fmt.Errorf("cannot resolve %s; check `resolvectl status`", host)
		}
	}
	return nil
}

func carrier() bool {
	links, _ := filepath.Glob("/sys/class/net/*/carrier")
	for _, path := range links {
		dev := filepath.Base(filepath.Dir(path))
		if dev == "lo" {
			continue
		}
		if _, err := os.Stat(filepath.Join("/sys/class/net", dev, "device")); err != nil {
			continue
		}
		if data, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(data)) == "1" {
			return true
		}
	}
	return false
}
