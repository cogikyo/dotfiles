package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"dotfiles/cmds/internal/dctl/iso"
	"dotfiles/cmds/internal/dctl/packages"
	"dotfiles/cmds/internal/dctl/secureboot"
)

type prep struct {
	wg       sync.WaitGroup
	disks    survey
	firmware secureboot.Firmware
	targets  []string
	errs     [4]error
}

func (s *session) prepare(ctx context.Context) *prep {
	p := &prep{}
	p.wg.Go(func() {
		boot, out, err := s.lsblk(ctx)
		if err == nil {
			p.disks, err = scan(out, boot)
		}
		p.errs[0] = err
	})
	p.wg.Go(func() { p.errs[1] = iso.VerifyPayload(ctx, s.path(iso.Payload)) })
	p.wg.Go(func() {
		p.firmware, p.errs[2] = secureboot.Read(s.path("/sys/firmware/efi"))
		if p.errs[2] == nil && !p.firmware.UEFI {
			p.errs[2] = errors.New("not booted with UEFI; Limine is installed for UEFI only")
		}
	})
	p.wg.Go(func() {
		var err error
		p.targets, err = packages.Read(s.path(iso.Targets))
		if err == nil && len(p.targets) == 0 {
			err = fmt.Errorf("%s names no packages", iso.Targets)
		}
		p.errs[3] = err
	})
	return p
}

func (p *prep) wait() error {
	p.wg.Wait()
	return errors.Join(p.errs[:]...)
}

func (s *session) lsblk(ctx context.Context) (boot string, out []byte, err error) {
	cmdline, err := os.ReadFile(s.path("/proc/cmdline"))
	if err != nil {
		return "", nil, err
	}
	for field := range strings.FieldsSeq(string(cmdline)) {
		if v, ok := strings.CutPrefix(field, "archisosearchuuid="); ok {
			boot = "/dev/disk/by-uuid/" + v
		}
	}
	if boot == "" {
		return "", nil, errors.New("boot medium not identified: no archisosearchuuid= in /proc/cmdline")
	}
	out, err = s.sh.output(ctx, "lsblk", "-J", "-b", "-o", "NAME,PATH,TYPE,SIZE,RO,RM,TRAN,MODEL,SERIAL,WWN,LOG-SEC,UUID,PARTUUID,LABEL,MOUNTPOINTS")
	return boot, out, err
}
