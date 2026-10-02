package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
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
	p.wg.Go(func() { p.errs[1] = verify(ctx, s.path(iso.Payload)) })
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

type sum struct{ name, hex string }

func verify(ctx context.Context, dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	data, err := root.ReadFile(iso.Sums)
	if err != nil {
		return err
	}
	var sums []sum
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSuffix(line, "\n")
		if len(line) < 67 || (line[64:66] != "  " && line[64:66] != " *") {
			return fmt.Errorf("%s: malformed line %q", iso.Sums, line)
		}
		sums = append(sums, sum{line[66:], line[:64]})
	}
	if len(sums) == 0 {
		return fmt.Errorf("%s lists no files", iso.Sums)
	}

	jobs := make(chan int)
	errs := make([]error, len(sums))
	var wg sync.WaitGroup
	for range runtime.NumCPU() {
		wg.Go(func() {
			for i := range jobs {
				errs[i] = sums[i].check(root)
			}
		})
	}
feed:
	for i := range sums {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("payload verification: %w", err)
	}
	return nil
}

func (s sum) check(root *os.Root) error {
	f, err := root.Open(s.name)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("%s: %w", s.name, err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != s.hex {
		return fmt.Errorf("%s: sha256 %s, want %s", s.name, got, s.hex)
	}
	return nil
}
