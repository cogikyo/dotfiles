package installer

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func golden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs (run go test -update to accept):\n--- got\n%s--- want\n%s", path, got, want)
	}
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "lsblk", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestScan(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		boot    string
		picks   []string
	}{
		{"nvme", "nvme", "/dev/sda1", []string{"", "/dev/sda", "/dev/loop0"}},
		{"by-uuid", "nvme", "/dev/disk/by-uuid/2026-10-01-12-00-00-00", []string{""}},
		{"no-boot", "nvme", "", nil},
		{"isohybrid", "isohybrid", "/dev/disk/by-uuid/2026-10-01-12-00-00-00", []string{"", "/dev/sda"}},
		{"duplicate", "duplicate", "/dev/disk/by-uuid/2026-10-01-12-00-00-00", nil},
		{"unmatched", "nvme", "/dev/disk/by-id/usb-SanDisk_Extreme_Pro-0:0-part1", nil},
		{"sector4k", "sector4k", "/dev/sr0", []string{"", "/dev/vdb"}},
		{"cdrom", "cdrom", "/dev/disk/by-uuid/2026-10-05-10-13-34-00", []string{"", "/dev/sr0", "/dev/loop1"}},
		{"ambiguous", "ambiguous", "/dev/sda1", []string{"", "/dev/nvme1n1", "/dev/zram0"}},
		{"mounted", "mounted", "/dev/sda1", []string{"", "/dev/nvme0n1", "/dev/sdb", "/dev/sdz"}},
		{"small", "small", "/dev/disk/by-label/ARCH_202610", []string{"", "/dev/mmcblk0", "/dev/sdc", "/dev/sdd"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			fmt.Fprintf(&b, "boot %q\n", tt.boot)
			path := filepath.Join("testdata", "lsblk", tt.name+".golden")
			s, err := scan(fixture(t, tt.fixture), tt.boot)
			if err != nil {
				fmt.Fprintf(&b, "scan error: %v\n", err)
				golden(t, path, []byte(b.String()))
				return
			}
			for _, d := range s.Candidates {
				fmt.Fprintf(&b, "candidate %s parts=%s,%s\n", d, d.part(1), d.part(2))
			}
			for _, r := range s.Refused {
				fmt.Fprintf(&b, "refused %s: %s\n", r.Disk, r.Reason)
			}
			for _, p := range tt.picks {
				d, err := s.pick(p)
				if err != nil {
					fmt.Fprintf(&b, "pick %q: error: %v\n", p, err)
					continue
				}
				fmt.Fprintf(&b, "pick %q: %s\n", p, d.Path)
			}
			golden(t, path, []byte(b.String()))
		})
	}
}

func TestRecheck(t *testing.T) {
	lsblk := fixture(t, "nvme")
	s, err := scan(lsblk, "/dev/sda1")
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.pick("")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.recheck(lsblk, "/dev/sda1"); err != nil {
		t.Errorf("unchanged disk: %v", err)
	}
	tests := []struct {
		name  string
		lsblk []byte
		boot  string
	}{
		{"mounted", fixture(t, "mounted"), "/dev/sda1"},
		{"different disk", fixture(t, "small"), "/dev/sda1"},
		{"changed serial", bytes.ReplaceAll(lsblk, []byte(`"24123A800123"`), []byte(`"24123A899999"`)), "/dev/sda1"},
		{"no boot", lsblk, ""},
		{"ambiguous boot", fixture(t, "duplicate"), "/dev/disk/by-uuid/2026-10-01-12-00-00-00"},
	}
	for _, tt := range tests {
		if err := d.recheck(tt.lsblk, tt.boot); err == nil {
			t.Errorf("%s: recheck passed", tt.name)
		}
	}
}
