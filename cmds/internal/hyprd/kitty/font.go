package kitty

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// fontPath ignores XDG_STATE_HOME because kitty.conf includes this literal path and Kitty's include cannot fall back.
func fontPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "hyprd", "kitty.conf"), nil
}

// SetFontSize writes size to the file kitty.conf includes, then has every running Kitty reload its config.
//
// Reloading, unlike set-font-size, also moves the size that new OS windows and the font-size reset use.
func SetFontSize(size float64) error {
	path, err := fontPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, fmt.Appendf(nil, "font_size %s\n", strconv.FormatFloat(size, 'f', -1, 64)), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}

	pids, err := socketPIDs()
	if err != nil {
		return err
	}
	var errs []error
	for _, pid := range pids {
		if err := NewClient(pid).loadConfig(); err != nil {
			errs = append(errs, fmt.Errorf("kitty %d: %w", pid, err))
		}
	}
	return errors.Join(errs...)
}

func (k *Client) loadConfig() error {
	out, err := exec.Command("kitty", "@", "--to", k.socketPath, "load-config").CombinedOutput()
	if err != nil {
		return fmt.Errorf("load-config: %w: %s", err, out)
	}
	return nil
}
