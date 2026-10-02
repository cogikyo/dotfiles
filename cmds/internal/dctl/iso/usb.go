package iso

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/paths"
	"dotfiles/cmds/internal/dctl/ui"
)

type USBCmd struct {
	ISO    string `arg:"" type:"existingfile" help:"ISO with .sha256 and .sha256.sig beside it."`
	Device string `arg:"" help:"Whole removable disk, e.g. /dev/sdX."`
}

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

func check(src io.ReadSeeker, want string) error {
	h := sha256.New()
	if _, err := io.Copy(h, src); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("ISO sha256 %s does not match the signed %s; nothing written", got, want)
	}
	_, err := src.Seek(0, io.SeekStart)
	return err
}

func copySigned(dst io.Writer, src io.Reader, want string) error {
	h := sha256.New()
	if _, err := io.Copy(dst, io.TeeReader(src, h)); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("the ISO changed while writing (sha256 %s, signed %s); the device is untrusted, do not boot it", got, want)
	}
	return nil
}

func (c USBCmd) Run(ctx context.Context, u *ui.UI, root paths.Root) error {
	want, err := signedSum(ctx, root.Share("allowed_signers"), c.ISO)
	if err != nil {
		return err
	}
	u.OK("checksum file signature verified")
	src, err := os.Open(c.ISO)
	if err != nil {
		return err
	}
	defer src.Close()
	st, err := src.Stat()
	if err != nil {
		return err
	}
	dev, err := inspect(ctx, c.Device)
	if err != nil {
		return err
	}
	if reason := dev.refusal(st.Size()); reason != "" {
		return fmt.Errorf("refusing %s: %s", c.Device, reason)
	}
	node, err := os.Stat(dev.Path)
	if err != nil {
		return err
	}
	before, err := identify(ctx, dev.Path, node)
	if err != nil {
		return err
	}
	u.Warn("Writing %s erases %s %q serial=%s wwn=%s (%s, %s)", c.ISO, dev.Path, strings.TrimSpace(dev.Model), before.Serial, before.WWN, mib(dev.Size), dev.Tran)
	typed, err := u.Text("Type "+dev.Path+" to erase it", "")
	if err != nil {
		return err
	}
	if typed != dev.Path {
		return errors.New("confirmation did not match; nothing written")
	}
	if err := check(src, want); err != nil {
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
		if err := copySigned(out, src, want); err != nil {
			return err
		}
		return out.Sync()
	})
	if err != nil {
		return err
	}
	u.OK("wrote and verified %s on %s", c.ISO, dev.Path)
	return nil
}
