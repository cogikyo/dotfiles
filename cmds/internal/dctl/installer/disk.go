package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

const minSize int64 = 32 << 30

var errAmbiguous = errors.New("more than one installable disk")

type disk struct {
	Path   string
	Model  string
	Serial string
	WWN    string
	Tran   string
	Size   int64
	Sector int
}

func (d disk) part(n int) string {
	if last := d.Path[len(d.Path)-1]; last >= '0' && last <= '9' {
		return fmt.Sprintf("%sp%d", d.Path, n)
	}
	return fmt.Sprintf("%s%d", d.Path, n)
}

func (d disk) String() string {
	return fmt.Sprintf("%s %q serial=%s wwn=%s tran=%s size=%d sector=%d", d.Path, d.Model, d.Serial, d.WWN, d.Tran, d.Size, d.Sector)
}

type refusal struct {
	Disk   disk
	Reason string
}

type survey struct {
	Candidates []disk
	Refused    []refusal
}

type blockdev struct {
	Name        string     `json:"name"`
	Path        string     `json:"path"`
	Type        string     `json:"type"`
	Size        int64      `json:"size"`
	RO          bool       `json:"ro"`
	RM          bool       `json:"rm"`
	Tran        string     `json:"tran"`
	Model       string     `json:"model"`
	Serial      string     `json:"serial"`
	WWN         string     `json:"wwn"`
	Sector      int        `json:"log-sec"`
	UUID        string     `json:"uuid"`
	PartUUID    string     `json:"partuuid"`
	Label       string     `json:"label"`
	Mountpoints []string   `json:"mountpoints"`
	Children    []blockdev `json:"children"`
}

func (b blockdev) is(source string) bool {
	alias := func(kind, id string) bool { return id != "" && source == "/dev/disk/"+kind+"/"+id }
	return source == b.Path || alias("by-uuid", b.UUID) || alias("by-partuuid", b.PartUUID) || alias("by-label", b.Label)
}

func (b blockdev) walk(visit func(blockdev) bool) bool {
	if !visit(b) {
		return false
	}
	for _, c := range b.Children {
		if !c.walk(visit) {
			return false
		}
	}
	return true
}

func (b blockdev) mounts() []string {
	var out []string
	b.walk(func(dev blockdev) bool {
		for _, m := range dev.Mountpoints {
			if m != "" {
				out = append(out, m)
			}
		}
		return true
	})
	return out
}

func (b blockdev) holds(source string) bool {
	return !b.walk(func(dev blockdev) bool { return !dev.is(source) })
}

func (b blockdev) refusal(boot string) string {
	switch {
	case b.Path == boot:
		return "boot medium"
	case b.Type != "disk":
		return "not a whole disk (" + b.Type + ")"
	case strings.HasPrefix(b.Name, "zram"):
		return "compressed RAM device"
	case b.RO:
		return "read-only"
	case b.RM || b.Tran == "usb":
		return "removable or USB"
	}
	if m := b.mounts(); len(m) > 0 {
		return "mounted at " + strings.Join(m, ", ")
	}
	switch {
	case b.Size < minSize:
		return fmt.Sprintf("smaller than %d GiB", minSize>>30)
	case strings.TrimSpace(b.Serial) == "" && b.WWN == "":
		return "no serial or WWN to verify identity"
	case b.Sector != 512 && b.Sector != 4096:
		return fmt.Sprintf("unsupported logical sector size %d", b.Sector)
	}
	return ""
}

func scan(lsblk []byte, boot string) (survey, error) {
	var out struct {
		Devices []blockdev `json:"blockdevices"`
	}
	if err := json.Unmarshal(lsblk, &out); err != nil {
		return survey{}, fmt.Errorf("parse lsblk: %w", err)
	}
	if boot == "" {
		return survey{}, errors.New("boot medium not identified: no boot source given")
	}
	var owners []string
	for _, b := range out.Devices {
		if b.Type != "loop" && b.holds(boot) {
			owners = append(owners, b.Path)
		}
	}
	switch len(owners) {
	case 0:
		return survey{}, fmt.Errorf("boot medium not identified: %s matches no block device", boot)
	case 1:
	default:
		return survey{}, fmt.Errorf("boot medium ambiguous: %s matches %s", boot, strings.Join(owners, ", "))
	}
	medium := owners[0]
	var s survey
	for _, b := range out.Devices {
		if b.Path == "" {
			return survey{}, fmt.Errorf("lsblk: device %q has no path", b.Name)
		}
		d := disk{
			Path:   b.Path,
			Model:  strings.TrimSpace(b.Model),
			Serial: strings.TrimSpace(b.Serial),
			WWN:    b.WWN,
			Tran:   b.Tran,
			Size:   b.Size,
			Sector: b.Sector,
		}
		if reason := b.refusal(medium); reason != "" {
			s.Refused = append(s.Refused, refusal{d, reason})
			continue
		}
		s.Candidates = append(s.Candidates, d)
	}
	return s, nil
}

func (s survey) pick(path string) (disk, error) {
	if path != "" {
		if i := slices.IndexFunc(s.Candidates, func(d disk) bool { return d.Path == path }); i >= 0 {
			return s.Candidates[i], nil
		}
		if i := slices.IndexFunc(s.Refused, func(r refusal) bool { return r.Disk.Path == path }); i >= 0 {
			return disk{}, fmt.Errorf("refusing %s: %s", path, s.Refused[i].Reason)
		}
		return disk{}, fmt.Errorf("no disk %s", path)
	}
	switch len(s.Candidates) {
	case 0:
		return disk{}, errors.New("no installable disk")
	case 1:
		return s.Candidates[0], nil
	}
	paths := make([]string, len(s.Candidates))
	for i, d := range s.Candidates {
		paths[i] = d.Path
	}
	return disk{}, fmt.Errorf("%w: %s", errAmbiguous, strings.Join(paths, ", "))
}

func (d disk) recheck(lsblk []byte, boot string) error {
	s, err := scan(lsblk, boot)
	if err != nil {
		return err
	}
	now, err := s.pick(d.Path)
	if err != nil {
		return err
	}
	if now != d {
		return fmt.Errorf("disk %s changed: was %s, now %s", d.Path, d, now)
	}
	return nil
}
