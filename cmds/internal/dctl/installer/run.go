package installer

import (
	"context"
	"debug/pe"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"dotfiles/cmds/internal/dctl/binaries"
	"dotfiles/cmds/internal/dctl/iso"
	"dotfiles/cmds/internal/dctl/secureboot"
	"dotfiles/cmds/internal/dctl/ui"
)

const (
	hooks  = "/etc/pacman.d/hooks"
	staged = "/var/tmp/dotfiles.bundle"
	origin = "git@github.com:cogikyo/dotfiles.git"
	mirror = "Server = https://geo.mirror.pkgbuild.com/$repo/os/$arch\n"
	serial = "/dev/ttyS0"
)

var (
	masked     = []string{"60-mkinitcpio-remove.hook", "90-mkinitcpio-install.hook", "60-limine-mkinitcpio-remove-pre.hook", "80-limine-efi-deploy.hook", "90-limine-mkinitcpio-remove-post.hook"}
	rootGroups = []string{"system", "packages"}
	homeGroups = []string{"home", "binaries"}
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
	confirm     func(Plan) error
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
	if err := s.acquire(ctx); err != nil {
		return err
	}
	defer s.lock.Close()
	defer func() { err = errors.Join(err, s.unmountTest(context.WithoutCancel(ctx))) }()
	test, err := s.dctltest(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			s.report(test != nil, err)
		}
	}()

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

	disk, err := pick(s.u, prep.disks, test)
	if err != nil {
		return err
	}
	p, err := New(disk, a.User, a.Host, a.Zone)
	if err != nil {
		return err
	}
	p.SecureBoot = prep.firmware.Setup
	if !p.SecureBoot {
		s.u.Warn("firmware is not in Secure Boot Setup Mode; installing without Secure Boot (later: dctl doctor --fix secureboot)")
	}
	if test == nil {
		if err := s.confirm(p); err != nil {
			return err
		}
	}
	boot, out, err := s.lsblk(ctx)
	if err != nil {
		return err
	}
	if err := p.Disk.Recheck(out, boot); err != nil {
		return err
	}
	if err := s.vacant(); err != nil {
		return err
	}

	if err := s.install(ctx, p, a, prep.targets); err != nil {
		return err
	}
	s.report(test != nil, nil)
	if test == nil {
		s.u.Info("after first login as %s, run `dctl doctor`, then `dctl doctor --fix`", a.User)
		if ok, err := s.u.Confirm("Reboot now?"); err != nil || !ok {
			s.u.Info("installed; reboot when ready")
			return nil
		}
	}
	if err := s.unmountTest(context.WithoutCancel(ctx)); err != nil {
		return err
	}
	return s.sh.run(context.WithoutCancel(ctx), nil, "systemctl", "reboot")
}

func pick(u *ui.UI, survey Survey, test *iso.Answers) (Disk, error) {
	if test != nil {
		i := slices.IndexFunc(survey.Candidates, func(d Disk) bool { return d.Serial == test.Serial })
		if i < 0 {
			return Disk{}, fmt.Errorf("DCTLTEST: no installable disk has serial %q", test.Serial)
		}
		return survey.Candidates[i], nil
	}
	for _, r := range survey.Refused {
		u.Dim("skipping %s: %s", r.Disk.Path, r.Reason)
	}
	d, err := survey.Pick("")
	if !errors.Is(err, ErrAmbiguous) {
		return d, err
	}
	labels := make([]string, len(survey.Candidates))
	for i, c := range survey.Candidates {
		labels[i] = fmt.Sprintf("%s  %s  %.0f GB  %s", c.Path, c.Model, float64(c.Size)/1e9, c.Serial)
	}
	i, err := u.Select("Install to which disk?", labels, 0)
	if err != nil {
		return Disk{}, err
	}
	return survey.Candidates[i], nil
}

func (s *session) typed(p Plan) error {
	d := p.Disk
	s.u.Header("Erase %s", d.Path)
	s.u.KV("model", d.Model)
	s.u.KV("size", fmt.Sprintf("%.0f GB", float64(d.Size)/1e9))
	s.u.KV("serial", d.Serial)
	if d.WWN != "" {
		s.u.KV("wwn", d.WWN)
	}
	v, err := s.u.Text(fmt.Sprintf("Type the hostname %q to erase %s:", p.Host, d.Path), "")
	if err != nil {
		return err
	}
	if v != p.Host {
		return errors.New("confirmation did not match the hostname; nothing was written")
	}
	return nil
}

func (s *session) acquire(ctx context.Context) error {
	s.lock = s.adopt(ctx)
	if s.lock == nil {
		f, err := os.OpenFile(s.path(lockFile), os.O_RDWR|os.O_CREATE|unix.O_CLOEXEC, 0o600)
		if err != nil {
			return err
		}
		s.lock = f
	}
	if err := syscall.Flock(int(s.lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		s.lock.Close()
		return fmt.Errorf("another dctl install is running (%s is locked): %w", lockFile, err)
	}
	return nil
}

func (s *session) adopt(ctx context.Context) *os.File {
	fd, err := strconv.Atoi(os.Getenv(testEnv))
	if err != nil || fd < 3 {
		return nil
	}
	if _, err := s.sh.output(ctx, "systemd-detect-virt", "--vm"); err != nil {
		return nil
	}
	var held, want unix.Stat_t
	if unix.Fstat(fd, &held) != nil || unix.Stat(s.path(lockFile), &want) != nil || held.Dev != want.Dev || held.Ino != want.Ino {
		return nil
	}
	unix.CloseOnExec(fd)
	return os.NewFile(uintptr(fd), s.path(lockFile))
}

func (s *session) vacant() error {
	if exists(s.path("/dev/mapper", Mapper)) {
		return fmt.Errorf("refusing to install: /dev/mapper/%s is already open", Mapper)
	}
	busy, err := s.busy()
	if err != nil {
		return err
	}
	if busy {
		return fmt.Errorf("refusing to install: something is mounted under %s", Target)
	}
	return nil
}

func (s *session) busy() (bool, error) {
	data, err := os.ReadFile(s.path("/proc/self/mountinfo"))
	if err != nil {
		return false, err
	}
	for line := range strings.Lines(string(data)) {
		if f := strings.Fields(line); len(f) > 4 && (f[4] == Target || strings.HasPrefix(f[4], Target+"/")) {
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
			errs = append(errs, s.sh.run(ctx, nil, "umount", "-R", Target))
		}
	}
	if s.opened {
		errs = append(errs, s.sh.run(ctx, nil, "cryptsetup", "close", Mapper))
	}
	errs = append(errs, s.sh.run(ctx, nil, "sync"))
	return errors.Join(errs...)
}

func (s *session) install(ctx context.Context, p Plan, a iso.Answers, pkgs []string) (err error) {
	defer func() {
		if rerr := s.release(context.WithoutCancel(ctx)); rerr != nil {
			err = errors.Join(err, fmt.Errorf("release target: %w", rerr))
		}
	}()
	cmds := func(list []Cmd) func(context.Context) error {
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
				case a[0] == "mount" && strings.HasPrefix(a[len(a)-1], Target):
					s.mounted = true
				}
			}
			return nil
		}
	}
	steps := []step{
		{"partition", cmds(p.Partition())},
		{"format", cmds(p.Format())},
		{"subvolumes", cmds(p.Subvolumes())},
		{"mount", cmds(p.Mount())},
		{"pacstrap", func(ctx context.Context) error { return s.pacstrap(ctx, pkgs) }},
		{"files", func(context.Context) error { return s.write(p) }},
		{"firstboot", cmds(p.Firstboot())},
		{"accounts", func(ctx context.Context) error { return s.accounts(ctx, a) }},
		{"keyring", cmds([]Cmd{{Args: chroot("pacman-key", "--init")}, {Args: chroot("pacman-key", "--populate", "archlinux")}})},
		{"dotfiles", func(ctx context.Context) error { return s.dotfiles(ctx, a.User) }},
		{"doctor", func(ctx context.Context) error { return s.doctor(ctx, a.User) }},
		{"snapshots", cmds(p.Snapshots())},
		{"boot", func(ctx context.Context) error { return s.boot(ctx, p) }},
		{"validate", func(ctx context.Context) error { return s.validate(ctx, p) }},
	}
	if s.testMounted {
		steps = append(steps, step{"dctltest", func(ctx context.Context) error { return s.testDoctor(ctx, a.User) }})
	}
	if p.SecureBoot {
		steps = append(steps, step{"enroll", cmds([]Cmd{{Args: chroot("sbctl", "enroll-keys", "-m")}})})
	}
	for _, st := range steps {
		s.u.Step("%s", st.name)
		start := time.Now()
		err := st.do(ctx)
		s.times = append(s.times, iso.Phase{Name: st.name, Seconds: time.Since(start).Seconds()})
		if err != nil {
			return fmt.Errorf("%s: %w", st.name, err)
		}
	}
	return nil
}

const testUnit = "dctltest-doctor.service"

func (s *session) testDoctor(ctx context.Context, user string) error {
	unit := fmt.Sprintf(`[Unit]
Description=DCTLTEST post-boot doctor
After=multi-user.target

[Service]
Type=oneshot
User=%s
ExecStart=/home/%s/.local/bin/dctl --json doctor --offline
StandardOutput=tty
TTYPath=%s
StandardError=journal

[Install]
WantedBy=graphical.target
`, user, user, serial)
	if err := os.WriteFile(s.path(Target, "/etc/systemd/system", testUnit), []byte(unit), 0o644); err != nil {
		return err
	}
	return s.sh.run(ctx, nil, chroot("systemctl", "enable", testUnit)...)
}

func chroot(args ...string) []string {
	return append([]string{"arch-chroot", Target}, args...)
}

func as(user string, args ...string) []string {
	return chroot(append([]string{"runuser", "-u", user, "--"}, args...)...)
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
	cache := filepath.Join(Target, "var", "cache", "pacman", "pkg")
	if err := s.sh.run(ctx, nil, "mount", "--bind", iso.Payload, cache); err != nil {
		return err
	}
	err = s.sh.run(ctx, nil, append([]string{"pacstrap", "-G", "-M", Target}, pkgs...)...)
	return errors.Join(err, s.sh.run(context.WithoutCancel(ctx), nil, "umount", cache))
}

func (s *session) write(p Plan) error {
	root, err := os.OpenRoot(s.path(Target))
	if err != nil {
		return err
	}
	defer root.Close()
	for _, f := range append(p.Files(), File{"etc/pacman.d/mirrorlist", 0o644, mirror}) {
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
		{args: chroot("useradd", "-m", "-G", "wheel", "-s", "/usr/bin/zsh", a.User)},
		{args: chroot("chpasswd"), stdin: a.User + ":" + a.Password + "\n"},
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

func (s *session) dotfiles(ctx context.Context, user string) error {
	if err := copyFile(s.path(s.bundle), s.path(Target, staged), 0o644); err != nil {
		return err
	}
	defer os.Remove(s.path(Target, staged))
	home := filepath.Join("/home", user)
	bin := s.path(Target, home, ".local", "bin")
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
	for _, args := range [][]string{
		chroot("chown", "-R", user+":", filepath.Join(home, ".local")),
		as(user, "git", "clone", "--quiet", staged, repo),
		as(user, "git", "-C", repo, "remote", "set-url", "origin", origin),
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

func (s *session) doctor(ctx context.Context, user string) error {
	home := filepath.Join("/home", user)
	repo := filepath.Join(home, "dotfiles")
	dctl := filepath.Join(home, ".local", "bin", "dctl")
	s.u.Info("doctor groups: root %s; %s %s", strings.Join(rootGroups, ", "), user, strings.Join(homeGroups, ", "))
	if err := s.sh.run(ctx, nil, chroot(append([]string{"env", "DOTFILES=" + repo, dctl, "doctor", "--fix", "--offline"}, rootGroups...)...)...); err != nil {
		return err
	}
	return s.sh.run(ctx, nil, as(user, append([]string{"env", "DOTFILES=" + repo, "HOME=" + home, dctl, "doctor", "--fix", "--offline"}, homeGroups...)...)...)
}

func (s *session) boot(ctx context.Context, p Plan) error {
	var list [][]string
	if p.SecureBoot {
		list = append(list, chroot("sbctl", "create-keys"))
	}
	list = append(list, chroot("limine-update"), chroot("limine-install"))
	for _, args := range list {
		if err := s.sh.run(ctx, nil, args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) validate(ctx context.Context, p Plan) error {
	dir := s.path(Target, esp)
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
	want := "rd.luks.name=" + p.LUKSID + "=" + Mapper
	var errs []error
	for _, uki := range ukis {
		name := filepath.Base(uki)
		if !strings.Contains(string(conf), name) {
			errs = append(errs, fmt.Errorf("limine.conf has no entry for %s", name))
		}
		cmdline, embedded, err := embeddedCmdline(uki)
		switch {
		case err != nil:
			errs = append(errs, err)
		case embedded && p.SecureBoot:
			errs = append(errs, fmt.Errorf("%s embeds a cmdline under Secure Boot; snapshot entries could not boot", name))
		case embedded && !strings.Contains(cmdline, want):
			errs = append(errs, fmt.Errorf("%s cmdline %q lacks %s", name, cmdline, want))
		case !embedded && !strings.Contains(string(conf), want):
			errs = append(errs, fmt.Errorf("%s has no embedded cmdline and limine.conf lacks %s", name, want))
		}
	}
	if p.SecureBoot {
		errs = append(errs, s.signed(ctx, dir))
	}
	out, err := s.sh.output(ctx, "blkid", "-s", "UUID", "-o", "value", p.Disk.Part(2))
	switch got := strings.TrimSpace(string(out)); {
	case err != nil:
		errs = append(errs, err)
	case got != p.LUKSID:
		errs = append(errs, fmt.Errorf("LUKS UUID on %s is %s, cmdline names %s", p.Disk.Part(2), got, p.LUKSID))
	}
	return errors.Join(errs...)
}

func (s *session) signed(ctx context.Context, dir string) error {
	if exists(filepath.Join(dir, secureboot.Fallback)) {
		return fmt.Errorf("fallback %s exists under Secure Boot; it is neither signed nor config-enrolled", secureboot.Fallback)
	}
	files, err := secureboot.Files(dir)
	if err != nil {
		return err
	}
	for i, f := range files {
		files[i] = strings.TrimPrefix(f, s.path(Target))
	}
	out, err := s.sh.output(ctx, chroot(secureboot.Verify(files)...)...)
	if err != nil {
		return err
	}
	bad, err := secureboot.Unsigned(out, files)
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		return fmt.Errorf("not signed by the sbctl db key: %s", strings.Join(bad, ", "))
	}
	return nil
}

func embeddedCmdline(path string) (string, bool, error) {
	f, err := pe.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	sec := f.Section(".cmdline")
	if sec == nil {
		return "", false, nil
	}
	data, err := sec.Data()
	if err != nil {
		return "", false, err
	}
	return strings.TrimRight(string(data), "\x00\n "), true, nil
}

func (s *session) report(test bool, err error) {
	var total float64
	for _, t := range s.times {
		total += t.Seconds
	}
	if len(s.times) > 0 {
		s.u.Header("Timings")
		for _, t := range s.times {
			s.u.KV(t.Name, fmt.Sprintf("%.1fs", t.Seconds))
		}
		s.u.KV("total", fmt.Sprintf("%.1fs", total))
	}
	if !test {
		return
	}
	line := iso.Report{Event: "install", OK: err == nil, Total: total, Phases: s.times}
	if err != nil {
		line.Error = err.Error()
	}
	data, _ := json.Marshal(line)
	fmt.Fprintf(os.Stdout, "%s\n", data)
	f, ferr := os.OpenFile(s.path(serial), os.O_WRONLY|os.O_APPEND, 0)
	if ferr != nil {
		s.u.Warn("serial console: %v", ferr)
		return
	}
	fmt.Fprintf(f, "%s\n", data)
	f.Close()
}
