package iso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/ui"
)

type stick struct {
	Path        string   `json:"path"`
	Type        string   `json:"type"`
	Size        int64    `json:"size"`
	RO          bool     `json:"ro"`
	RM          bool     `json:"rm"`
	Tran        string   `json:"tran"`
	Model       string   `json:"model"`
	Serial      string   `json:"serial"`
	WWN         string   `json:"wwn"`
	Mountpoints []string `json:"mountpoints"`
	Children    []stick  `json:"children"`
}

type identity struct {
	Rdev   uint64
	Serial string
	WWN    string
	Size   int64
}

func (s stick) mounted() bool {
	for _, m := range s.Mountpoints {
		if m != "" {
			return true
		}
	}
	for _, c := range s.Children {
		if c.mounted() {
			return true
		}
	}
	return false
}

func (s stick) refusal(isoSize int64) string {
	switch {
	case s.Type != "disk":
		return "not a whole disk (" + s.Type + ")"
	case s.RO:
		return "read-only"
	case !s.RM && s.Tran != "usb":
		return "not removable or USB; refusing internal disks"
	case s.mounted():
		return "it or one of its partitions is mounted"
	case s.Size < isoSize:
		return fmt.Sprintf("smaller (%s) than the ISO (%s)", mib(s.Size), mib(isoSize))
	case strings.TrimSpace(s.Serial) == "" && s.WWN == "":
		return "no serial or WWN to pin the confirmation to"
	}
	return ""
}

func inspect(ctx context.Context, device string) (stick, error) {
	out, err := execx.OSRunner{}.Output(ctx, "", "lsblk", "-J", "-b", "-o", "PATH,TYPE,SIZE,RO,RM,TRAN,MODEL,SERIAL,WWN,MOUNTPOINTS", device)
	if err != nil {
		return stick{}, err
	}
	var tree struct {
		Devices []stick `json:"blockdevices"`
	}
	if err := json.Unmarshal([]byte(out), &tree); err != nil {
		return stick{}, fmt.Errorf("parse lsblk: %w", err)
	}
	if len(tree.Devices) != 1 {
		return stick{}, fmt.Errorf("lsblk returned %d devices for %s", len(tree.Devices), device)
	}
	return tree.Devices[0], nil
}

func identify(ctx context.Context, device string, st os.FileInfo) (identity, error) {
	dev, err := inspect(ctx, device)
	if err != nil {
		return identity{}, err
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok || st.Mode()&os.ModeDevice == 0 {
		return identity{}, fmt.Errorf("%s is not a block device", device)
	}
	return identity{sys.Rdev, strings.TrimSpace(dev.Serial), dev.WWN, dev.Size}, nil
}

func copySigned(dst io.Writer, src io.Reader, name, want string) error {
	got, err := digest(io.TeeReader(src, dst))
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("%s does not match its signed checksum", name)
	}
	return nil
}

func USB(ctx context.Context, u *ui.UI, root paths.Root, iso, device string) error {
	want, err := signedSum(ctx, root.Share("allowed_signers"), iso)
	if err != nil {
		return err
	}
	u.OK("checksum file signature verified")
	src, err := os.Open(iso)
	if err != nil {
		return err
	}
	defer src.Close()
	st, err := src.Stat()
	if err != nil {
		return err
	}
	dev, err := inspect(ctx, device)
	if err != nil {
		return err
	}
	if reason := dev.refusal(st.Size()); reason != "" {
		return fmt.Errorf("refusing %s: %s", device, reason)
	}
	node, err := os.Stat(dev.Path)
	if err != nil {
		return err
	}
	before, err := identify(ctx, dev.Path, node)
	if err != nil {
		return err
	}
	u.Warn("Writing %s erases %s %q serial=%s wwn=%s (%s, %s)", iso, dev.Path, strings.TrimSpace(dev.Model), before.Serial, before.WWN, mib(dev.Size), dev.Tran)
	typed, err := u.Text("Type "+dev.Path+" to erase it", "")
	if err != nil {
		return err
	}
	if typed != dev.Path {
		return errors.New("confirmation did not match; nothing written")
	}
	if err := copySigned(io.Discard, src, iso, want); err != nil {
		return fmt.Errorf("%w; nothing written", err)
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return err
	}
	out, err := os.OpenFile(dev.Path, os.O_WRONLY|os.O_EXCL, 0)
	if err != nil {
		return fmt.Errorf("open %s exclusively (mounted or in use?): %w", dev.Path, err)
	}
	defer out.Close()
	held, err := out.Stat()
	if err != nil {
		return err
	}
	after, err := identify(ctx, dev.Path, held)
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("%s changed after confirmation (%+v, now %+v); nothing written", dev.Path, before, after)
	}
	err = u.Spin(ctx, "Writing "+mib(st.Size())+" to "+dev.Path, func(context.Context) error {
		if err := copySigned(out, src, iso, want); err != nil {
			return fmt.Errorf("writing %s: %w; do not boot it", dev.Path, err)
		}
		return out.Sync()
	})
	if err != nil {
		return err
	}
	u.OK("wrote and verified %s on %s", iso, dev.Path)
	return nil
}
