// Package iso builds, verifies, writes, and releases the custom Arch ISO.
//
// Responsibilities:
// - Prepare the archiso profile with dotfiles and cached packages.
// - Validate USB targets before destructive writes.
// - Gate release publishing on branch, tag, auth, and worktree safety checks.
package iso

// iso.go defines ISO command handlers, build orchestration, USB validation, and release publishing.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dotfiles/cmds/internal/dctl/app"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/output"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/pkglist"
	"dotfiles/cmds/internal/dctl/prompt"
)

const version = "0.1.0"

var basePackages = []string{"base", "linux", "linux-firmware", "base-devel", "git", "networkmanager"}

type Cmd struct {
	Build   BuildCmd   `cmd:"" help:"Build a custom Arch ISO."`
	Verify  VerifyCmd  `cmd:"" help:"Verify ISO build inputs and outputs."`
	USB     USBCmd     `cmd:"" name:"usb" help:"Write newest ISO to a USB device."`
	Release ReleaseCmd `cmd:"" help:"Publish an ISO release."`
}
type BuildCmd struct {
	Clean   bool `help:"Clean work/out dirs first."`
	SkipAUR bool `name:"skip-aur" help:"Skip AUR package builds."`
}
type VerifyCmd struct{}
type USBCmd struct {
	Device string `arg:"" optional:"" help:"Whole USB block device, e.g. /dev/sdX."`
}
type ReleaseCmd struct {
	Tag string `help:"Release tag."`
}

type isoPaths struct {
	dotfiles    string
	profile     string
	work        string
	out         string
	packages    string
	aurPackages string
	localRepo   string
}

type isoFile struct {
	Path    string
	ModTime time.Time
}

type lsblkOutput struct {
	BlockDevices []blockDevice `json:"blockdevices"`
}

type blockDevice struct {
	Name        string        `json:"name"`
	Path        string        `json:"path"`
	Type        string        `json:"type"`
	Tran        string        `json:"tran"`
	Size        json.Number   `json:"size"`
	Mountpoints []string      `json:"mountpoints"`
	Children    []blockDevice `json:"children"`
}

func (c *BuildCmd) Run(ctx *app.Context) error {
	runner := execx.OSRunner{IO: true}
	return build(ctx.Context, ctx.Root, ctx.Output, runner, c.Clean, c.SkipAUR)
}

func (c *VerifyCmd) Run(ctx *app.Context) error {
	runner := execx.OSRunner{}
	return verify(ctx.Context, ctx.Root, ctx.Output, runner)
}

func (c *USBCmd) Run(ctx *app.Context) error {
	runner := execx.OSRunner{IO: true}
	return writeUSB(ctx.Context, ctx.Root, ctx.Output, runner, c.Device, ctx.Yes)
}

func (c *ReleaseCmd) Run(ctx *app.Context) error {
	runner := execx.OSRunner{IO: true}
	return release(ctx.Context, ctx.Root, ctx.Output, runner, c.Tag, ctx.Yes)
}

func build(ctx context.Context, root paths.Root, out *output.Printer, runner execx.Runner, clean bool, skipAUR bool) error {
	p := newPaths(root)
	if err := requireRoot(); err != nil {
		return err
	}
	if err := preflight(ctx, p, out, runner, false); err != nil {
		return err
	}

	out.Header("ISO Build")
	if clean {
		out.Step("Cleaning previous artifacts")
		if err := os.RemoveAll(p.work); err != nil {
			return err
		}
		if err := os.RemoveAll(p.out); err != nil {
			return err
		}
	}
	if err := prepareProfile(p, out); err != nil {
		return err
	}
	if err := cacheRepoPackages(ctx, p, out, runner); err != nil {
		return err
	}
	if err := buildAURPackages(ctx, p, out, runner, skipAUR); err != nil {
		return err
	}
	if err := createRepoDB(ctx, p, out, runner); err != nil {
		return err
	}
	if err := run(ctx, runner, "", "mkarchiso", "-v", "-w", p.work, "-o", p.out, p.profile); err != nil {
		return err
	}
	iso, err := newestISO(p.out)
	if err != nil {
		return err
	}
	out.OK("ISO built: %s", iso)
	out.Dim("QEMU: qemu-system-x86_64 -cdrom %s -m 8G -smp 4 -enable-kvm", iso)
	return nil
}

func verify(ctx context.Context, root paths.Root, out *output.Printer, runner execx.Runner) error {
	p := newPaths(root)
	out.Header("ISO Verify")
	if err := preflight(ctx, p, out, runner, true); err != nil {
		return err
	}
	repo, err := readPackageFile(p.packages)
	if err != nil {
		return err
	}
	aur, err := readPackageFile(p.aurPackages)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	out.KV("repo packages", len(repo))
	out.KV("aur packages", len(aur))
	if err := verifyLocalRepo(p.localRepo); err != nil {
		return err
	}
	iso, err := newestISO(p.out)
	if err != nil {
		out.Warn("No ISO found in %s", p.out)
	} else {
		out.KV("newest iso", iso)
	}
	out.OK("ISO inputs verified")
	return nil
}

// writeUSB writes the newest ISO to a whole USB disk after root, size, mount, and transport checks.
func writeUSB(ctx context.Context, root paths.Root, out *output.Printer, runner execx.Runner, device string, yes bool) error {
	p := newPaths(root)
	if err := requireRoot(); err != nil {
		return err
	}
	iso, err := newestISO(p.out)
	if err != nil {
		return err
	}
	if strings.TrimSpace(device) == "" {
		return fmt.Errorf("USB device is required, e.g. dctl iso usb /dev/sdX")
	}
	info, err := inspectBlockDevice(ctx, runner, device)
	if err != nil {
		return err
	}
	isoStat, err := os.Stat(iso)
	if err != nil {
		return err
	}
	if err := validateUSBTarget(info, isoStat.Size()); err != nil {
		return err
	}
	out.Warn("This will erase all data on %s", device)
	out.KV("iso", iso)
	out.KV("device", fmt.Sprintf("%s (%d bytes)", info.Path, blockSize(info)))
	if !yes {
		ok, err := prompt.Confirm("Write ISO to this whole USB disk?", false)
		if err != nil {
			return err
		}
		if !ok {
			out.Info("Aborted")
			return nil
		}
	}
	return run(ctx, runner, "", "dd", "bs=4M", "if="+iso, "of="+device, "status=progress", "oflag=sync")
}

// release publishes the newest ISO by tagging and pushing master, then creating a GitHub release.
func release(ctx context.Context, root paths.Root, out *output.Printer, runner execx.Runner, tag string, yes bool) error {
	p := newPaths(root)
	iso, err := newestISO(p.out)
	if err != nil {
		return err
	}
	capture := execx.OSRunner{}
	if err := requireCommands("git", "gh"); err != nil {
		return err
	}
	branch, err := capture.Output(ctx, p.dotfiles, "git", "branch", "--show-current")
	if err != nil {
		return err
	}
	if strings.TrimSpace(branch) != "master" {
		return fmt.Errorf("release requires master branch, currently on %q", strings.TrimSpace(branch))
	}
	if _, err := capture.Run(ctx, p.dotfiles, "gh", "auth", "status"); err != nil {
		return fmt.Errorf("gh is not authenticated: %w", err)
	}
	if tag == "" {
		tag = "iso-" + time.Now().Format("2006.01.02")
	}
	if _, err := capture.Run(ctx, p.dotfiles, "git", "rev-parse", "refs/tags/"+tag); err == nil {
		return fmt.Errorf("tag already exists: %s", tag)
	}
	dirty, err := gitDirty(ctx, capture, p.dotfiles)
	if err != nil {
		return err
	}
	if dirty {
		return errors.New("release requires a clean git worktree; commit intentional changes first")
	}
	out.Warn("Release will tag, push master, push %s, and upload %s", tag, filepath.Base(iso))
	if !yes {
		ok, err := prompt.Confirm("Publish this ISO release?", false)
		if err != nil {
			return err
		}
		if !ok {
			out.Info("Aborted")
			return nil
		}
	}
	if err := run(ctx, runner, p.dotfiles, "git", "tag", "-a", tag, "-m", tag); err != nil {
		return err
	}
	if err := run(ctx, runner, p.dotfiles, "git", "push", "origin", "master"); err != nil {
		return err
	}
	if err := run(ctx, runner, p.dotfiles, "git", "push", "origin", tag); err != nil {
		return err
	}
	notes := fmt.Sprintf("Custom Arch Linux ISO with pre-cached packages (repo + AUR).\n\nBuilt with `dctl iso v%s`.\n\nISO: `%s`", version, filepath.Base(iso))
	if err := run(ctx, runner, p.dotfiles, "gh", "release", "create", tag, "--title", tag, "--notes", notes, iso); err != nil {
		return err
	}
	out.OK("Release published: %s", tag)
	return nil
}

func preflight(ctx context.Context, p isoPaths, out *output.Printer, runner execx.Runner, verifyOnly bool) error {
	commands := []string{"mkarchiso", "pacman", "makepkg", "repo-add", "git"}
	if verifyOnly {
		commands = []string{"mkarchiso", "pacman", "repo-add", "git"}
	}
	for _, cmd := range commands {
		if _, err := exec.LookPath(cmd); err != nil {
			return fmt.Errorf("missing required command %q", cmd)
		}
	}
	if _, err := os.Stat(p.profile); err != nil {
		return fmt.Errorf("ISO profile not found: %s", p.profile)
	}
	if _, err := os.Stat(p.packages); err != nil {
		return fmt.Errorf("package list not found: %s", p.packages)
	}
	if _, err := runner.Run(ctx, "", "pacman", "-V"); err != nil {
		return err
	}
	out.KV("dotfiles", p.dotfiles)
	out.KV("profile", p.profile)
	return nil
}

func prepareProfile(p isoPaths, out *output.Printer) error {
	out.Step("Preparing ISO profile")
	if err := os.MkdirAll(p.work, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(p.out, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(p.localRepo, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(p.profile, "airootfs", "root", "dotfiles")
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := copyProfile(p.dotfiles, dst); err != nil {
		return err
	}
	out.OK("Dotfiles copied to live environment")
	return nil
}

func cacheRepoPackages(ctx context.Context, p isoPaths, out *output.Printer, runner execx.Runner) error {
	packages, err := readPackageFile(p.packages)
	if err != nil {
		return err
	}
	packages = pkglist.Unique(append(append([]string{}, basePackages...), packages...))
	out.Step("Caching %d repo packages plus dependencies", len(packages))
	args := append([]string{"-Syw", "--noconfirm", "--needed", "--cachedir", p.localRepo, "--dbpath", "/var/lib/pacman"}, packages...)
	if err := run(ctx, runner, "", "pacman", args...); err != nil {
		out.Warn("Repo cache failed; retrying with packages present in sync DBs")
		valid := make([]string, 0, len(packages))
		for _, pkg := range packages {
			if _, err := runner.Run(ctx, "", "pacman", "-Si", pkg); err == nil {
				valid = append(valid, pkg)
			} else {
				out.Warn("Skipping non-repo package: %s", pkg)
			}
		}
		if len(valid) == 0 {
			return fmt.Errorf("no valid repo packages found")
		}
		args = append([]string{"-Syw", "--noconfirm", "--needed", "--cachedir", p.localRepo, "--dbpath", "/var/lib/pacman"}, valid...)
		if err := run(ctx, runner, "", "pacman", args...); err != nil {
			return err
		}
	}
	return nil
}

func buildAURPackages(ctx context.Context, p isoPaths, out *output.Printer, runner execx.Runner, skip bool) error {
	if skip {
		out.Warn("Skipping AUR builds")
		return nil
	}
	packages, err := readPackageFile(p.aurPackages)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			out.Info("No AUR package list found")
			return nil
		}
		return err
	}
	if len(packages) == 0 {
		out.Info("No AUR packages to build")
		return nil
	}
	buildUser := "_isobuild"
	buildHome := "/tmp/_isobuild"
	createdBuildUser := false
	if _, err := user.Lookup(buildUser); err != nil {
		if err := run(ctx, runner, "", "useradd", "-m", "-d", buildHome, "-s", "/bin/bash", buildUser); err != nil {
			return err
		}
		createdBuildUser = true
		if err := os.WriteFile(filepath.Join("/etc/sudoers.d", buildUser), []byte(buildUser+" ALL=(ALL) NOPASSWD: ALL\n"), 0o440); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("build user %s already exists; remove it manually or choose a clean build host", buildUser)
	}
	defer func() {
		if createdBuildUser {
			_ = run(ctx, runner, "", "userdel", "-r", buildUser)
			_ = os.Remove(filepath.Join("/etc/sudoers.d", buildUser))
		}
	}()
	_ = os.MkdirAll(buildHome, 0o755)
	_ = run(ctx, runner, "", "chown", "-R", buildUser+":"+buildUser, buildHome)
	_ = run(ctx, runner, "", "pacman", "-S", "--noconfirm", "--needed", "perl")
	_ = run(ctx, runner, "", "sudo", "-u", buildUser, "env", "RUSTUP_HOME="+filepath.Join(buildHome, ".rustup"), "CARGO_HOME="+filepath.Join(buildHome, ".cargo"), "rustup", "default", "stable")

	failed := 0
	for _, pkg := range packages {
		if cachedPackageExists(p.localRepo, pkg) {
			continue
		}
		buildDir := filepath.Join(buildHome, pkg)
		_ = os.RemoveAll(buildDir)
		out.Step("Building AUR: %s", pkg)
		if err := run(ctx, runner, "", "sudo", "-u", buildUser, "git", "clone", "--depth", "1", "https://aur.archlinux.org/"+pkg+".git", buildDir); err != nil {
			out.Warn("Failed to clone AUR package: %s", pkg)
			failed++
			continue
		}
		if err := run(ctx, runner, buildDir, "sudo", "-u", buildUser, "env", "RUSTUP_HOME="+filepath.Join(buildHome, ".rustup"), "CARGO_HOME="+filepath.Join(buildHome, ".cargo"), "PATH="+filepath.Join(buildHome, ".cargo/bin")+":/usr/bin/core_perl:"+os.Getenv("PATH"), "makepkg", "-s", "--noconfirm", "--noprogressbar"); err != nil {
			out.Warn("Failed to build AUR package: %s", pkg)
			failed++
			continue
		}
		if err := copyBuiltPackages(buildDir, p.localRepo); err != nil {
			out.Warn("No package file copied for %s: %v", pkg, err)
			failed++
		}
		_ = os.RemoveAll(buildDir)
	}
	if removed, err := removeDebugPackages(p.localRepo); err == nil && removed > 0 {
		out.Info("Removed %d debug packages", removed)
	}
	if failed > 0 {
		out.Warn("%d AUR packages failed; install may fall back to network", failed)
	}
	return nil
}

func createRepoDB(ctx context.Context, p isoPaths, out *output.Printer, runner execx.Runner) error {
	packages, err := packageFiles(p.localRepo)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		return fmt.Errorf("no packages found in localrepo")
	}
	for _, name := range []string{"localrepo.db", "localrepo.db.tar.zst", "localrepo.files", "localrepo.files.tar.zst"} {
		_ = os.Remove(filepath.Join(p.localRepo, name))
	}
	out.Step("Creating localrepo database for %d packages", len(packages))
	return run(ctx, runner, "", "repo-add", append([]string{filepath.Join(p.localRepo, "localrepo.db.tar.zst")}, packages...)...)
}

func newPaths(root paths.Root) isoPaths {
	profile := filepath.Join(root.Dotfiles, "iso")
	return isoPaths{
		dotfiles:    root.Dotfiles,
		profile:     profile,
		work:        filepath.Join(profile, "work"),
		out:         filepath.Join(profile, "out"),
		packages:    filepath.Join(root.Dotfiles, "etc", "packages.lst"),
		aurPackages: filepath.Join(root.Dotfiles, "etc", "packages-aur.lst"),
		localRepo:   filepath.Join(profile, "airootfs", "var", "cache", "localrepo"),
	}
}

func readPackageFile(path string) ([]string, error) {
	return pkglist.Read(path)
}

func parsePackageList(r io.Reader) []string {
	return pkglist.Parse(r)
}

func copyProfile(src string, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		if shouldExcludeProfileRel(filepath.ToSlash(rel)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()
		switch {
		case mode&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case d.IsDir():
			return os.MkdirAll(target, mode.Perm())
		case mode.IsRegular():
			return copyFile(path, target, mode.Perm())
		default:
			return nil
		}
	})
}

func shouldExcludeProfileRel(rel string) bool {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	if rel == ".git" || strings.HasPrefix(rel, ".git/") {
		return true
	}
	exact := map[string]bool{
		"iso/work":                   true,
		"iso/out":                    true,
		"iso/airootfs/var":           true,
		"iso/airootfs/root/dotfiles": true,
		"cmds/cmd/newtab/dna.webm":   true,
		"etc/fonts.tar.gz":           true,
	}
	if exact[rel] {
		return true
	}
	return strings.HasPrefix(rel, "iso/work/") || strings.HasPrefix(rel, "iso/out/") || strings.HasPrefix(rel, "iso/airootfs/var/") || strings.HasPrefix(rel, "iso/airootfs/root/dotfiles/") || (strings.HasPrefix(rel, "share/videos/") && strings.HasSuffix(rel, ".mp4"))
}

func copyFile(src string, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func newestISO(dir string) (string, error) {
	var files []isoFile
	if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".iso") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files = append(files, isoFile{Path: path, ModTime: info.ModTime()})
		return nil
	}); err != nil {
		return "", err
	}
	return selectNewestISO(files)
}

func selectNewestISO(files []isoFile) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no ISO found")
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].ModTime.Equal(files[j].ModTime) {
			return files[i].Path > files[j].Path
		}
		return files[i].ModTime.After(files[j].ModTime)
	})
	return files[0].Path, nil
}

func verifyLocalRepo(dir string) error {
	packages, err := packageFiles(dir)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		return fmt.Errorf("localrepo has no package archives: %s", dir)
	}
	for _, name := range []string{"localrepo.db", "localrepo.db.tar.zst"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return nil
		}
	}
	return fmt.Errorf("localrepo database missing in %s", dir)
}

func packageFiles(dir string) ([]string, error) {
	var packages []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isPackageArchive(d.Name()) {
			return nil
		}
		packages = append(packages, path)
		return nil
	})
	sort.Strings(packages)
	return packages, err
}

func isPackageArchive(name string) bool {
	return strings.Contains(name, ".pkg.tar.") && !strings.HasSuffix(name, ".sig")
}

func cachedPackageExists(dir string, pkg string) bool {
	files, err := packageFiles(dir)
	if err != nil {
		return false
	}
	prefix := pkg + "-"
	for _, file := range files {
		if strings.HasPrefix(filepath.Base(file), prefix) {
			return true
		}
	}
	return false
}

func copyBuiltPackages(buildDir string, repo string) error {
	files, err := packageFiles(buildDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no built package archives")
	}
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			return err
		}
		if err := copyFile(file, filepath.Join(repo, filepath.Base(file)), info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func removeDebugPackages(repo string) (int, error) {
	files, err := packageFiles(repo)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, file := range files {
		if strings.Contains(filepath.Base(file), "-debug-") {
			if err := os.Remove(file); err != nil {
				return removed, err
			}
			removed++
		}
	}
	return removed, nil
}

func inspectBlockDevice(ctx context.Context, runner execx.Runner, device string) (blockDevice, error) {
	if !strings.HasPrefix(device, "/dev/") {
		return blockDevice{}, fmt.Errorf("USB target must be an absolute /dev path")
	}
	if fi, err := os.Stat(device); err != nil {
		return blockDevice{}, err
	} else if fi.Mode()&os.ModeDevice == 0 {
		return blockDevice{}, fmt.Errorf("not a block device: %s", device)
	}
	out, err := runner.Output(ctx, "", "lsblk", "-J", "-b", "-o", "NAME,PATH,TYPE,TRAN,SIZE,MOUNTPOINTS", device)
	if err != nil {
		return blockDevice{}, err
	}
	var parsed lsblkOutput
	dec := json.NewDecoder(strings.NewReader(out))
	dec.UseNumber()
	if err := dec.Decode(&parsed); err != nil {
		return blockDevice{}, err
	}
	if len(parsed.BlockDevices) != 1 {
		return blockDevice{}, fmt.Errorf("expected exactly one block device from lsblk")
	}
	return parsed.BlockDevices[0], nil
}

// validateUSBTarget rejects anything except an unmounted whole USB disk large enough for the ISO.
func validateUSBTarget(dev blockDevice, isoSize int64) error {
	if dev.Type != "disk" {
		return fmt.Errorf("USB target must be a whole disk, got type %q", dev.Type)
	}
	if dev.Tran != "usb" {
		return fmt.Errorf("refusing non-USB device %s with transport %q", dev.Path, dev.Tran)
	}
	if hasMountpoints(dev) {
		return fmt.Errorf("%s or one of its partitions is mounted", dev.Path)
	}
	if size := blockSize(dev); size <= 0 {
		return fmt.Errorf("could not determine size for %s", dev.Path)
	} else if isoSize > size {
		return fmt.Errorf("ISO (%d bytes) is larger than device (%d bytes)", isoSize, size)
	}
	return nil
}

func hasMountpoints(dev blockDevice) bool {
	for _, mount := range dev.Mountpoints {
		if strings.TrimSpace(mount) != "" {
			return true
		}
	}
	for _, child := range dev.Children {
		if hasMountpoints(child) {
			return true
		}
	}
	return false
}

func blockSize(dev blockDevice) int64 {
	size, _ := dev.Size.Int64()
	return size
}

func gitDirty(ctx context.Context, runner execx.Runner, dir string) (bool, error) {
	if _, err := runner.Run(ctx, dir, "git", "diff", "--quiet", "HEAD"); err != nil {
		return true, nil
	}
	untracked, err := runner.Output(ctx, dir, "git", "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(untracked) != "", nil
}

func requireRoot() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("this operation must run as root")
	}
	return nil
}

func requireCommands(commands ...string) error {
	for _, command := range commands {
		if _, err := exec.LookPath(command); err != nil {
			return fmt.Errorf("missing required command %q", command)
		}
	}
	return nil
}

func run(ctx context.Context, runner execx.Runner, dir string, name string, args ...string) error {
	_, err := runner.Run(ctx, dir, name, args...)
	return err
}
