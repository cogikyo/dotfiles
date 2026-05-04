package iso

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParsePackageList(t *testing.T) {
	got := parsePackageList(strings.NewReader(`
# comment
git
linux # inline
git duplicate-field

  base-devel extra ignored
`))
	want := []string{"base-devel", "git", "linux"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("parsePackageList() = %#v, want %#v", got, want)
	}
}

func TestShouldExcludeProfileRel(t *testing.T) {
	cases := map[string]bool{
		".git":                                 true,
		".git/config":                          true,
		"iso/work":                             true,
		"iso/work/rootfs":                      true,
		"iso/out/latest.iso":                   true,
		"iso/airootfs/var/cache/localrepo/pkg": true,
		"iso/airootfs/root/dotfiles/config":    true,
		"share/videos/clip.mp4":                true,
		"share/videos/clip.webm":               false,
		"daemons/newtab/dna.webm":              true,
		"etc/fonts.tar.gz":                     true,
		"config/hypr/hyprland.conf":            false,
	}
	for rel, want := range cases {
		if got := shouldExcludeProfileRel(rel); got != want {
			t.Fatalf("shouldExcludeProfileRel(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestSelectNewestISO(t *testing.T) {
	base := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	got, err := selectNewestISO([]isoFile{
		{Path: "old.iso", ModTime: base},
		{Path: "new.iso", ModTime: base.Add(time.Second)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "new.iso" {
		t.Fatalf("selectNewestISO() = %q, want new.iso", got)
	}
}

func TestValidateUSBTarget(t *testing.T) {
	dev := blockDevice{Path: "/dev/sdz", Type: "disk", Tran: "usb", Size: json.Number("4096")}
	if err := validateUSBTarget(dev, 1024); err != nil {
		t.Fatalf("validateUSBTarget(valid) error = %v", err)
	}

	cases := []struct {
		name string
		dev  blockDevice
		iso  int64
	}{
		{name: "partition", dev: blockDevice{Path: "/dev/sdz1", Type: "part", Tran: "usb", Size: json.Number("4096")}, iso: 1024},
		{name: "non usb", dev: blockDevice{Path: "/dev/nvme0n1", Type: "disk", Tran: "nvme", Size: json.Number("4096")}, iso: 1024},
		{name: "mounted child", dev: blockDevice{Path: "/dev/sdz", Type: "disk", Tran: "usb", Size: json.Number("4096"), Children: []blockDevice{{Mountpoints: []string{"/run/media/usb"}}}}, iso: 1024},
		{name: "too small", dev: blockDevice{Path: "/dev/sdz", Type: "disk", Tran: "usb", Size: json.Number("512")}, iso: 1024},
	}
	for _, tt := range cases {
		if err := validateUSBTarget(tt.dev, tt.iso); err == nil {
			t.Fatalf("validateUSBTarget(%s) succeeded, want error", tt.name)
		}
	}
}

func TestIsPackageArchive(t *testing.T) {
	cases := map[string]bool{
		"foo-1-1-x86_64.pkg.tar.zst":     true,
		"foo-1-1-x86_64.pkg.tar.zst.sig": false,
		"localrepo.db.tar.zst":           false,
	}
	for name, want := range cases {
		if got := isPackageArchive(name); got != want {
			t.Fatalf("isPackageArchive(%q) = %v, want %v", name, got, want)
		}
	}
}
