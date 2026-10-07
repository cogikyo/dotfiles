package iso

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	bins "dotfiles/cmds/internal/dctl/binaries"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/packages"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/ui"
)

const cacheDir = "/var/cache/dctl-iso"

type lap struct {
	name, note string
	took       time.Duration
}

type build struct {
	u       *ui.UI
	run     execx.Runner
	owner   *user.User
	as      []string
	repo    string
	work    string
	cache   string
	tmp     string
	out     string
	resolve string

	laps    []lap
	recipes []lap
	current string
	start   time.Time
}

func Build(ctx context.Context, u *ui.UI, root paths.Root, fresh bool) error {
	name := os.Getenv("SUDO_USER")
	if os.Geteuid() != 0 || name == "" || name == "root" {
		return errors.New("run `dctl iso build` from your user account")
	}
	owner, err := user.Lookup(name)
	if err != nil {
		return err
	}
	unlock, err := lock(cacheDir)
	if err != nil {
		return err
	}
	defer unlock()
	if fresh {
		u.Info("discarding the build cache %s", cacheDir)
		if err := discard(cacheDir); err != nil {
			return err
		}
	}
	b := &build{
		u:       u,
		run:     execx.OSRunner{Group: true},
		owner:   owner,
		as:      []string{"runuser", "-u", owner.Username, "--", "env", "HOME=" + owner.HomeDir},
		repo:    root.Dotfiles,
		work:    filepath.Join(root.Dotfiles, "iso", "work"),
		cache:   cacheDir,
		tmp:     filepath.Join(cacheDir, "tmp"),
		resolve: filepath.Join(cacheDir, "tmp", "resolve"),
		out:     filepath.Join(root.Dotfiles, "iso", "out"),
	}
	if err := errors.Join(os.RemoveAll(b.work), os.RemoveAll(b.tmp)); err != nil {
		return err
	}
	if err := os.MkdirAll(b.tmp, 0o755); err != nil {
		return err
	}
	iso, err := b.build(ctx)
	b.summary()
	if err := errors.Join(err, os.RemoveAll(b.work), os.RemoveAll(b.tmp)); err != nil {
		return err
	}
	u.Close(ui.OK, "ISO built; sign and publish with `dctl iso release %s`", iso)
	return nil
}

// lock refuses concurrent builds until the returned release function runs.
// Cache resets must preserve the lock file so every build locks the same inode.
func lock(cache string) (func(), error) {
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return nil, err
	}
	file := filepath.Join(cache, "lock")
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("another `dctl iso build` holds %s; wait for it to finish", file)
		}
		return nil, fmt.Errorf("lock %s: %w", file, err)
	}
	return func() { f.Close() }, nil
}

func discard(cache string) error {
	entries, err := os.ReadDir(cache)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == "lock" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(cache, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (b *build) section(name, note string) {
	b.stop()
	b.u.Section(name, note)
	b.current, b.start = name, time.Now()
}

func (b *build) stop() {
	if b.current != "" {
		b.laps = append(b.laps, lap{name: b.current, took: time.Since(b.start)})
		b.current = ""
	}
}

func (b *build) summary() {
	b.stop()
	if len(b.laps) == 0 {
		return
	}
	b.u.Section("timings", "")
	var total time.Duration
	for _, l := range b.laps {
		total += l.took
		b.u.KV(l.name, seconds(l.took))
		if l.name != "recipes" {
			continue
		}
		for _, r := range b.recipes {
			b.u.KV("  "+r.name, r.note+" "+seconds(r.took))
		}
	}
	b.u.KV("total", seconds(total))
}

func seconds(d time.Duration) string { return fmt.Sprintf("%.1fs", d.Seconds()) }

func (b *build) build(ctx context.Context) (string, error) {
	rev, err := revision(ctx, b.run, b.repo, b.as)
	if err != nil {
		return "", err
	}
	b.u.KV("revision", rev)
	stage := filepath.Join(b.work, "stage")
	if err := mkdir(stage, b.owner); err != nil {
		return "", err
	}
	src := filepath.Join(stage, "src")
	bundle := filepath.Join(stage, "dotfiles.bundle")
	b.section("bundle", "master tip into the ISO source")
	if err := b.user(ctx, "git", "clone", "--quiet", "--depth=1", "--branch=master", "file://"+b.repo, src); err != nil {
		return "", err
	}
	head, err := b.output(ctx, "git", "-C", src, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if head != rev {
		return "", fmt.Errorf("clone HEAD %q is not %s", head, rev)
	}
	if err := b.user(ctx, "git", "-C", src, "bundle", "create", bundle, "master"); err != nil {
		return "", err
	}
	air := filepath.Join(src, "iso", "airootfs")
	payload := filepath.Join(air, Payload)
	if err := errors.Join(os.MkdirAll(payload, 0o755), mkdir(filepath.Join(air, Bin), b.owner)); err != nil {
		return "", err
	}
	if err := os.Rename(bundle, filepath.Join(air, Bundle)); err != nil {
		return "", err
	}

	b.section("go build", strings.Join(bins.Names, ", "))
	args := append(gocmd(filepath.Join(src, "cmds")), "-o", filepath.Join(air, Bin)+"/")
	for _, name := range bins.Names {
		args = append(args, "./cmd/"+name)
	}
	if err := b.user(ctx, args...); err != nil {
		return "", err
	}

	l, err := packages.Load(filepath.Join(src, "packages"))
	if err != nil {
		return "", err
	}
	b.section("chroot", "cached build chroot for local recipes")
	if err := b.chroot(ctx); err != nil {
		return "", err
	}
	b.section("recipes", "AUR and local packages, reused by content key")
	built, err := b.compile(ctx, src, l)
	if err != nil {
		return "", err
	}
	b.section("payload", "offline package closure")
	sizes, targets, err := b.payload(ctx, l, built, payload)
	if err != nil {
		return "", err
	}
	if err := writeTargets(filepath.Join(air, Targets), targets); err != nil {
		return "", err
	}
	var total int64
	for _, s := range sizes {
		total += s.Size
	}
	b.u.KV("payload", fmt.Sprintf("%d packages, %s", len(sizes), mib(total)))
	if err := oversize(total, sizes); err != nil {
		return "", err
	}

	b.section("mkarchiso", "")
	staged := filepath.Join(b.work, "out")
	if err := b.run.Run(ctx, "", "mkarchiso", "-v", "-w", filepath.Join(b.work, "mkarchiso"), "-o", staged, filepath.Join(src, "iso")); err != nil {
		return "", err
	}
	b.stop()
	found, err := filepath.Glob(filepath.Join(staged, "*.iso"))
	if err != nil || len(found) != 1 {
		return "", fmt.Errorf("mkarchiso produced %d ISOs in %s", len(found), staged)
	}
	if err := os.MkdirAll(b.out, 0o755); err != nil {
		return "", err
	}
	iso := path(b.repo, rev)
	if err := os.Rename(found[0], iso); err != nil {
		return "", err
	}
	uid, gid, err := ids(b.owner)
	if err != nil {
		return "", err
	}
	if err := errors.Join(os.Chown(b.out, uid, gid), os.Chown(iso, uid, gid)); err != nil {
		return "", err
	}
	st, err := os.Stat(iso)
	if err != nil {
		return "", err
	}
	b.u.KV("iso", iso)
	b.u.KV("size", fmt.Sprintf("%s (%d bytes, limit %d)", mib(st.Size()), st.Size(), MaxSize))
	if err := oversize(st.Size(), sizes); err != nil {
		return "", fmt.Errorf("%s is kept for local use but cannot be released: %w", iso, err)
	}
	return iso, nil
}

func (b *build) chroot(ctx context.Context) error {
	dir := filepath.Join(b.cache, "chroot")
	ready := filepath.Join(dir, "ready")
	confs := []string{"-C", "/usr/share/devtools/pacman.conf.d/extra.conf", "-M", "/usr/share/devtools/makepkg.conf.d/x86_64.conf"}
	_, err := os.Stat(ready)
	if err == nil {
		b.u.Info("refreshing %s", dir)
		if err := b.run.Run(ctx, "", "arch-nspawn", slices.Concat(confs, []string{filepath.Join(dir, "root"), "pacman", "-Syu", "--noconfirm"})...); err != nil {
			return fmt.Errorf("refresh the build chroot (rerun with --fresh to recreate it): %w", err)
		}
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	b.u.Info("creating %s", dir)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := b.run.Run(ctx, "", "mkarchroot", slices.Concat(confs, []string{filepath.Join(dir, "root"), "base-devel"})...); err != nil {
		return err
	}
	return os.WriteFile(ready, nil, 0o644)
}

func (b *build) compile(ctx context.Context, src string, l packages.Lists) (string, error) {
	lists := filepath.Join(src, "packages")
	recipes := filepath.Join(b.tmp, "recipes")
	built := filepath.Join(b.tmp, "built")
	srcdest := filepath.Join(b.cache, "srcdest")
	if err := errors.Join(os.MkdirAll(recipes, 0o755), os.MkdirAll(built, 0o755), mkdir(srcdest, b.owner)); err != nil {
		return "", err
	}
	if err := b.confs(built); err != nil {
		return "", err
	}
	sources := map[string]string{}
	for _, name := range l.AUR {
		dir := filepath.Join(recipes, name)
		if err := b.run.Run(ctx, "", "git", "clone", "--quiet", "--depth", "1", "https://aur.archlinux.org/"+name+".git", dir); err != nil {
			return "", err
		}
		if _, err := os.Stat(filepath.Join(dir, "PKGBUILD")); err != nil {
			return "", fmt.Errorf("AUR package %s does not exist (no PKGBUILD)", name)
		}
		key, err := b.run.Output(ctx, "", "git", "-C", dir, "rev-parse", "HEAD")
		if err != nil {
			return "", err
		}
		sources[name] = key
	}
	for _, name := range l.Local {
		if err := b.run.Run(ctx, "", "cp", "-rT", filepath.Join(lists, name), filepath.Join(recipes, name)); err != nil {
			return "", err
		}
		key, err := b.output(ctx, "git", "-C", src, "rev-parse", "HEAD:packages/"+name)
		if err != nil {
			return "", err
		}
		sources[name] = key
	}
	names := slices.Concat(l.AUR, l.Local)
	gnupg := filepath.Join(b.tmp, "gnupg")
	if err := errors.Join(os.Mkdir(gnupg, 0o700), mkdir(gnupg, b.owner)); err != nil {
		return "", err
	}
	gpg := []string{"-u", b.owner.Username, "--", "gpg", "--batch", "--homedir", gnupg}
	for _, name := range names {
		dir := filepath.Join(recipes, name)
		pgp, err := filepath.Glob(filepath.Join(dir, "keys", "pgp", "*.asc"))
		if err != nil {
			return "", err
		}
		if len(pgp) > 0 {
			if err := b.run.Run(ctx, "", "runuser", slices.Concat(gpg, []string{"--import"}, pgp)...); err != nil {
				return "", err
			}
		}
		fprs, err := validpgpkeys(dir)
		if err != nil {
			return "", err
		}
		for _, fpr := range fprs {
			if _, err := b.run.Output(ctx, "", "runuser", slices.Concat(gpg, []string{"--with-colons", "--list-keys", fpr})...); err != nil {
				return "", fmt.Errorf("%s: validpgpkeys %s is not in the recipe's keys/pgp: %w", name, fpr, err)
			}
		}
	}

	if err := b.seed(); err != nil {
		return "", err
	}
	keys, infos, err := b.keys(ctx, recipes, names, sources)
	if err != nil {
		return "", err
	}
	store := filepath.Join(b.cache, "recipes")
	for _, name := range names {
		start := time.Now()
		entry := filepath.Join(store, name, keys[name])
		files, err := cached(ctx, entry)
		note := "hit"
		switch {
		case err == nil:
			b.u.OK("%s %s from the cache", name, short(keys[name]))
		case errors.Is(err, fs.ErrNotExist):
			note = "miss"
		default:
			note = "miss"
			b.u.Warn("%s: cache entry %s is unusable, rebuilding: %v", name, short(keys[name]), err)
		}
		if note == "miss" {
			if files, err = b.makepkg(ctx, name, filepath.Join(recipes, name), entry, srcdest, gnupg, infos[name]); err != nil {
				return "", err
			}
		}
		if err := b.run.Run(ctx, "", "cp", slices.Concat([]string{"--reflink=auto", "-t", built}, files)...); err != nil {
			return "", err
		}
		b.recipes = append(b.recipes, lap{name, note, time.Since(start)})
	}
	if err := prune(store, keys); err != nil {
		return "", err
	}
	pkgs, err := filepath.Glob(filepath.Join(built, "*.pkg.tar.zst"))
	if err != nil {
		return "", err
	}
	if err := b.run.Run(ctx, built, "repo-add", append([]string{"--quiet", Repo + ".db.tar.zst"}, pkgs...)...); err != nil {
		return "", err
	}
	return built, nil
}

// keys combines AUR commits or local recipe trees with direct runtime dependency versions.
// Build-dependency versions and upstream VCS revisions are not separate key inputs.
func (b *build) keys(ctx context.Context, recipes string, names []string, sources map[string]string) (map[string]string, map[string]recipeInfo, error) {
	scratch, err := b.scratch()
	if err != nil {
		return nil, nil, err
	}
	infos := map[string]recipeInfo{}
	provider := map[string]string{}
	for _, name := range names {
		dir := filepath.Join(recipes, name)
		data, err := os.ReadFile(filepath.Join(dir, ".SRCINFO"))
		info := string(data)
		if errors.Is(err, fs.ErrNotExist) {
			info, err = b.makepkgQuery(ctx, dir, scratch, "--printsrcinfo")
		}
		if err != nil {
			return nil, nil, fmt.Errorf("%s: .SRCINFO: %w", name, err)
		}
		infos[name] = srcinfo(info)
		provider[name] = name
		for _, p := range slices.Concat(infos[name].pkgnames, infos[name].provides) {
			provider[p] = name
		}
	}

	keys := map[string]string{}
	for _, name := range names {
		var deps []string
		for _, dep := range infos[name].depends {
			other, ok := provider[depName(dep)]
			switch {
			case !ok:
				deps = append(deps, dep)
			case other != name:
				return nil, nil, fmt.Errorf("%s: runtime depends %s is provided by recipe %s; recipes cannot depend on each other because misses build without the other recipe installed", name, dep, other)
			}
		}
		var terms []string
		info := infos[name]
		info.pins = map[string]pin{}
		global := slices.DeleteFunc(slices.Clone(deps), func(d string) bool { return !slices.Contains(info.global, d) })
		local := slices.DeleteFunc(slices.Clone(deps), func(d string) bool { return slices.Contains(info.global, d) })
		for _, group := range []struct {
			deps     []string
			required bool
		}{{local, false}, {global, true}} {
			if len(group.deps) == 0 {
				continue
			}
			out, err := b.run.Output(ctx, "", "pacman", b.pacman("official.conf", slices.Concat([]string{"-Sp", "-dd", "--noconfirm", "--print-format", "%n %v"}, group.deps)...)...)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: runtime depends %s do not resolve in core or extra: %w", name, strings.Join(group.deps, " "), err)
			}
			for line := range strings.Lines(out) {
				f := strings.Fields(line)
				if len(f) != 2 {
					return nil, nil, fmt.Errorf("%s: pacman -Sp: unexpected line %q", name, strings.TrimSpace(line))
				}
				terms = append(terms, "repo "+f[0]+" "+f[1])
				info.pins[f[0]] = pin{f[1], group.required}
			}
		}
		infos[name] = info
		keys[name] = fingerprint(sources[name], terms)
	}
	return keys, infos, nil
}

type recipeInfo struct {
	pkgnames, provides, depends, global []string
	pins                                map[string]pin
}

type pin struct {
	version  string
	required bool
}

func srcinfo(info string) recipeInfo {
	var r recipeInfo
	split := false
	for line := range strings.Lines(info) {
		field, value, ok := strings.Cut(strings.TrimSpace(line), " = ")
		if !ok || value == "" {
			continue
		}
		switch field {
		case "pkgname":
			split = true
			r.pkgnames = append(r.pkgnames, value)
		case "provides", "provides_x86_64":
			r.provides = append(r.provides, depName(value))
		case "depends", "depends_x86_64":
			r.depends = append(r.depends, value)
			if !split {
				r.global = append(r.global, value)
			}
		}
	}
	slices.Sort(r.depends)
	r.depends = slices.Compact(r.depends)
	slices.Sort(r.global)
	r.global = slices.Compact(r.global)
	return r
}

func depName(expr string) string {
	if i := strings.IndexAny(expr, "<>="); i >= 0 {
		return expr[:i]
	}
	return expr
}

func (b *build) scratch() (string, error) {
	dir := filepath.Join(b.tmp, "makepkg")
	return dir, mkdir(dir, b.owner)
}

func (b *build) makepkgQuery(ctx context.Context, dir, scratch string, args ...string) (string, error) {
	env := []string{"HOME=" + scratch, "BUILDDIR=" + scratch, "PKGDEST=" + scratch, "SRCDEST=" + scratch}
	conf := []string{"--config", filepath.Join(b.cache, "chroot", "root", "etc", "makepkg.conf")}
	argv := slices.Concat(b.as, env, []string{"makepkg"}, conf, args)
	return b.run.Output(ctx, dir, argv[0], argv[1:]...)
}

func fingerprint(source string, terms []string) string {
	h := sha256.New()
	fmt.Fprintf(h, "source %s\n", source)
	for _, t := range slices.Compact(slices.Sorted(slices.Values(terms))) {
		fmt.Fprintf(h, "%s\n", t)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// makepkg publishes a cache entry only after its packages and checksums are complete, using an atomic directory rename.
func (b *build) makepkg(ctx context.Context, name, dir, entry, srcdest, gnupg string, info recipeInfo) ([]string, error) {
	b.u.Info("makepkg %s in the chroot as %s", name, b.owner.Username)
	pkgdest := filepath.Join(b.tmp, "pkgdest", name)
	if err := os.MkdirAll(pkgdest, 0o755); err != nil {
		return nil, err
	}
	err := b.run.Run(ctx, "", "chown", "-R", b.owner.Username+":", dir)
	if err == nil {
		err = b.run.Run(ctx, dir, "env", "-i",
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/bin", "HOME=/root", "USER=root", "LANG=C.UTF-8", "TERM="+os.Getenv("TERM"), "PKGDEST="+pkgdest, "SRCDEST="+srcdest, "GNUPGHOME="+gnupg,
			"makechrootpkg", "-c", "-U", b.owner.Username, "-r", filepath.Join(b.cache, "chroot"))
	}
	if err := errors.Join(err, b.run.Run(ctx, "", "chown", "-R", "root:", dir)); err != nil {
		return nil, fmt.Errorf("build %s: %w", name, err)
	}
	if err := b.complete(ctx, dir, pkgdest, info); err != nil {
		return nil, fmt.Errorf("build %s: %w", name, err)
	}
	if err := writeSums(ctx, pkgdest); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(entry), 0o755); err != nil {
		return nil, err
	}
	if err := os.RemoveAll(entry); err != nil {
		return nil, err
	}
	if err := os.Rename(pkgdest, entry); err != nil {
		return nil, err
	}
	files, err := cached(ctx, entry)
	if err != nil {
		return nil, fmt.Errorf("build %s: %w", name, err)
	}
	return files, nil
}

func (b *build) complete(ctx context.Context, dir, pkgdest string, info recipeInfo) error {
	scratch, err := b.scratch()
	if err != nil {
		return err
	}
	list, err := b.makepkgQuery(ctx, dir, scratch, "--packagelist")
	if err != nil {
		return err
	}
	built, err := filepath.Glob(filepath.Join(pkgdest, "*.pkg.tar.zst"))
	if err != nil {
		return err
	}
	have := map[string]string{}
	for _, file := range built {
		pkg, _, _ := pkgfile(filepath.Base(file))
		have[pkg] = file
	}
	var main string
	for line := range strings.Lines(list) {
		want, _, ok := pkgfile(filepath.Base(strings.TrimSpace(line)))
		if !ok {
			return fmt.Errorf("makepkg --packagelist: unexpected line %q", strings.TrimSpace(line))
		}
		declared := slices.Contains(info.pkgnames, want)
		debug, isDebug := strings.CutSuffix(want, "-debug")
		optional := !declared && isDebug && slices.Contains(info.pkgnames, debug)
		file, ok := have[want]
		switch {
		case !ok && optional:
			continue
		case !ok:
			return fmt.Errorf("package %s is missing from %s", want, pkgdest)
		}
		if _, err := b.run.Output(ctx, "", "bsdtar", "-tf", file); err != nil {
			return fmt.Errorf("package %s is not a readable archive: %w", filepath.Base(file), err)
		}
		if main == "" && !optional {
			main = file
		}
	}
	if main == "" {
		return errors.New("makepkg --packagelist listed no declared packages")
	}
	buildinfo, err := b.run.Output(ctx, "", "bsdtar", "-xOf", main, ".BUILDINFO")
	if err != nil {
		return err
	}
	return drift(info.pins, buildinfo)
}

func drift(pins map[string]pin, buildinfo string) error {
	installed := map[string]string{}
	for line := range strings.Lines(buildinfo) {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "installed = ")
		if !ok {
			continue
		}
		if name, version, ok := pkgfile(value); ok {
			installed[name] = version
		}
	}
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(pins)) {
		want := pins[name]
		got, ok := installed[name]
		if !ok && !want.required {
			continue
		}
		if got != want.version {
			errs = append(errs, fmt.Errorf("runtime dependency %s was keyed at %s but the chroot built with %q", name, want.version, got))
		}
	}
	return errors.Join(errs...)
}

func pkgfile(s string) (name, version string, ok bool) {
	s = strings.TrimSuffix(s, ".pkg.tar.zst")
	f := strings.Split(s, "-")
	if len(f) < 4 {
		return "", "", false
	}
	n := len(f)
	return strings.Join(f[:n-3], "-"), f[n-3] + "-" + f[n-2], true
}

func cached(ctx context.Context, entry string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(entry, Sums))
	if err != nil {
		return nil, err
	}
	sums, err := parseSums(data)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, s := range sums {
		if strings.HasSuffix(s.name, ".pkg.tar.zst") {
			files = append(files, filepath.Join(entry, s.name))
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s lists no packages", filepath.Join(entry, Sums))
	}
	return files, VerifyPayload(ctx, entry)
}

func prune(store string, keys map[string]string) error {
	recipes, err := os.ReadDir(store)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, r := range recipes {
		dir := filepath.Join(store, r.Name())
		key, ok := keys[r.Name()]
		if !ok || !r.IsDir() {
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.Name() != key {
				if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (b *build) payload(ctx context.Context, l packages.Lists, built, payload string) ([]sized, []string, error) {
	if err := b.run.Run(ctx, "", "pacman", b.pacman("dctl.conf", "-Sy")...); err != nil {
		return nil, nil, err
	}
	targets := l.Payload()
	out, err := b.run.Output(ctx, "", "pacman", b.pacman("pacman.conf", slices.Concat([]string{"-Sp", "--print-format", "%r %n %f"}, targets)...)...)
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

	pkgs := filepath.Join(b.cache, "pkg")
	if err := os.MkdirAll(pkgs, 0o755); err != nil {
		return nil, nil, err
	}
	var official, files []string
	keep := map[string]bool{}
	for _, p := range closure {
		if p.Repo == Repo {
			files = append(files, filepath.Join(built, p.File))
			continue
		}
		official = append(official, p.Repo+"/"+p.Name)
		files = append(files, filepath.Join(pkgs, p.File))
		keep[p.File] = true
		keep[p.File+".sig"] = true
	}
	b.u.Info("fetching %d packages through %s", len(official), pkgs)
	if len(official) > 0 {
		if err := b.run.Run(ctx, "", "pacman", b.pacman("pacman.conf", slices.Concat([]string{"--cachedir", pkgs, "-Sw", "-dd", "--noconfirm"}, official)...)...); err != nil {
			return nil, nil, err
		}
	}
	entries, err := os.ReadDir(pkgs)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		if !keep[e.Name()] {
			if err := os.RemoveAll(filepath.Join(pkgs, e.Name())); err != nil {
				return nil, nil, err
			}
		}
	}
	if err := b.run.Run(ctx, "", "cp", slices.Concat([]string{"--reflink=auto", "-t", payload}, files)...); err != nil {
		return nil, nil, err
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

func (b *build) seed() error {
	sync := filepath.Join(b.resolve, "db", "sync")
	if err := os.MkdirAll(sync, 0o755); err != nil {
		return err
	}
	for _, repo := range []string{"core", "extra"} {
		db := repo + ".db"
		if err := copyFile(filepath.Join(b.cache, "chroot", "root", "var", "lib", "pacman", "sync", db), filepath.Join(sync, db)); err != nil {
			return fmt.Errorf("seed the resolve database from the build chroot: %w", err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func (b *build) confs(built string) error {
	if err := os.MkdirAll(filepath.Join(b.resolve, "db"), 0o755); err != nil {
		return err
	}
	options := "[options]\nArchitecture = auto\nSigLevel = Required DatabaseOptional\nParallelDownloads = 5\n"
	local := fmt.Sprintf("\n[%s]\nSigLevel = Never\nServer = file://%s\n", Repo, built)
	official := "\n[core]\nInclude = /etc/pacman.d/mirrorlist\n\n[extra]\nInclude = /etc/pacman.d/mirrorlist\n"
	for name, conf := range map[string]string{
		"official.conf": options + official,
		"dctl.conf":     options + local,
		"pacman.conf":   options + local + official,
	} {
		if err := os.WriteFile(filepath.Join(b.resolve, name), []byte(conf), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (b *build) pacman(conf string, args ...string) []string {
	return slices.Concat([]string{"--config", filepath.Join(b.resolve, conf), "--dbpath", filepath.Join(b.resolve, "db"), "--logfile", "/dev/null"}, args)
}

func (b *build) user(ctx context.Context, args ...string) error {
	argv := slices.Concat(b.as, args)
	return b.run.Run(ctx, "", argv[0], argv[1:]...)
}

func (b *build) output(ctx context.Context, args ...string) (string, error) {
	argv := slices.Concat(b.as, args)
	return b.run.Output(ctx, "", argv[0], argv[1:]...)
}

func short(key string) string { return key[:min(12, len(key))] }

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
