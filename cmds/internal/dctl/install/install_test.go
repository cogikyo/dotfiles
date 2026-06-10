package install

// install_test.go covers install step contracts, symlink planning, and healthcheck behavior.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dotfiles/cmds/internal/dctl/app"
	"dotfiles/cmds/internal/dctl/health"
	"dotfiles/cmds/internal/dctl/output"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/tui"
)

func TestStepRegistryIncludesSafeUserSteps(t *testing.T) {
	for _, name := range []string{"link", "repos", "go", "fonts", "firefox"} {
		if _, ok := FindStep(name); !ok {
			t.Fatalf("missing step %q", name)
		}
	}
}

func TestStepMetadataDocumentsRiskAndDryRun(t *testing.T) {
	for _, def := range stepDefs {
		if def.Risk == "" {
			t.Fatalf("step %q missing risk metadata", def.Name)
		}
	}
	for _, name := range []string{"packages", "link", "system", "hibernate", "fonts", "go", "eww", "firefox", "shell", "dns"} {
		def, ok := FindStep(name)
		if !ok {
			t.Fatalf("missing step %q", name)
		}
		if !def.SupportsDryRun {
			t.Fatalf("step %q should support dry-run", name)
		}
	}
	for _, name := range []string{"secrets", "repos"} {
		def, ok := FindStep(name)
		if !ok {
			t.Fatalf("missing step %q", name)
		}
		if def.SupportsDryRun {
			t.Fatalf("step %q should not advertise dry-run", name)
		}
	}
}

func TestInstallCatalogListsSubcommands(t *testing.T) {
	catalog := installCatalog()
	for _, name := range []string{"all", "packages", "link", "go", "list", "check"} {
		if !catalogHas(catalog.Children, name) {
			t.Fatalf("install catalog missing %s", name)
		}
	}
}

func TestUnsupportedDirectDryRunFailsBeforeMutation(t *testing.T) {
	out := output.New(&bytes.Buffer{}, &bytes.Buffer{}, output.Options{Plain: true})
	err := RunStep(context.Background(), paths.Root{}, out, "secrets", Options{DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "does not support --dry-run") {
		t.Fatalf("RunStep() error = %v, want unsupported dry-run", err)
	}
}

func TestAllDryRunSkipsUnsupportedSteps(t *testing.T) {
	var stdout bytes.Buffer
	out := output.New(&stdout, &bytes.Buffer{}, output.Options{Plain: true})
	ctx := &app.Context{Context: context.Background(), Root: paths.Root{}, Output: out}
	err := (&AllCmd{DryRun: true}).Run(ctx)
	if err == nil || !strings.Contains(err.Error(), "package list not found") {
		t.Fatalf("All dry-run error = %v, want first supported step to run", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Skipping install secrets") || !strings.Contains(got, "Skipping install repos") {
		t.Fatalf("All dry-run did not warn about unsupported steps:\n%s", got)
	}
}

func catalogHas(children []tui.CommandNode, name string) bool {
	for _, child := range children {
		if child.Name == name {
			return true
		}
	}
	return false
}

func TestPlanSymlinkRefusesUserDirectory(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	src := filepath.Join(repo, "config", "nvim")
	dst := filepath.Join(dir, "home", ".config", "nvim")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "init.lua"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanSymlink(src, dst, repo)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != LinkRefuse {
		t.Fatalf("expected refuse, got %s", plan.Action)
	}
}

func TestPlanSymlinkAllowsRepoManagedDirectoryBackup(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	src := filepath.Join(repo, "config", "nvim")
	managedTarget := filepath.Join(repo, "config", "old", "init.lua")
	dst := filepath.Join(dir, "home", ".config", "nvim")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(managedTarget), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedTarget, []byte("repo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managedTarget, filepath.Join(dst, "init.lua")); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanSymlink(src, dst, repo)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != LinkBackup {
		t.Fatalf("expected backup, got %s", plan.Action)
	}
}

func TestUpsertSwapFstab(t *testing.T) {
	base := "UUID=root / btrfs subvol=/@ 0 0\n"
	got := upsertSwapSubvolFstab(base, "abc", "/swap")
	want := base + "UUID=abc /swap btrfs subvol=/@swap,noatime 0 0\n"
	if got != want {
		t.Fatalf("subvol fstab mismatch:\n%s", got)
	}
	if again := upsertSwapSubvolFstab(got, "abc", "/swap"); again != got {
		t.Fatalf("subvol fstab was not idempotent:\n%s", again)
	}

	got = upsertSwapFileFstab(got, "/swap/swapfile")
	if !containsLine(got, "/swap/swapfile none swap defaults,pri=10 0 0") {
		t.Fatalf("swapfile fstab entry missing:\n%s", got)
	}
	if again := upsertSwapFileFstab(got, "/swap/swapfile"); again != got {
		t.Fatalf("swapfile fstab was not idempotent:\n%s", again)
	}
}

func TestUpdateLoaderResume(t *testing.T) {
	in := "title Arch\noptions root=UUID=root rw resume=UUID=old quiet resume_offset=12\n"
	got, err := updateLoaderResume(in, "new", "99")
	if err != nil {
		t.Fatal(err)
	}
	want := "title Arch\noptions root=UUID=root rw quiet resume=UUID=new resume_offset=99\n"
	if got != want {
		t.Fatalf("loader mismatch:\nwant %q\ngot  %q", want, got)
	}
}

func TestEnsureResumeHook(t *testing.T) {
	got, changed := ensureResumeHook("HOOKS=(base udev autodetect modconf block filesystems fsck)\n")
	if !changed {
		t.Fatal("expected change")
	}
	want := "HOOKS=(base udev autodetect modconf block filesystems resume fsck)\n"
	if got != want {
		t.Fatalf("hook mismatch:\nwant %q\ngot  %q", want, got)
	}
	if again, changed := ensureResumeHook(got); changed || again != got {
		t.Fatalf("resume hook was not idempotent")
	}
}

func TestConfigPlanningHelpers(t *testing.T) {
	if networkManagerDNSDropin() != "[main]\ndns=systemd-resolved\n" {
		t.Fatalf("unexpected NetworkManager DNS drop-in")
	}
}

func TestHealthForUnknownStepFails(t *testing.T) {
	checks := healthFor(t.Context(), paths.Root{}, "nope")
	if len(checks) != 1 || checks[0].Status != health.Fail {
		t.Fatalf("expected one failing check, got %#v", checks)
	}
}

func TestSecretsHealthSkipsMissingManifest(t *testing.T) {
	dir := t.TempDir()
	root := paths.Root{Dotfiles: filepath.Join(dir, "repo"), Home: filepath.Join(dir, "home")}
	if err := os.MkdirAll(root.Etc("secrets"), 0o755); err != nil {
		t.Fatal(err)
	}

	checks := secretsHealth(root)
	if len(checks) != 1 || checks[0].Status != health.Skip {
		t.Fatalf("expected missing manifest to skip, got %#v", checks)
	}
}

func containsLine(s, want string) bool {
	for _, line := range splitLines(s) {
		if line == want {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for idx, r := range s {
		if r == '\n' {
			lines = append(lines, s[start:idx])
			start = idx + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
