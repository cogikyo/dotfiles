package secrets

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/dctl/paths"

	"golang.org/x/sys/unix"
)

var (
	errIrregular = errors.New("not a regular file")
	errCheckout  = errors.New("inside the dotfiles checkout")
)

var beneath = unix.OpenHow{
	Flags:   unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC,
	Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_BENEATH | unix.RESOLVE_NO_XDEV,
}

type id struct{ dev, ino uint64 }

func idOf(fd int) (id, error) {
	var st unix.Stat_t
	err := unix.Fstat(fd, &st)
	return id{st.Dev, st.Ino}, err
}

type Tree struct {
	*os.File
	fence *id
}

func openRepo(root paths.Root) (*Tree, error) {
	f, err := os.Open(root.Secrets())
	if err != nil {
		return nil, err
	}
	return &Tree{File: f}, nil
}

func OpenHome(root paths.Root) (*Tree, error) {
	var st unix.Stat_t
	if err := unix.Stat(root.Dotfiles, &st); err != nil {
		return nil, &fs.PathError{Op: "stat", Path: root.Dotfiles, Err: err}
	}
	checkout := id{st.Dev, st.Ino}
	f, err := os.OpenFile(root.Home, os.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	if err := outside(f, checkout); err != nil {
		f.Close()
		return nil, err
	}
	return &Tree{File: f, fence: &checkout}, nil
}

func outside(home *os.File, checkout id) error {
	cur, err := unix.Openat(int(home.Fd()), ".", unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer func() { unix.Close(cur) }()
	here, err := idOf(cur)
	if err != nil {
		return err
	}
	for here != checkout {
		up, err := unix.Openat(cur, "..", unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
		if err != nil {
			return err
		}
		unix.Close(cur)
		cur = up
		above, err := idOf(cur)
		if err != nil || above == here {
			return err
		}
		here = above
	}
	return fmt.Errorf("HOME %s is %w", home.Name(), errCheckout)
}

func (t *Tree) parent(rel string, create bool) (*os.File, error) {
	cur, err := unix.Openat2(int(t.Fd()), ".", &beneath)
	if err != nil {
		return nil, err
	}
	for name := range strings.SplitSeq(filepath.Dir(rel), "/") {
		if name == "." {
			break
		}
		next, err := unix.Openat2(cur, name, &beneath)
		if errors.Is(err, unix.ENOENT) && create {
			if err = unix.Mkdirat(cur, name, 0o700); err == nil || errors.Is(err, unix.EEXIST) {
				next, err = unix.Openat2(cur, name, &beneath)
			}
		}
		unix.Close(cur)
		switch {
		case errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR):
			return nil, fmt.Errorf("%s: parent %s is a symlink or not a directory: %w", rel, name, errIrregular)
		case errors.Is(err, unix.EXDEV):
			return nil, fmt.Errorf("%s: parent %s is a mount point; refusing to cross it", rel, name)
		case err != nil:
			return nil, &fs.PathError{Op: "open", Path: rel, Err: err}
		}
		cur = next
		if err := t.fenced(cur); err != nil {
			unix.Close(cur)
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
	}
	return os.NewFile(uintptr(cur), filepath.Dir(rel)), nil
}

func (t *Tree) fenced(fd int) error {
	if t.fence == nil {
		return nil
	}
	here, err := idOf(fd)
	if err == nil && here == *t.fence {
		err = errCheckout
	}
	return err
}

func (t *Tree) leaf(rel string, flags int) (*os.File, fs.FileInfo, error) {
	p, err := t.parent(rel, false)
	if err != nil {
		return nil, nil, err
	}
	defer p.Close()
	fd, err := unix.Openat(int(p.Fd()), filepath.Base(rel), flags|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ELOOP) {
		return nil, nil, errIrregular
	}
	if err != nil {
		return nil, nil, &fs.PathError{Op: "open", Path: rel, Err: err}
	}
	f := os.NewFile(uintptr(fd), rel)
	st, err := f.Stat()
	if err == nil && !st.Mode().IsRegular() {
		err = errIrregular
	}
	if err != nil {
		f.Close()
		return nil, st, err
	}
	return f, st, nil
}

func (t *Tree) stat(rel string) (fs.FileInfo, error) {
	f, st, err := t.leaf(rel, unix.O_PATH)
	if err == nil {
		f.Close()
	}
	return st, err
}

func (t *Tree) read(rel string) ([]byte, fs.FileInfo, error) {
	f, st, err := t.leaf(rel, unix.O_RDONLY|unix.O_NONBLOCK)
	if err != nil {
		return nil, st, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	return data, st, err
}

func (t *Tree) chmod(rel string, mode fs.FileMode) error {
	f, _, err := t.leaf(rel, unix.O_RDONLY|unix.O_NONBLOCK)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Chmod(mode)
}

func (t *Tree) write(rel string, data []byte, mode fs.FileMode) error {
	p, err := t.parent(rel, true)
	if err != nil {
		return err
	}
	defer p.Close()
	pfd, base := int(p.Fd()), filepath.Base(rel)
	var st unix.Stat_t
	switch err := unix.Fstatat(pfd, base, &st, unix.AT_SYMLINK_NOFOLLOW); {
	case errors.Is(err, unix.ENOENT):
	case err != nil:
		return err
	case st.Mode&unix.S_IFMT != unix.S_IFREG:
		return fmt.Errorf("refusing to replace %s: %w", rel, errIrregular)
	}
	tmp := "." + base + ".dctl-" + rand.Text()[:8]
	fd, err := unix.Openat(pfd, tmp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), tmp)
	_, err = f.Write(data)
	err = errors.Join(err, f.Chmod(mode), f.Sync(), f.Close())
	if err == nil {
		err = unix.Renameat(pfd, tmp, pfd, base)
	}
	if err != nil {
		_ = unix.Unlinkat(pfd, tmp, 0)
		return err
	}
	return unix.Fsync(pfd)
}

var ErrCommitted = errors.New("secrets/ replaced")

func OpenCheckout(root paths.Root) (*Tree, error) {
	f, err := os.OpenFile(root.Dotfiles, os.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	return &Tree{File: f}, nil
}

var errBusy = errors.New("another dctl secrets operation is running")

func Locked(root paths.Root, fn func() error) error {
	checkout, err := OpenCheckout(root)
	if err != nil {
		return err
	}
	defer checkout.Close()
	err = unix.Flock(int(checkout.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return errBusy
	}
	if err != nil {
		return &fs.PathError{Op: "flock", Path: root.Dotfiles, Err: err}
	}
	return fn()
}

func (t *Tree) ReadFile(rel string) ([]byte, error) {
	data, _, err := t.read(rel)
	return data, err
}

func (t *Tree) WriteFile(rel string, data []byte, mode fs.FileMode) error {
	return t.write(rel, data, mode)
}

func Preflight(root paths.Root) error {
	entries, err := Manifest(root)
	if err != nil {
		return err
	}
	managed := map[string][]byte{"manifest": nil, "recipients": nil, "identities": nil, "identity.age": nil}
	for _, e := range entries {
		managed[e.Name+".age"] = nil
	}
	_, err = inventory(root.Secrets(), managed)
	return err
}

func inventory(dir string, files map[string][]byte) (map[string]fs.FileMode, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	modes := map[string]fs.FileMode{}
	for _, e := range entries {
		if _, ok := files[e.Name()]; !ok {
			return nil, fmt.Errorf("%s unchanged: %s is not managed by dctl; move it out first", dir, e.Name())
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s unchanged: %s: %w", dir, e.Name(), errIrregular)
		}
		modes[e.Name()] = info.Mode().Perm()
	}
	return modes, nil
}

func swap(dir string, files map[string][]byte) error {
	st, err := os.Stat(dir)
	if err != nil {
		return err
	}
	modes, err := inventory(dir, files)
	if err != nil {
		return err
	}
	parent, err := os.Open(filepath.Dir(dir))
	if err != nil {
		return err
	}
	defer parent.Close()
	pfd, name := int(parent.Fd()), filepath.Base(dir)
	stage := "." + name + ".dctl-" + rand.Text()[:8]
	staged := filepath.Join(filepath.Dir(dir), stage)
	if err := unix.Mkdirat(pfd, stage, 0o700); err != nil {
		return fmt.Errorf("%s unchanged: %w", dir, err)
	}
	err = fill(staged, st.Mode().Perm(), files, modes)
	if err == nil {
		err = unix.Renameat2(pfd, stage, pfd, name, unix.RENAME_EXCHANGE)
	}
	if err != nil {
		return fmt.Errorf("%s unchanged: %w", dir, errors.Join(err, os.RemoveAll(staged)))
	}
	if err := unix.Fsync(pfd); err != nil {
		return fmt.Errorf("%w, but the swap may not survive a crash; the old copy is at %s: %w", ErrCommitted, staged, err)
	}
	if err := os.RemoveAll(staged); err != nil {
		return fmt.Errorf("%w; the old copy is left at %s: %w", ErrCommitted, staged, err)
	}
	return nil
}

func fill(dst string, perm fs.FileMode, files map[string][]byte, modes map[string]fs.FileMode) error {
	d, err := os.Open(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	for name, data := range files {
		mode, ok := modes[name]
		if !ok {
			mode = 0o644
		}
		if err := create(d, name, data, mode); err != nil {
			return err
		}
	}
	return errors.Join(d.Chmod(perm), d.Sync())
}

func create(dir *os.File, name string, data []byte, mode fs.FileMode) error {
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), name)
	_, err = f.Write(data)
	return errors.Join(err, f.Chmod(mode), f.Sync(), f.Close())
}
