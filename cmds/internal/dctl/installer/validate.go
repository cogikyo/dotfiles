package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/dctl/secureboot"
)

func (s *session) validate(ctx context.Context, p plan) error {
	dir := s.path(target, esp)
	ukis, err := filepath.Glob(filepath.Join(dir, "EFI", "Linux", "*.efi"))
	if err != nil {
		return err
	}
	if len(ukis) == 0 {
		return errors.New("no UKI in /boot/EFI/Linux")
	}
	conf, err := os.ReadFile(filepath.Join(dir, "limine.conf"))
	if err != nil {
		return err
	}
	want := "rd.luks.name=" + p.LUKSID + "=" + mapper
	var errs []error
	for _, uki := range ukis {
		name := filepath.Base(uki)
		if !strings.Contains(string(conf), name) {
			errs = append(errs, fmt.Errorf("limine.conf has no entry for %s", name))
		}
		cmdline, embedded, err := secureboot.Cmdline(uki)
		switch {
		case err != nil:
			errs = append(errs, err)
		case embedded && !strings.Contains(cmdline, want):
			errs = append(errs, fmt.Errorf("%s cmdline %q lacks %s", name, cmdline, want))
		case !embedded && !strings.Contains(string(conf), want):
			errs = append(errs, fmt.Errorf("%s has no embedded cmdline and limine.conf lacks %s", name, want))
		}
	}
	out, err := s.sh.output(ctx, "blkid", "-s", "UUID", "-o", "value", p.Disk.part(2))
	switch got := strings.TrimSpace(string(out)); {
	case err != nil:
		errs = append(errs, err)
	case got != p.LUKSID:
		errs = append(errs, fmt.Errorf("LUKS UUID on %s is %s, cmdline names %s", p.Disk.part(2), got, p.LUKSID))
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	s.u.OK("boot entries match %s, LUKS UUID %s", p.Disk.part(2), p.LUKSID)
	return nil
}
