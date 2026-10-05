package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"dotfiles/cmds/internal/dctl/binaries"
	"dotfiles/cmds/internal/dctl/iso"
	"dotfiles/cmds/internal/ui"
)

const (
	hooks   = "/etc/pacman.d/hooks"
	staged  = "/var/tmp/dotfiles.bundle"
	origin  = "git@github.com:cogikyo/dotfiles.git"
	mirror  = "Server = https://geo.mirror.pkgbuild.com/$repo/os/$arch\n"
	console = "/dev/ttyS0"
)

var (
	masked     = []string{"60-mkinitcpio-remove.hook", "90-mkinitcpio-install.hook", "60-limine-mkinitcpio-remove-pre.hook", "80-limine-efi-deploy.hook", "90-limine-mkinitcpio-remove-post.hook"}
	rootStages = []string{"system", "packages"}
	userStages = []string{"home"}
)

type session struct {
	u           *ui.UI
	sh          shell
	root        string
	exe         string
	bundle      string
	testMounted bool
	lock        *os.File
	opened      bool
	mounted     bool
	ask         func(context.Context) (iso.Answers, error)
	confirm     func(disk) error
	times       []iso.Phase
}

type step struct {
	name string
	do   func(context.Context) error
}

func Run(ctx context.Context, u *ui.UI) error {
	if os.Geteuid() != 0 {
		return errors.New("dctl install must run as root")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	s := &session{u: u, sh: host{u}, root: "/", exe: exe}
	s.ask, s.confirm = s.form, s.typed
	return s.main(ctx)
}

func (s *session) path(parts ...string) string {
	return filepath.Join(append([]string{s.root}, parts...)...)
}

func (s *session) main(ctx context.Context) (err error) {
	for _, p := range []string{"/run/archiso", iso.Payload} {
		if _, err := os.Stat(s.path(p)); err != nil {
			return fmt.Errorf("refusing to install outside the dctl ISO: %w", err)
		}
	}
	s.bundle = iso.Bundle
	defer func() { err = errors.Join(err, s.unmountTest(context.WithoutCancel(ctx))) }()
	test, err := s.dctltest(ctx)
	if err != nil {
		return err
	}
	if err := s.acquire(); err != nil {
		return err
	}
	defer s.lock.Close()
	if test != nil {
		defer func() {
			if err != nil {
				s.report(err)
			}
		}()
	}

	ctx, cancel := context.WithCancel(ctx)
	prep := s.prepare(ctx)
	defer func() {
		cancel()
		prep.wg.Wait()
	}()
	var a iso.Answers
	if test != nil {
		a = *test
	} else if a, err = s.ask(ctx); err != nil {
		return err
	}
	if err := s.u.Spin(ctx, "preparing", func(context.Context) error { return prep.wait() }); err != nil {
		return err
	}
	s.u.OK("payload verified; %d packages to install", len(prep.targets))

	d, err := choose(s.u, prep.disks, test)
	if err != nil {
		return err
	}
	p := newPlan(d, a.Zone)
	if test == nil {
		if err := s.confirm(p.Disk); err != nil {
			return err
		}
	}
	boot, out, err := s.lsblk(ctx)
	if err != nil {
		return err
	}
	if err := p.Disk.recheck(out, boot); err != nil {
		return err
	}
	if err := s.vacant(); err != nil {
		return err
	}

	start := time.Now()
	if err := s.install(ctx, p, a, prep.targets); err != nil {
		return errors.Join(err, errModified)
	}
	if test != nil {
		s.report(nil)
	}
	s.u.OK("installed %s on %s in %s", machine, p.Disk.Path, time.Since(start).Round(time.Second))
	s.u.Detail("after first login, run `dctl setup`; Secure Boot keys enroll later with `sudo dctl setup secureboot`")
	if test == nil {
		ok, err := s.u.Confirm("Reboot now?")
		if err != nil || !ok {
			return err
		}
	}
	if err := s.unmountTest(context.WithoutCancel(ctx)); err != nil {
		return err
	}
	return s.sh.run(context.WithoutCancel(ctx), nil, "systemctl", "reboot")
}

func choose(u *ui.UI, found survey, test *iso.Answers) (disk, error) {
	if test != nil {
		i := slices.IndexFunc(found.Candidates, func(d disk) bool { return d.Serial == test.Serial })
		if i < 0 {
			return disk{}, fmt.Errorf("DCTLTEST: no installable disk has serial %q", test.Serial)
		}
		return found.Candidates[i], nil
	}
	for _, r := range found.Refused {
		u.Info("skipping %s: %s", r.Disk.Path, r.Reason)
	}
	d, err := found.pick("")
	if !errors.Is(err, errAmbiguous) {
		return d, err
	}
	labels := make([]string, len(found.Candidates))
	for i, c := range found.Candidates {
		labels[i] = fmt.Sprintf("%s  %s  %.0f GB  %s", c.Path, c.Model, float64(c.Size)/1e9, c.Serial)
	}
	i, err := u.Select("Install to which disk?", labels, 0)
	if err != nil {
		return disk{}, err
	}
	return found.Candidates[i], nil
}

func (s *session) typed(d disk) error {
	s.u.Section("erase", d.Path)
	s.u.KV("model", d.Model)
	s.u.KV("size", fmt.Sprintf("%.0f GB", float64(d.Size)/1e9))
	s.u.KV("serial", d.Serial)
	if d.WWN != "" {
		s.u.KV("wwn", d.WWN)
	}
	s.u.Warn("installing permanently erases %s and all its partitions and data", d.Path)
	v, err := s.u.Text(fmt.Sprintf("Type %s to erase this disk:", d.Path), "")
	if err != nil {
		return err
	}
	if v != d.Path {
		return errors.New("confirmation did not match the disk; nothing was written")
	}
	return nil
}

func (s *session) acquire() error {
	f, err := os.OpenFile(s.path(lockFile), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return fmt.Errorf("another dctl install is running (%s is locked): %w", lockFile, err)
	}
	s.lock = f
	return nil
}

func (s *session) vacant() error {
	if exists(s.path("/dev/mapper", mapper)) {
		return fmt.Errorf("refusing to install: /dev/mapper/%s is already open", mapper)
	}
	busy, err := s.busy()
	if err != nil {
		return err
	}
	if busy {
		return fmt.Errorf("refusing to install: something is mounted under %s", target)
	}
	return nil
}

func (s *session) busy() (bool, error) {
	data, err := os.ReadFile(s.path("/proc/self/mountinfo"))
	if err != nil {
		return false, err
	}
	for line := range strings.Lines(string(data)) {
		if f := strings.Fields(line); len(f) > 4 && (f[4] == target || strings.HasPrefix(f[4], target+"/")) {
			return true, nil
		}
	}
	return false, nil
}

func (s *session) release(ctx context.Context) error {
	var errs []error
	if s.mounted {
		busy, err := s.busy()
		errs = append(errs, err)
		if busy {
			errs = append(errs, s.sh.run(ctx, nil, "umount", "-R", target))
		}
	}
	if s.opened {
		errs = append(errs, s.sh.run(ctx, nil, "cryptsetup", "close", mapper))
	}
	errs = append(errs, s.sh.run(ctx, nil, "sync"))
	return errors.Join(errs...)
}

func (s *session) install(ctx context.Context, p plan, a iso.Answers, pkgs []string) (err error) {
	defer func() {
		if rerr := s.release(context.WithoutCancel(ctx)); rerr != nil {
			err = errors.Join(err, fmt.Errorf("release target: %w", rerr))
		}
	}()
	cmds := func(list []cmd) func(context.Context) error {
		return func(ctx context.Context) error {
			for _, c := range list {
				var stdin []byte
				if c.Key {
					stdin = []byte(a.LUKS)
				}
				if err := s.sh.run(ctx, stdin, c.Args...); err != nil {
					return err
				}
				switch a := c.Args; {
				case a[0] == "cryptsetup" && a[1] == "open":
					s.opened = true
				case a[0] == "mount" && strings.HasPrefix(a[len(a)-1], target):
					s.mounted = true
				}
			}
			return nil
		}
	}
	steps := []step{
		{"partition", cmds(p.partition())},
		{"format", cmds(p.format())},
		{"subvolumes", cmds(p.subvolumes())},
		{"mount", cmds(p.mount())},
		{"pacstrap", func(ctx context.Context) error { return s.pacstrap(ctx, pkgs) }},
		{"files", func(context.Context) error { return s.write(p) }},
		{"firstboot", cmds(p.firstboot())},
		{"accounts", func(ctx context.Context) error { return s.accounts(ctx, a) }},
		{"keyring", cmds([]cmd{{Args: chroot("pacman-key", "--init")}, {Args: chroot("pacman-key", "--populate", "archlinux")}})},
		{"dotfiles", func(ctx context.Context) error { return s.dotfiles(ctx) }},
		{"setup", func(ctx context.Context) error { return s.setup(ctx) }},
		{"snapshots", cmds(p.snapshots())},
		{"boot", cmds([]cmd{{Args: chroot("limine-install")}})},
		{"kernel", cmds([]cmd{{Args: chroot("limine-update")}})},
		{"validate", func(ctx context.Context) error { return s.validate(ctx, p) }},
	}
	if s.testMounted {
		steps = append(steps, step{"dctltest", func(ctx context.Context) error { return s.testSetup(ctx) }})
	}
	for _, st := range steps {
		s.u.Section(st.name, "")
		start := time.Now()
		err := st.do(ctx)
		s.times = append(s.times, iso.Phase{Name: st.name, Seconds: time.Since(start).Seconds()})
		if err != nil {
			return fmt.Errorf("%s: %w", st.name, err)
		}
	}
	return nil
}

const testUnit = "dctltest-setup.service"

func (s *session) testSetup(ctx context.Context) error {
	home := filepath.Join("/home", login)
	unit := fmt.Sprintf(`[Unit]
Description=DCTLTEST post-boot setup status
After=multi-user.target display-manager.service

[Service]
Type=oneshot
Environment=HOME=/root DOTFILES=%[1]s/dotfiles
ExecStart=-%[2]s --json setup --status %[3]s secureboot
ExecStart=-/usr/bin/runuser -u %[4]s -- %[2]s --json setup --status %[5]s
ExecStart=/bin/sh -c 'echo "%[6]s$$(systemctl is-active display-manager)"'
StandardOutput=tty
TTYPath=%[7]s
StandardError=journal

[Install]
WantedBy=graphical.target
`, home, filepath.Join(home, ".local", "bin", "dctl"), strings.Join(rootStages, " "), login, strings.Join(userStages, " "), iso.Greeter, console)
	if err := os.WriteFile(s.path(target, "/etc/systemd/system", testUnit), []byte(unit), 0o644); err != nil {
		return err
	}
	return s.sh.run(ctx, nil, chroot("systemctl", "enable", testUnit)...)
}

func chroot(args ...string) []string {
	return append([]string{"arch-chroot", "-r", target}, args...)
}

func as(args ...string) []string {
	return chroot(append([]string{"runuser", "-u", login, "--"}, args...)...)
}

func (s *session) mask() (func() error, error) {
	dir := s.path(hooks)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var made []string
	restore := func() error {
		var errs []error
		for _, p := range made {
			errs = append(errs, os.Remove(p))
		}
		made = nil
		return errors.Join(errs...)
	}
	for _, name := range masked {
		p := filepath.Join(dir, name)
		if err := os.Symlink("/dev/null", p); err != nil {
			return nil, errors.Join(fmt.Errorf("mask live pacman hook: %w", err), restore())
		}
		made = append(made, p)
	}
	return restore, nil
}

func (s *session) pacstrap(ctx context.Context, pkgs []string) (err error) {
	restore, err := s.mask()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, restore()) }()
	cache := filepath.Join(target, "var", "cache", "pacman", "pkg")
	if err := s.sh.run(ctx, nil, "mount", "--bind", iso.Payload, cache); err != nil {
		return err
	}
	err = s.sh.run(ctx, nil, append([]string{"pacstrap", "-G", "-M", target}, pkgs...)...)
	return errors.Join(err, s.sh.run(context.WithoutCancel(ctx), nil, "umount", cache))
}

func (s *session) write(p plan) error {
	root, err := os.OpenRoot(s.path(target))
	if err != nil {
		return err
	}
	defer root.Close()
	for _, f := range append(p.files(), file{"etc/pacman.d/mirrorlist", 0o644, mirror}) {
		if err := root.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
			return err
		}
		if err := root.WriteFile(f.Path, []byte(f.Data), f.Mode); err != nil {
			return err
		}
		if err := root.Chmod(f.Path, f.Mode); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) accounts(ctx context.Context, a iso.Answers) error {
	for _, c := range []struct {
		args  []string
		stdin string
	}{
		{args: chroot("locale-gen")},
		{args: chroot("useradd", "-m", "-G", "wheel", "-s", "/usr/bin/zsh", login)},
		{args: chroot("chpasswd"), stdin: login + ":" + a.Password + "\n"},
		{args: chroot("passwd", "-l", "root")},
	} {
		var stdin []byte
		if c.stdin != "" {
			stdin = []byte(c.stdin)
		}
		if err := s.sh.run(ctx, stdin, c.args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) dotfiles(ctx context.Context) error {
	if err := copyFile(s.path(s.bundle), s.path(target, staged), 0o644); err != nil {
		return err
	}
	defer os.Remove(s.path(target, staged))
	home := filepath.Join("/home", login)
	bin := s.path(target, home, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	for _, name := range binaries.Names {
		src := s.path(iso.Bin, name)
		if name == "dctl" {
			src = s.exe
		}
		if err := copyFile(src, filepath.Join(bin, name), 0o755); err != nil {
			return err
		}
	}
	repo := filepath.Join(home, "dotfiles")
	git := func(args ...string) []string { return as(append([]string{"git", "-C", repo}, args...)...) }
	for _, args := range [][]string{
		chroot("chown", "-R", login+":", filepath.Join(home, ".local")),
		as("git", "init", "--quiet", "--initial-branch=master", repo),
	} {
		if err := s.sh.run(ctx, nil, args...); err != nil {
			return err
		}
	}
	heads, err := s.sh.output(ctx, git("bundle", "list-heads", staged, "refs/heads/master")...)
	if err != nil {
		return err
	}
	tip, _, ok := strings.Cut(string(heads), " ")
	if !ok {
		return fmt.Errorf("%s has no master: %q", s.bundle, heads)
	}
	shallow := filepath.Join(repo, ".git", "shallow")
	if err := os.WriteFile(s.path(target, shallow), []byte(tip+"\n"), 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{
		chroot("chown", login+":", shallow),
		git("remote", "add", "origin", origin),
		git("fetch", "--quiet", staged, "master:refs/remotes/origin/master"),
		git("switch", "--quiet", "--force-create", "master", "--track", "origin/master"),
	} {
		if err := s.sh.run(ctx, nil, args...); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	return errors.Join(err, out.Close())
}

func dctl(args ...string) []string {
	home := filepath.Join("/home", login)
	return append([]string{"env", "DOTFILES=" + filepath.Join(home, "dotfiles"), filepath.Join(home, ".local", "bin", "dctl")}, args...)
}

func (s *session) setup(ctx context.Context) error {
	s.u.Info("setup stages: root %s; %s %s", strings.Join(rootStages, ", "), login, strings.Join(userStages, ", "))
	if err := s.sh.run(ctx, nil, chroot(dctl(slices.Concat([]string{"setup"}, rootStages)...)...)...); err != nil {
		return err
	}
	return s.sh.run(ctx, nil, as(dctl(slices.Concat([]string{"setup"}, userStages)...)...)...)
}

var errModified = errors.New("The target disk has already been modified; installation is incomplete.")

func (s *session) report(err error) {
	line := iso.Report{Event: "install", OK: err == nil, Phases: s.times}
	for _, t := range s.times {
		line.Total += t.Seconds
	}
	if err != nil {
		line.Error = err.Error()
	}
	data, _ := json.Marshal(line)
	fmt.Fprintf(os.Stdout, "%s\n", data)
	f, ferr := os.OpenFile(s.path(console), os.O_WRONLY|os.O_APPEND, 0)
	if ferr != nil {
		s.u.Warn("serial console: %v", ferr)
		return
	}
	fmt.Fprintf(f, "%s\n", data)
	f.Close()
}
