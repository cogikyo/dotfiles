package iso

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	bins "dotfiles/cmds/internal/dctl/binaries"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/packages"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/gobuild"
	"dotfiles/cmds/internal/ui"
)

type build struct {
	u       *ui.UI
	run     execx.Runner
	owner   *user.User
	nobody  *user.User
	as      []string
	repo    string
	work    string
	recipes string
	out     string
}

func Build(ctx context.Context, u *ui.UI, root paths.Root) error {
	name := os.Getenv("SUDO_USER")
	if os.Geteuid() != 0 || name == "" || name == "root" {
		return errors.New("run as `sudo dctl iso build` from your user account")
	}
	owner, err := user.Lookup(name)
	if err != nil {
		return err
	}
	nobody, err := user.Lookup("nobody")
	if err != nil {
		return err
	}
	b := build{
		u:      u,
		run:    execx.OSRunner{Group: true},
		owner:  owner,
		nobody: nobody,
		as:     []string{"runuser", "-u", owner.Username, "--", "env", "HOME=" + owner.HomeDir},
		repo:   root.Dotfiles,
		work:   filepath.Join(root.Dotfiles, "iso", "work"),
		out:    filepath.Join(root.Dotfiles, "iso", "out"),
	}
	if err := os.RemoveAll(b.work); err != nil {
		return err
	}
	if b.recipes, err = os.MkdirTemp("/var/tmp", "dctl-iso-"); err != nil {
		return err
	}
	err = os.Chmod(b.recipes, 0o755)
	if err == nil {
		err = b.build(ctx)
	}
	return errors.Join(err, os.RemoveAll(b.work), os.RemoveAll(b.recipes))
}

func (b build) build(ctx context.Context) error {
	rev, err := revision(ctx, b.run, b.repo, b.as)
	if err != nil {
		return err
	}
	b.u.KV("revision", rev)
	stage := filepath.Join(b.work, "stage")
	if err := mkdir(stage, b.owner); err != nil {
		return err
	}
	src := filepath.Join(stage, "src")
	bundle := filepath.Join(stage, "dotfiles.bundle")
	b.u.Section("bundle", "master into the ISO source")
	if err := b.user(ctx, "git", "-C", b.repo, "bundle", "create", bundle, "master"); err != nil {
		return err
	}
	if err := b.user(ctx, "git", "clone", "--quiet", bundle, src); err != nil {
		return err
	}
	head, err := b.run.Output(ctx, "", b.as[0], append(b.as[1:], "git", "-C", src, "rev-parse", "HEAD")...)
	if err != nil {
		return err
	}
	if head != rev {
		return fmt.Errorf("bundle clone HEAD %q is not %s", head, rev)
	}
	air := filepath.Join(src, "iso", "airootfs")
	payload := filepath.Join(air, Payload)
	if err := errors.Join(os.MkdirAll(payload, 0o755), mkdir(filepath.Join(air, Bin), b.owner)); err != nil {
		return err
	}
	if err := os.Rename(bundle, filepath.Join(air, Bundle)); err != nil {
		return err
	}

	b.u.Section("build", strings.Join(bins.Names, ", "))
	args := slices.Concat([]string{"env", "GOENV=off", "GOTOOLCHAIN=local"}, gobuild.Env,
		[]string{"go", "-C", filepath.Join(src, "cmds"), "build"}, gobuild.Flags)
	args = append(args, "-o", filepath.Join(air, Bin)+"/")
	for _, name := range bins.Names {
		args = append(args, "./cmd/"+name)
	}
	if err := b.user(ctx, args...); err != nil {
		return err
	}

	sizes, targets, err := b.payload(ctx, filepath.Join(src, "packages"), payload)
	if err != nil {
		return err
	}
	if err := writeTargets(filepath.Join(air, Targets), targets); err != nil {
		return err
	}
	var total int64
	for _, s := range sizes {
		total += s.Size
	}
	b.u.KV("payload", fmt.Sprintf("%d packages, %s", len(sizes), mib(total)))
	if err := oversize(total, sizes); err != nil {
		return err
	}

	b.u.Section("mkarchiso", "")
	staged := filepath.Join(b.work, "out")
	if err := b.run.Run(ctx, "", "mkarchiso", "-v", "-w", filepath.Join(b.work, "mkarchiso"), "-o", staged, filepath.Join(src, "iso")); err != nil {
		return err
	}
	built, err := filepath.Glob(filepath.Join(staged, "*.iso"))
	if err != nil || len(built) != 1 {
		return fmt.Errorf("mkarchiso produced %d ISOs in %s", len(built), staged)
	}
	if err := os.MkdirAll(b.out, 0o755); err != nil {
		return err
	}
	iso := filepath.Join(b.out, "dotfiles-"+rev[:12]+".iso")
	if err := os.Rename(built[0], iso); err != nil {
		return err
	}
	uid, gid, err := ids(b.owner)
	if err != nil {
		return err
	}
	if err := errors.Join(os.Chown(b.out, uid, gid), os.Chown(iso, uid, gid)); err != nil {
		return err
	}
	st, err := os.Stat(iso)
	if err != nil {
		return err
	}
	b.u.KV("iso", iso)
	b.u.KV("size", fmt.Sprintf("%s (%d bytes, limit %d)", mib(st.Size()), st.Size(), MaxSize))
	if err := oversize(st.Size(), sizes); err != nil {
		return fmt.Errorf("%s is kept for local use but cannot be released: %w", iso, err)
	}
	b.u.Close(ui.OK, "ISO built; sign and publish with `dctl iso release %s`", iso)
	return nil
}

func (b build) payload(ctx context.Context, lists, payload string) ([]sized, []string, error) {
	l, err := packages.Load(lists)
	if err != nil {
		return nil, nil, err
	}

	chroot := filepath.Join(b.recipes, "chroot")
	built := filepath.Join(b.recipes, "built")
	srcdest := filepath.Join(b.recipes, "srcdest")
	if err := errors.Join(os.MkdirAll(chroot, 0o755), os.MkdirAll(built, 0o755), mkdir(srcdest, b.nobody)); err != nil {
		return nil, nil, err
	}
	b.u.Section("chroot", "clean build chroot for local recipes")
	if err := b.run.Run(ctx, "", "mkarchroot",
		"-C", "/usr/share/devtools/pacman.conf.d/extra.conf",
		"-M", "/usr/share/devtools/makepkg.conf.d/x86_64.conf",
		filepath.Join(chroot, "root"), "base-devel"); err != nil {
		return nil, nil, err
	}
	for _, name := range l.AUR {
		dir := filepath.Join(b.recipes, name)
		if err := b.run.Run(ctx, "", "git", "clone", "--quiet", "--depth", "1", "https://aur.archlinux.org/"+name+".git", dir); err != nil {
			return nil, nil, err
		}
		if _, err := os.Stat(filepath.Join(dir, "PKGBUILD")); err != nil {
			return nil, nil, fmt.Errorf("AUR package %s does not exist (no PKGBUILD)", name)
		}
	}
	for _, name := range l.Local {
		if err := b.run.Run(ctx, "", "cp", "-rT", filepath.Join(lists, name), filepath.Join(b.recipes, name)); err != nil {
			return nil, nil, err
		}
	}
	recipes := slices.Concat(l.AUR, l.Local)
	gnupg := filepath.Join(b.recipes, "gnupg")
	if err := errors.Join(os.Mkdir(gnupg, 0o700), mkdir(gnupg, b.nobody)); err != nil {
		return nil, nil, err
	}
	gpg := []string{"-u", b.nobody.Username, "--", "gpg", "--batch", "--homedir", gnupg}
	for _, name := range recipes {
		dir := filepath.Join(b.recipes, name)
		keys, err := filepath.Glob(filepath.Join(dir, "keys", "pgp", "*.asc"))
		if err != nil {
			return nil, nil, err
		}
		if len(keys) > 0 {
			if err := b.run.Run(ctx, "", "runuser", slices.Concat(gpg, []string{"--import"}, keys)...); err != nil {
				return nil, nil, err
			}
		}
		fprs, err := validpgpkeys(dir)
		if err != nil {
			return nil, nil, err
		}
		for _, fpr := range fprs {
			if _, err := b.run.Output(ctx, "", "runuser", slices.Concat(gpg, []string{"--with-colons", "--list-keys", fpr})...); err != nil {
				return nil, nil, fmt.Errorf("%s: validpgpkeys %s is not in the recipe's keys/pgp: %w", name, fpr, err)
			}
		}
	}
	for _, name := range recipes {
		b.u.Info("makepkg %s in the chroot as nobody", name)
		dir := filepath.Join(b.recipes, name)
		err := b.run.Run(ctx, "", "chown", "-R", b.nobody.Username+":", dir)
		if err == nil {
			err = b.run.Run(ctx, dir, "env", "-i",
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/bin", "HOME=/root", "USER=root", "LANG=C.UTF-8", "TERM="+os.Getenv("TERM"), "PKGDEST="+built, "SRCDEST="+srcdest, "GNUPGHOME="+gnupg,
				"makechrootpkg", "-c", "-U", b.nobody.Username, "-r", chroot)
		}
		if err := errors.Join(err, b.run.Run(ctx, "", "chown", "-R", "root:", dir)); err != nil {
			return nil, nil, fmt.Errorf("build %s: %w", name, err)
		}
	}
	files, err := filepath.Glob(filepath.Join(built, "*.pkg.tar.zst"))
	if err != nil {
		return nil, nil, err
	}
	if err := b.run.Run(ctx, built, "repo-add", append([]string{"--quiet", Repo + ".db.tar.zst"}, files...)...); err != nil {
		return nil, nil, err
	}

	b.u.Section("payload", "offline package closure")
	resolve := filepath.Join(b.recipes, "resolve")
	if err := os.MkdirAll(filepath.Join(resolve, "db"), 0o755); err != nil {
		return nil, nil, err
	}
	conf := filepath.Join(resolve, "pacman.conf")
	if err := os.WriteFile(conf, fmt.Appendf(nil, `[options]
Architecture = auto
SigLevel = Required DatabaseOptional
ParallelDownloads = 5

[%s]
SigLevel = Never
Server = file://%s

[core]
Include = /etc/pacman.d/mirrorlist

[extra]
Include = /etc/pacman.d/mirrorlist
`, Repo, built), 0o644); err != nil {
		return nil, nil, err
	}
	pacman := []string{"--config", conf, "--dbpath", filepath.Join(resolve, "db"), "--logfile", "/dev/null"}
	if err := b.run.Run(ctx, "", "pacman", append(slices.Clone(pacman), "-Sy")...); err != nil {
		return nil, nil, err
	}
	targets := l.Payload()
	out, err := b.run.Output(ctx, "", "pacman", slices.Concat(pacman, []string{"-Sp", "--print-format", "%n %f"}, targets)...)
	if err != nil {
		return nil, nil, err
	}
	closure, err := parseResolved(out)
	if err != nil {
		return nil, nil, err
	}
	if missing := unresolved(targets, closure); len(missing) > 0 {
		return nil, nil, fmt.Errorf("not package names in the closure (group or provider?): %s", strings.Join(missing, " "))
	}
	b.u.Info("fetching %d packages", len(closure))
	if err := b.run.Run(ctx, "", "pacman", slices.Concat(pacman, []string{"--cachedir", payload, "-Sw", "--noconfirm"}, targets)...); err != nil {
		return nil, nil, err
	}

	want := map[string]bool{}
	for _, p := range closure {
		want[p.File] = true
	}
	entries, err := os.ReadDir(payload)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		if !want[e.Name()] {
			if err := os.RemoveAll(filepath.Join(payload, e.Name())); err != nil {
				return nil, nil, err
			}
		}
	}
	var sizes []sized
	var names []string
	for _, p := range closure {
		st, err := os.Stat(filepath.Join(payload, p.File))
		if err != nil {
			return nil, nil, fmt.Errorf("payload is missing resolved package %s: %w", p.Name, err)
		}
		sizes = append(sizes, sized{p.Name, st.Size()})
		names = append(names, p.File)
	}
	if err := b.run.Run(ctx, payload, "repo-add", append([]string{"--quiet", Repo + ".db.tar.zst"}, names...)...); err != nil {
		return nil, nil, err
	}
	return sizes, targets, writeSums(ctx, payload)
}

func (b build) user(ctx context.Context, args ...string) error {
	argv := append(slices.Clone(b.as), args...)
	return b.run.Run(ctx, "", argv[0], argv[1:]...)
}

func ids(u *user.User) (int, int, error) {
	uid, uerr := strconv.Atoi(u.Uid)
	gid, gerr := strconv.Atoi(u.Gid)
	return uid, gid, errors.Join(uerr, gerr)
}

func mkdir(dir string, owner *user.User) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	uid, gid, err := ids(owner)
	if err != nil {
		return err
	}
	return os.Chown(dir, uid, gid)
}

func validpgpkeys(recipe string) ([]string, error) {
	info, err := os.ReadFile(filepath.Join(recipe, ".SRCINFO"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var fprs []string
	for line := range strings.Lines(string(info)) {
		if fpr, ok := strings.CutPrefix(strings.TrimSpace(line), "validpgpkeys = "); ok {
			fprs = append(fprs, fpr)
		}
	}
	return fprs, nil
}
