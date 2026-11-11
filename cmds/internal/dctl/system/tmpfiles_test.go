package system

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func tmpfilesOverlay(t *testing.T, conf string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, tmpfilesDir, "app.conf")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestParseTmpfiles(t *testing.T) {
	got, err := parseTmpfiles(strings.NewReader("# lock\n\nf /run/app.lock 0644 root root -\nd /home/u/.cache :0755 u u -\ne /home/u/.cache - - - 3d\nx /home/u/.cache/keep\n"))
	want := []tmpEntry{{'f', "/run/app.lock"}, {'d', "/home/u/.cache"}}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("got %v, %v; want %v", got, err, want)
	}
	for _, bad := range []string{"f", "L /run/a - - - - /b", "f+ /run/a", "f run/a", "d /home/%u", "d /run/*"} {
		if _, err := parseTmpfiles(strings.NewReader(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestCheckTmpfilesReportsOnlyMissing(t *testing.T) {
	src, root := tmpfilesOverlay(t, "f /run/app.lock 0644 root root -\nd /var/lib/app 0755 root root -\n"), t.TempDir()
	err := checkTmpfiles(root, src)
	if err == nil || !strings.Contains(err.Error(), "2 of 2 tmpfiles paths missing") {
		t.Fatalf("empty root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "run"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "run", "app.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	err = checkTmpfiles(root, src)
	if err == nil || !strings.Contains(err.Error(), "1 of 2 tmpfiles paths missing") || strings.Contains(err.Error(), "app.lock") {
		t.Fatalf("lock present: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "var", "lib", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := checkTmpfiles(root, src); err != nil {
		t.Fatalf("all present: %v", err)
	}
}

func TestCheckTmpfilesRejectsWrongKind(t *testing.T) {
	src, root := tmpfilesOverlay(t, "f /run/app.lock 0644 root root -\n"), t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "run", "app.lock"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := checkTmpfiles(root, src); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("directory at file path: %v", err)
	}
}

func TestCreateTmpfilesRunsOverlayConfigs(t *testing.T) {
	src, s := tmpfilesOverlay(t, "f /run/app.lock 0644 root root -\n"), &systemctl{}
	if err := createTmpfiles(t.Context(), s, "/mnt", src); err != nil {
		t.Fatal(err)
	}
	want := []string{"systemd-tmpfiles --create --root=/mnt /etc/tmpfiles.d/app.conf"}
	if !slices.Equal(s.calls, want) {
		t.Errorf("calls %q, want %q", s.calls, want)
	}
	empty := &systemctl{}
	if err := createTmpfiles(t.Context(), empty, "/", t.TempDir()); err != nil || len(empty.calls) != 0 {
		t.Errorf("empty overlay: calls %q, err %v", empty.calls, err)
	}
}

func TestRepositoryTmpfilesParse(t *testing.T) {
	confs, entries, err := tmpfiles(filepath.Join("..", "..", "..", "..", "system"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(confs, "/etc/tmpfiles.d/vpn-keep.conf") || !slices.Contains(entries, tmpEntry{'f', "/run/vpn-keep.lock"}) {
		t.Errorf("confs %q, entries %v; want the vpn-keep lock", confs, entries)
	}
}
