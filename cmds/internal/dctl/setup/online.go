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
)

var hosts = []string{"archlinux.org", "aur.archlinux.org", "github.com"}

func Online(ctx context.Context) error {
	if !carrier() {
		return errors.New("Ethernet not connected — plug in the cable, then rerun dctl setup")
	}
	for _, host := range hosts {
		lookup, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := net.DefaultResolver.LookupHost(lookup, host)
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return fmt.Errorf("cannot resolve %s — check `resolvectl status`, then rerun dctl setup", host)
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
