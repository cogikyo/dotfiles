package system

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

const (
	resolv = "etc/resolv.conf"
	stub   = "/run/systemd/resolve/stub-resolv.conf"
)

func checkResolv(rootDir string) error {
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	target, err := root.Readlink(resolv)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("/%s missing; want symlink to %s, then restart NetworkManager", resolv, stub)
	case err != nil:
		return fmt.Errorf("/%s is not a symlink to %s (%v); restart NetworkManager after fixing", resolv, stub, err)
	case target != stub:
		return fmt.Errorf("/%s points to %s, want %s; restart NetworkManager after fixing", resolv, target, stub)
	}
	return nil
}

func linkResolv(rootDir string) error {
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.MkdirAll("etc", 0o755); err != nil {
		return err
	}
	tmp := "etc/.resolv.conf.dctl-" + rand.Text()[:8]
	if err := root.Symlink(stub, tmp); err != nil {
		return err
	}
	if err := root.Rename(tmp, resolv); err != nil {
		_ = root.Remove(tmp)
		return err
	}
	return nil
}
