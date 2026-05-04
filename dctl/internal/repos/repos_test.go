package repos

import (
	"os"
	"path/filepath"
	"testing"

	"dotfiles/dctl/internal/paths"
)

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "repos.toml")
	data := `# comment
[dotfiles]
repo = "cogikyo/dotfiles"
path = "~/dotfiles"

[vagari]
repo = "cogikyo/vagari" # inline comment
path = "~/vagari/vagari"

[cullyn.dev]
repo = "cogikyo/cullyn.dev"
path = "~/cogikyo/cullyn.dev"
`
	if err := os.WriteFile(manifest, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	repos, err := LoadManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}
	if repos[0].Name != "cullyn.dev" || repos[0].Repo != "cogikyo/cullyn.dev" || repos[0].Path != "~/cogikyo/cullyn.dev" {
		t.Fatalf("dotted section was not preserved literally: %#v", repos[0])
	}
	if repos[2].Name != "vagari" || repos[2].Repo != "cogikyo/vagari" || repos[2].Path != "~/vagari/vagari" {
		t.Fatalf("unexpected repo: %#v", repos[2])
	}
}

func TestExpandHome(t *testing.T) {
	if got := paths.ExpandHome("/home/cullyn", "~/dotfiles"); got != "/home/cullyn/dotfiles" {
		t.Fatalf("unexpected expanded path: %s", got)
	}
	if got := paths.ExpandHome("/home/cullyn", "/tmp/x"); got != "/tmp/x" {
		t.Fatalf("absolute path changed: %s", got)
	}
}
