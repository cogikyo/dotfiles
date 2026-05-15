package install

// health.go defines install-step healthchecks and reusable check builders.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/health"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/repos"
)

func healthFor(ctx context.Context, root paths.Root, step string) []health.Check {
	runner := execx.OSRunner{}
	switch step {
	case "packages":
		return packageHealth(ctx, root, runner)
	case "link":
		return linkHealth(root)
	case "secrets":
		return secretsHealth(root)
	case "repos":
		return reposHealth(ctx, root, runner)
	case "system":
		return systemHealth(ctx, root, runner)
	case "hibernate":
		return hibernateHealth(ctx, runner)
	case "go":
		return goHealth(ctx, root, runner)
	case "fonts":
		return fontsHealth(root)
	case "eww":
		return ewwHealth(ctx, root, runner)
	case "firefox":
		return firefoxHealth(root)
	case "shell":
		return shellHealth(ctx, runner)
	case "dns":
		return dnsHealth(ctx, root, runner)
	default:
		return []health.Check{{ID: step, Name: step, Status: health.Fail, Observed: "unknown install step"}}
	}
}

func packageHealth(ctx context.Context, root paths.Root, runner execx.Runner) []health.Check {
	checks := []health.Check{commandCheck("packages:yay", "yay helper", "yay", health.Fail, "run dctl install packages")}

	for _, rel := range []string{"packages.lst", "packages-aur.lst"} {
		items, err := readSimpleList(root.Etc(rel))
		name := strings.TrimSuffix(rel, ".lst") + " list"
		if err != nil {
			checks = append(checks, fail("packages:"+rel, name, err.Error(), "restore "+root.Etc(rel)))
			continue
		}
		checks = append(checks, ok("packages:"+rel, name, fmt.Sprintf("%d packages declared", len(items))))
	}

	installed, err := installedPackages(ctx, runner, false)
	if err != nil {
		checks = append(checks, health.Check{ID: "packages:pacman-query", Name: "pacman package query", Status: health.Skip, Observed: err.Error()})
		return checks
	}
	missing := missingDeclared(root.Etc("packages.lst"), installed)
	checks = append(checks, packageListCheck("packages:pacman-installed", "repo packages installed", missing, "run dctl install packages"))

	aur, err := installedPackages(ctx, runner, true)
	if err != nil {
		checks = append(checks, health.Check{ID: "packages:aur-query", Name: "AUR package query", Status: health.Skip, Observed: err.Error()})
		return checks
	}
	missing = missingDeclared(root.Etc("packages-aur.lst"), aur)
	checks = append(checks, packageListCheck("packages:aur-installed", "AUR packages installed", missing, "run dctl install packages"))
	return checks
}

func linkHealth(root paths.Root) []health.Check {
	mappings, err := linkMappings(root)
	if err != nil {
		return []health.Check{fail("link:mappings", "link mappings", err.Error(), "run dctl install link")}
	}
	checks := make([]health.Check, 0, len(mappings)+1)
	for _, m := range mappings {
		id := "link:" + strings.TrimPrefix(m.dst, root.Home+string(os.PathSeparator))
		if _, err := os.Lstat(m.src); err != nil {
			checks = append(checks, fail(id, filepath.Base(m.dst), m.src+": "+err.Error(), "restore source or run dctl install link"))
			continue
		}
		if err := verifySymlink(m.src, m.dst); err != nil {
			checks = append(checks, fail(id, filepath.Base(m.dst), err.Error(), "run dctl install link"))
			continue
		}
		checks = append(checks, ok(id, filepath.Base(m.dst), m.dst))
	}
	scene := filepath.Join(root.Home, ".config", "obs-studio", "basic", "scenes", "Costello.json")
	if _, err := os.Stat(scene); err != nil {
		checks = append(checks, warn("link:obs-scene", "OBS scene file", scene+" missing", "run dctl install link"))
	} else {
		checks = append(checks, ok("link:obs-scene", "OBS scene file", scene))
	}
	return checks
}

func secretsHealth(root paths.Root) []health.Check {
	manifest := root.Etc("secrets", "manifest")
	entries, err := readSecretManifest(manifest, root.Home)
	if errors.Is(err, os.ErrNotExist) {
		return []health.Check{{ID: "secrets:manifest", Name: "secrets manifest", Status: health.Skip, Observed: "no secrets manifest"}}
	}
	if err != nil {
		return []health.Check{fail("secrets:manifest", "secrets manifest", err.Error(), "fix "+manifest)}
	}
	checks := []health.Check{ok("secrets:manifest", "secrets manifest", fmt.Sprintf("%d entries", len(entries)))}
	for _, p := range []string{"identity.age", "recipient.txt"} {
		path := root.Etc("secrets", p)
		if _, err := os.Stat(path); err != nil {
			checks = append(checks, warn("secrets:"+p, p, err.Error(), "run dctl secrets init or restore "+path))
		} else {
			checks = append(checks, ok("secrets:"+p, p, path))
		}
	}
	for _, e := range entries {
		ciphertext := root.Etc("secrets", e.name+".age")
		if _, err := os.Stat(ciphertext); err != nil {
			checks = append(checks, fail("secrets:"+e.name, e.name, "ciphertext missing: "+ciphertext, "run dctl secrets sync"))
			continue
		}
		if _, err := os.Stat(e.target); err != nil {
			checks = append(checks, warn("secrets:"+e.name, e.name, "target missing: "+e.target, "run dctl install secrets"))
			continue
		}
		checks = append(checks, ok("secrets:"+e.name, e.name, e.target))
	}
	return checks
}

func reposHealth(ctx context.Context, root paths.Root, runner execx.Runner) []health.Check {
	manifest := repos.ManifestPath(root)
	items, err := repos.LoadManifest(manifest)
	if err != nil {
		return []health.Check{fail("repos:manifest", "repos manifest", err.Error(), "fix "+manifest)}
	}
	checks := []health.Check{ok("repos:manifest", "repos manifest", fmt.Sprintf("%d repos", len(items)))}
	for _, repo := range items {
		target := paths.ExpandHome(root.Home, repo.Path)
		id := "repos:" + repo.Name
		if st, err := os.Stat(target); err != nil {
			checks = append(checks, fail(id, repo.Name, "missing: "+target, "run dctl install repos"))
			continue
		} else if !st.IsDir() {
			checks = append(checks, fail(id, repo.Name, "not a directory: "+target, "move it aside, then run dctl install repos"))
			continue
		}
		if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
			checks = append(checks, fail(id, repo.Name, "not a git repo: "+target, "reclone with dctl install repos"))
			continue
		}
		status, err := runner.Output(ctx, target, "git", "status", "--porcelain")
		if err != nil {
			checks = append(checks, warn(id, repo.Name, err.Error(), "inspect "+target))
			continue
		}
		if strings.TrimSpace(status) != "" {
			checks = append(checks, warn(id, repo.Name, "working tree has local changes", "inspect "+target))
			continue
		}
		if _, err := runner.Output(ctx, target, "git", "rev-parse", "--abbrev-ref", "@{upstream}"); err != nil {
			checks = append(checks, warn(id, repo.Name, "no upstream branch", "set upstream for "+target))
			continue
		}
		checks = append(checks, ok(id, repo.Name, target))
	}
	return checks
}

func systemHealth(ctx context.Context, root paths.Root, runner execx.Runner) []health.Check {
	keys := make([]string, 0, len(systemFiles))
	for key := range systemFiles {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	checks := make([]health.Check, 0, len(keys))
	for _, key := range keys {
		srcRel, _, _ := strings.Cut(key, "#")
		src := root.Etc(srcRel)
		dst := systemFiles[key]
		id := "system:" + dst
		if _, err := os.Stat(src); err != nil {
			checks = append(checks, fail(id, srcRel, "source missing: "+src, "restore "+src))
			continue
		}
		if _, err := os.Stat(dst); err != nil {
			checks = append(checks, fail(id, srcRel, "destination missing: "+dst, "run dctl install system"))
			continue
		}
		if !sameFileBytes(src, dst) {
			checks = append(checks, warn(id, srcRel, "destination differs: "+dst, "run dctl install system"))
			continue
		}
		checks = append(checks, ok(id, srcRel, dst))
	}
	checks = append(checks, serviceActiveCheck(ctx, runner, "system:earlyoom-active", "earlyoom service", "earlyoom", health.Warn, "run dctl install system"))
	checks = append(checks, sysctlCheck(ctx, runner, "system:swappiness", "vm.swappiness", "100"))
	return checks
}

func hibernateHealth(ctx context.Context, runner execx.Runner) []health.Check {
	checks := []health.Check{}
	fsType, err := runner.Output(ctx, "", "findmnt", "-no", "FSTYPE", "/")
	if err != nil {
		checks = append(checks, warn("hibernate:root-fs", "root filesystem", err.Error(), "install findmnt from util-linux"))
	} else if strings.TrimSpace(fsType) != "btrfs" {
		checks = append(checks, health.Check{ID: "hibernate:root-fs", Name: "root filesystem", Status: health.Skip, Expected: "btrfs", Observed: strings.TrimSpace(fsType)})
		return checks
	} else {
		checks = append(checks, ok("hibernate:root-fs", "root filesystem", "btrfs"))
	}
	if st, err := os.Stat("/swap/swapfile"); err != nil {
		checks = append(checks, fail("hibernate:swapfile", "swapfile", "/swap/swapfile missing", "run dctl install hibernate"))
	} else if st.Mode().Perm()&0o077 != 0 {
		checks = append(checks, warn("hibernate:swapfile", "swapfile permissions", st.Mode().Perm().String(), "chmod 600 /swap/swapfile"))
	} else {
		checks = append(checks, ok("hibernate:swapfile", "swapfile", "/swap/swapfile"))
	}
	checks = append(checks, fileContainsCheck("hibernate:fstab", "fstab swap entry", "/etc/fstab", "/swap/swapfile", health.Fail, "run dctl install hibernate"))
	checks = append(checks, fileContainsCheck("hibernate:loader", "boot resume params", "/boot/loader/entries/arch.conf", "resume=", health.Warn, "run dctl install hibernate"))
	checks = append(checks, fileContainsCheck("hibernate:mkinitcpio", "mkinitcpio resume hook", "/etc/mkinitcpio.conf", "resume", health.Warn, "run dctl install hibernate"))
	checks = append(checks, swapActiveCheck())
	return checks
}

func goHealth(ctx context.Context, root paths.Root, runner execx.Runner) []health.Check {
	checks := []health.Check{commandCheck("go:tool", "go toolchain", "go", health.Fail, "run dctl install packages")}
	for _, b := range goBinaries {
		path := goBinaryPath(root, b)
		id := "go:" + b.name
		if !executable(path) {
			checks = append(checks, fail(id, b.name, path+" missing or not executable", "run dctl install go"))
			continue
		}
		checks = append(checks, ok(id, b.name, path))
		if b.daemon {
			checks = append(checks, userServiceCheck(ctx, root, runner, b.name))
		}
	}
	return checks
}

func fontsHealth(root paths.Root) []health.Check {
	fontDir := filepath.Join(root.Home, ".local", "share", "fonts")
	archive := root.Etc("fonts.tar.gz")
	checks := []health.Check{}
	if _, err := os.Stat(archive); err != nil {
		checks = append(checks, warn("fonts:archive", "font archive", err.Error(), "restore "+archive))
	} else {
		checks = append(checks, ok("fonts:archive", "font archive", archive))
	}
	if !dirPopulated(fontDir) {
		checks = append(checks, fail("fonts:installed", "installed fonts", "font directory missing or empty", "run dctl install fonts"))
	} else {
		checks = append(checks, ok("fonts:installed", "installed fonts", fontDir))
	}
	return checks
}

func ewwHealth(ctx context.Context, root paths.Root, runner execx.Runner) []health.Check {
	checks := []health.Check{commandCheck("eww:cargo", "cargo toolchain", "cargo", health.Warn, "run dctl install packages")}
	path := filepath.Join(root.Home, ".local", "bin", "eww")
	if !executable(path) {
		checks = append(checks, fail("eww:binary", "eww binary", path+" missing or not executable", "run dctl install eww"))
	} else {
		checks = append(checks, ok("eww:binary", "eww binary", path))
	}
	patch := root.Etc("eww-poll-interval.patch")
	if _, err := os.Stat(patch); err != nil {
		checks = append(checks, fail("eww:patch", "eww patch", err.Error(), "restore "+patch))
	} else {
		checks = append(checks, ok("eww:patch", "eww patch", patch))
	}
	cache := filepath.Join(root.Home, ".cache", "eww")
	if _, err := os.Stat(filepath.Join(cache, ".git")); err != nil {
		checks = append(checks, warn("eww:cache", "eww source cache", cache+" missing", "run dctl install eww"))
		return checks
	}
	status, err := runner.Output(ctx, cache, "git", "status", "--porcelain")
	if err != nil {
		checks = append(checks, warn("eww:cache", "eww source cache", err.Error(), "inspect "+cache))
	} else if strings.TrimSpace(status) != "" {
		checks = append(checks, warn("eww:cache", "eww source cache", "working tree has local changes", "inspect "+cache))
	} else {
		checks = append(checks, ok("eww:cache", "eww source cache", cache))
	}
	return checks
}

func firefoxHealth(root paths.Root) []health.Check {
	profile, err := detectFirefoxProfile(root.Home)
	if err != nil {
		return []health.Check{{ID: "firefox:profile", Name: "Firefox Developer Edition profile", Status: health.Skip, Observed: err.Error()}}
	}
	checks := []health.Check{ok("firefox:profile", "Firefox Developer Edition profile", profile)}
	userJS := filepath.Join(profile, "user.js")
	if err := verifySymlink(root.Config("firefox", "user.js"), userJS); err != nil {
		checks = append(checks, fail("firefox:user.js", "Firefox user.js", err.Error(), "run dctl install firefox"))
	} else {
		checks = append(checks, ok("firefox:user.js", "Firefox user.js", userJS))
	}
	chrome := filepath.Join(profile, "chrome")
	if st, err := os.Stat(chrome); err != nil || !st.IsDir() {
		checks = append(checks, fail("firefox:chrome", "Firefox chrome directory", chrome+" missing", "run dctl install firefox"))
	} else {
		checks = append(checks, ok("firefox:chrome", "Firefox chrome directory", chrome))
	}
	css, err := filepath.Glob(filepath.Join(root.Home, "vagari", "firefox", "css", "*"))
	if err != nil || len(css) == 0 {
		checks = append(checks, fail("firefox:vagari-css", "vagari Firefox CSS", "no CSS files found", "run dctl install repos"))
		return checks
	}
	missing := []string{}
	for _, src := range css {
		dst := filepath.Join(chrome, filepath.Base(src))
		if err := verifySymlink(src, dst); err != nil {
			missing = append(missing, filepath.Base(src))
		}
	}
	if len(missing) > 0 {
		checks = append(checks, fail("firefox:vagari-css", "vagari Firefox CSS", "missing chrome links", "run dctl install firefox", missing...))
	} else {
		checks = append(checks, ok("firefox:vagari-css", "vagari Firefox CSS", fmt.Sprintf("%d CSS links", len(css))))
	}
	return checks
}

func shellHealth(ctx context.Context, runner execx.Runner) []health.Check {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		return []health.Check{fail("shell:zsh", "zsh binary", "zsh not found", "run dctl install shell")}
	}
	checks := []health.Check{ok("shell:zsh", "zsh binary", zsh)}
	checks = append(checks, fileContainsCheck("shell:etc-shells", "/etc/shells entry", "/etc/shells", zsh, health.Warn, "add "+zsh+" to /etc/shells"))
	user := os.Getenv("USER")
	if user == "" {
		checks = append(checks, health.Check{ID: "shell:login", Name: "login shell", Status: health.Skip, Observed: "USER not set"})
		return checks
	}
	passwd, err := runner.Output(ctx, "", "getent", "passwd", user)
	if err != nil {
		checks = append(checks, health.Check{ID: "shell:login", Name: "login shell", Status: health.Skip, Observed: err.Error()})
		return checks
	}
	fields := strings.Split(strings.TrimSpace(passwd), ":")
	if len(fields) < 7 {
		checks = append(checks, warn("shell:login", "login shell", "unexpected passwd entry", "inspect getent passwd "+user))
	} else if filepath.Base(fields[6]) != filepath.Base(zsh) {
		checks = append(checks, warn("shell:login", "login shell", fields[6], "run dctl install shell"))
	} else {
		checks = append(checks, ok("shell:login", "login shell", fields[6]))
	}
	return checks
}

func dnsHealth(ctx context.Context, root paths.Root, runner execx.Runner) []health.Check {
	checks := []health.Check{}
	if target, err := filepath.EvalSymlinks("/etc/resolv.conf"); err != nil {
		checks = append(checks, fail("dns:resolv-conf", "resolv.conf stub", err.Error(), "run dctl install dns"))
	} else if target != "/run/systemd/resolve/stub-resolv.conf" {
		checks = append(checks, fail("dns:resolv-conf", "resolv.conf stub", target, "run dctl install dns"))
	} else {
		checks = append(checks, ok("dns:resolv-conf", "resolv.conf stub", target))
	}
	checks = append(checks, systemFileMatchCheck("dns:resolved-conf", "systemd-resolved config", root.Etc("systemd", "resolved.conf"), "/etc/systemd/resolved.conf", health.Fail, "run dctl install dns"))
	checks = append(checks, fileContainsCheck("dns:nm-dropin", "NetworkManager DNS drop-in", "/etc/NetworkManager/conf.d/10-dotfiles-dns.conf", "dns=systemd-resolved", health.Warn, "run dctl install dns"))
	checks = append(checks, serviceActiveCheck(ctx, runner, "dns:resolved-service", "systemd-resolved service", "systemd-resolved", health.Warn, "systemctl start systemd-resolved"))
	return checks
}

type secretEntry struct{ name, target string }

func readSecretManifest(path, home string) ([]secretEntry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []secretEntry
	for n, line := range strings.Split(string(b), "\n") {
		line, _, _ = strings.Cut(line, "#")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 3 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("invalid secrets manifest line %d", n+1)
		}
		entries = append(entries, secretEntry{name: strings.TrimSpace(parts[0]), target: expandHome(home, strings.TrimSpace(parts[1]))})
	}
	return entries, nil
}

func installedPackages(ctx context.Context, runner execx.Runner, foreign bool) (map[string]bool, error) {
	args := []string{"-Qq"}
	if foreign {
		args = []string{"-Qmq"}
	}
	out, err := runner.Output(ctx, "", "pacman", args...)
	if err != nil {
		return nil, err
	}
	installed := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			installed[line] = true
		}
	}
	return installed, nil
}

func missingDeclared(path string, installed map[string]bool) []string {
	declared, err := readSimpleList(path)
	if err != nil {
		return []string{err.Error()}
	}
	var missing []string
	for _, pkg := range declared {
		if !installed[pkg] {
			missing = append(missing, pkg)
		}
	}
	return missing
}

func packageListCheck(id, name string, missing []string, fix string) health.Check {
	if len(missing) == 0 {
		return ok(id, name, "all declared packages installed")
	}
	return health.Check{ID: id, Name: name, Status: health.Fail, Observed: fmt.Sprintf("%d missing", len(missing)), Fix: fix, Details: compactDetails(missing, 12)}
}

func commandCheck(id, name, cmd string, missing health.Status, fix string) health.Check {
	path, err := exec.LookPath(cmd)
	if err != nil {
		return health.Check{ID: id, Name: name, Status: missing, Observed: cmd + " not found", Fix: fix}
	}
	return ok(id, name, path)
}

func fileContainsCheck(id, name, path, needle string, missing health.Status, fix string) health.Check {
	b, err := os.ReadFile(path)
	if err != nil {
		return health.Check{ID: id, Name: name, Status: missing, Observed: err.Error(), Fix: fix}
	}
	if !strings.Contains(string(b), needle) {
		return health.Check{ID: id, Name: name, Status: missing, Expected: "contains " + needle, Observed: "not found", Fix: fix}
	}
	return ok(id, name, path)
}

func systemFileMatchCheck(id, name, src, dst string, missing health.Status, fix string) health.Check {
	if _, err := os.Stat(src); err != nil {
		return health.Check{ID: id, Name: name, Status: health.Fail, Observed: "source missing: " + src, Fix: "restore " + src}
	}
	if _, err := os.Stat(dst); err != nil {
		return health.Check{ID: id, Name: name, Status: missing, Observed: err.Error(), Fix: fix}
	}
	if !sameFileBytes(src, dst) {
		return health.Check{ID: id, Name: name, Status: missing, Observed: dst + " differs", Fix: fix}
	}
	return ok(id, name, dst)
}

func serviceActiveCheck(ctx context.Context, runner execx.Runner, id, name, service string, inactive health.Status, fix string) health.Check {
	out, err := runner.Output(ctx, "", "systemctl", "is-active", service)
	if err != nil {
		return health.Check{ID: id, Name: name, Status: inactive, Observed: strings.TrimSpace(out), Fix: fix}
	}
	return ok(id, name, strings.TrimSpace(out))
}

func sysctlCheck(ctx context.Context, runner execx.Runner, id, key, expected string) health.Check {
	out, err := runner.Output(ctx, "", "sysctl", "-n", key)
	observed := strings.TrimSpace(out)
	if err != nil {
		return warn(id, key, err.Error(), "run dctl install system")
	}
	if observed != expected {
		return warn(id, key, observed, "run sudo sysctl --system or reboot")
	}
	return ok(id, key, observed)
}

func userServiceCheck(ctx context.Context, root paths.Root, runner execx.Runner, name string) health.Check {
	path := filepath.Join(root.Home, ".config", "systemd", "user", name+".service")
	if _, err := os.Stat(path); err != nil {
		return warn("go:"+name+":service", name+" user service", path+" missing", "run dctl install go")
	}
	out, err := runner.Output(ctx, "", "systemctl", "--user", "is-enabled", name)
	if err != nil {
		return warn("go:"+name+":service", name+" user service", strings.TrimSpace(out), "systemctl --user enable --now "+name)
	}
	return ok("go:"+name+":service", name+" user service", strings.TrimSpace(out))
}

func swapActiveCheck() health.Check {
	b, err := os.ReadFile("/proc/swaps")
	if err != nil {
		return warn("hibernate:active-swap", "active swap", err.Error(), "inspect /proc/swaps")
	}
	if !strings.Contains(string(b), "/swap/swapfile") {
		return warn("hibernate:active-swap", "active swap", "/swap/swapfile not active", "swapon /swap/swapfile")
	}
	return ok("hibernate:active-swap", "active swap", "/swap/swapfile")
}

func goBinaryPath(root paths.Root, b goBinary) string {
	dir := filepath.Join(root.Home, ".local", "bin")
	if b.outputDir != "" {
		dir = filepath.Join(root.Dotfiles, b.outputDir)
	}
	return filepath.Join(dir, b.name)
}

func expandHome(home, p string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, strings.TrimPrefix(p, "~/"))
	}
	return p
}

func compactDetails(items []string, limit int) []string {
	if len(items) <= limit {
		return items
	}
	out := slices.Clone(items[:limit])
	out = append(out, fmt.Sprintf("... %d more", len(items)-limit))
	return out
}

func ok(id, name, observed string) health.Check {
	return health.Check{ID: id, Name: name, Status: health.OK, Observed: observed}
}

func warn(id, name, observed, fix string) health.Check {
	return health.Check{ID: id, Name: name, Status: health.Warn, Observed: observed, Fix: fix}
}

func fail(id, name, observed, fix string, details ...string) health.Check {
	return health.Check{ID: id, Name: name, Status: health.Fail, Observed: observed, Fix: fix, Details: details}
}
