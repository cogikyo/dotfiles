package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"dotfiles/cmds/internal/dctl/iso"

	"golang.org/x/sys/unix"
)

const (
	defaultUser = "cullyn"
	defaultZone = "America/Los_Angeles"

	testLabel = "/dev/disk/by-label/" + iso.Label
	testMount = "/run/dctltest"
	testEnv   = "DCTLTEST_OVERRIDE"

	overrideCopy = "/run/dctl-override"
	lockFile     = "/run/dctl-install.lock"
)

func (s *session) form(context.Context) (iso.Answers, error) {
	var a iso.Answers
	var err error
	if a.User, err = s.text("Username", defaultUser, func(v string) error {
		if !userPattern.MatchString(v) || v == "root" {
			return errors.New("want a lowercase POSIX name other than root")
		}
		return nil
	}); err != nil {
		return a, err
	}
	if a.Password, err = s.secret("Password"); err != nil {
		return a, err
	}
	if a.Host, err = s.text("Hostname", "", func(v string) error {
		if !hostPattern.MatchString(v) {
			return errors.New("want one lowercase DNS label")
		}
		return nil
	}); err != nil {
		return a, err
	}
	if a.Zone, err = s.text("Timezone", defaultZone, s.zone); err != nil {
		return a, err
	}
	a.LUKS, err = s.secret("Disk passphrase")
	return a, err
}

func (s *session) zone(v string) error {
	if !filepath.IsLocal(v) {
		return fmt.Errorf("%q is not a zone name", v)
	}
	if st, err := os.Stat(s.path("/usr/share/zoneinfo", v)); err != nil || !st.Mode().IsRegular() {
		return fmt.Errorf("%q is not in /usr/share/zoneinfo", v)
	}
	if _, err := time.LoadLocation(v); err != nil {
		return err
	}
	return nil
}

func (s *session) text(label, initial string, valid func(string) error) (string, error) {
	for {
		v, err := s.u.Text(label, initial)
		if err != nil {
			return "", err
		}
		if err := valid(v); err != nil {
			s.u.Warn("%s: %v", label, err)
			initial = v
			continue
		}
		return v, nil
	}
}

func (s *session) secret(label string) (string, error) {
	for {
		v, err := s.u.Secret(label)
		if err != nil {
			return "", err
		}
		if v == "" {
			s.u.Warn("%s: empty", label)
			continue
		}
		again, err := s.u.Secret(label + " again")
		if err != nil {
			return "", err
		}
		if again != v {
			s.u.Warn("%s: entries differ", label)
			continue
		}
		return v, nil
	}
}

func (s *session) dctltest(ctx context.Context) (*iso.Answers, error) {
	if _, err := os.Stat(s.path(testLabel)); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if _, err := s.sh.output(ctx, "systemd-detect-virt", "--vm"); err != nil {
		return nil, fmt.Errorf("refusing unattended install: a DCTLTEST drive is attached but this is not a VM (%v)", err)
	}
	if os.Getenv(testEnv) != "" {
		s.testMounted = true
	} else {
		if err := os.MkdirAll(s.path(testMount), 0o755); err != nil {
			return nil, err
		}
		if err := s.sh.run(ctx, nil, "mount", "-o", "ro", testLabel, testMount); err != nil {
			return nil, err
		}
		s.testMounted = true
		if override := s.path(testMount, "dctl"); exists(override) {
			s.u.Info("re-executing the DCTLTEST dctl")
			err := s.reexec(override)
			return nil, errors.Join(err, s.unmountTest(context.WithoutCancel(ctx)))
		}
	}
	data, err := os.ReadFile(s.path(testMount, "answers.json"))
	if err != nil {
		return nil, err
	}
	var a iso.Answers
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return nil, fmt.Errorf("DCTLTEST answers.json: %w", err)
	}
	if a.User == "" || a.Password == "" || a.Host == "" || a.Zone == "" || a.LUKS == "" || a.Serial == "" {
		return nil, errors.New("DCTLTEST answers.json: user, password, host, zone, luks, and disk_serial are all required")
	}
	if err := s.zone(a.Zone); err != nil {
		return nil, fmt.Errorf("DCTLTEST answers.json: %w", err)
	}
	if b := s.path(testMount, "dotfiles.bundle"); exists(b) {
		s.bundle = filepath.Join(testMount, "dotfiles.bundle")
	}
	return &a, nil
}

func (s *session) unmountTest(ctx context.Context) error {
	if !s.testMounted {
		return nil
	}
	if err := s.sh.run(ctx, nil, "umount", testMount); err != nil {
		return err
	}
	s.testMounted = false
	return nil
}

var execve = syscall.Exec

func (s *session) reexec(override string) error {
	dst := s.path(overrideCopy)
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".dctl-override-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	in, err := os.Open(override)
	if err != nil {
		return errors.Join(err, tmp.Close())
	}
	_, err = io.Copy(tmp, in)
	if err := errors.Join(err, in.Close(), tmp.Chmod(0o755), tmp.Close()); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return err
	}
	fd := int(s.lock.Fd())
	if _, err := unix.FcntlInt(uintptr(fd), unix.F_SETFD, 0); err != nil {
		return fmt.Errorf("keep install lock across exec: %w", err)
	}
	env := append(os.Environ(), testEnv+"="+strconv.Itoa(fd))
	err = execve(dst, os.Args, env)
	unix.CloseOnExec(fd)
	return fmt.Errorf("exec %s: %w", dst, err)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
