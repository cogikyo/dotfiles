package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	minABI      = 9
	resolveUnix = 1 << 16
	inner       = "cage-confine"

	handled = resolveUnix<<1 - 1
	read    = unix.LANDLOCK_ACCESS_FS_READ_FILE | unix.LANDLOCK_ACCESS_FS_READ_DIR | unix.LANDLOCK_ACCESS_FS_EXECUTE
	write   = unix.LANDLOCK_ACCESS_FS_MAKE_REG | unix.LANDLOCK_ACCESS_FS_MAKE_DIR | unix.LANDLOCK_ACCESS_FS_WRITE_FILE
	churn   = write | unix.LANDLOCK_ACCESS_FS_REMOVE_FILE | unix.LANDLOCK_ACCESS_FS_REMOVE_DIR |
		unix.LANDLOCK_ACCESS_FS_REFER | unix.LANDLOCK_ACCESS_FS_TRUNCATE | unix.LANDLOCK_ACCESS_FS_MAKE_SYM
	device = unix.LANDLOCK_ACCESS_FS_READ_FILE | unix.LANDLOCK_ACCESS_FS_WRITE_FILE
	file   = unix.LANDLOCK_ACCESS_FS_EXECUTE | unix.LANDLOCK_ACCESS_FS_WRITE_FILE | unix.LANDLOCK_ACCESS_FS_READ_FILE |
		unix.LANDLOCK_ACCESS_FS_TRUNCATE | unix.LANDLOCK_ACCESS_FS_IOCTL_DEV
)

var (
	churned = []string{"tmp", "cache"}
	devices = map[string]uint64{
		"/dev/null":    device,
		"/dev/zero":    device,
		"/dev/full":    device,
		"/dev/random":  unix.LANDLOCK_ACCESS_FS_READ_FILE,
		"/dev/urandom": unix.LANDLOCK_ACCESS_FS_READ_FILE,
	}
)

type ruleset int

func main() {
	var hides []string
	flags := flag.NewFlagSet("cage", flag.ContinueOnError)
	flags.Func("hide", "deny reads beneath `path`", func(p string) error {
		hides = append(hides, p)
		return nil
	})
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: cage [--hide path]... <scratch-dir> -- <command> [args...]")
	}
	if err := unix.CloseRange(3, ^uint(0), unix.CLOSE_RANGE_CLOEXEC); err != nil {
		fmt.Fprintf(os.Stderr, "cage: close inherited files: %v\n", err)
		os.Exit(1)
	}
	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	args := flags.Args()
	if len(args) < 3 || args[1] != "--" {
		flags.Usage()
		os.Exit(2)
	}
	var err error
	if os.Args[0] == inner {
		err = confine(hides, args[0], args[2:])
	} else {
		err = spawn()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "cage: %v\n", err)
		os.Exit(1)
	}
}

func spawn() error {
	var alive [2]int
	if err := unix.Pipe2(alive[:], unix.O_CLOEXEC); err != nil {
		return err
	}
	cmd := exec.Command("/proc/self/exe")
	cmd.Args = append([]string{inner}, os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.ExtraFiles = []*os.File{os.NewFile(uintptr(alive[0]), "alive")}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWPID | syscall.CLONE_NEWIPC,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: os.Getuid(), HostID: os.Getuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: os.Getgid(), HostID: os.Getgid(), Size: 1}},
		AmbientCaps: []uintptr{unix.CAP_SYS_ADMIN},
	}
	if err := start(cmd); err != nil {
		return err
	}
	if err := cmd.Wait(); cmd.ProcessState == nil {
		return err
	}
	exit(cmd.ProcessState.Sys().(syscall.WaitStatus))
	return nil
}

func confine(hides []string, arg string, argv []string) error {
	if os.Getpid() != 1 {
		return errors.New(inner + " must run as PID 1")
	}
	go func() {
		os.NewFile(3, "alive").Read(make([]byte, 1))
		os.Exit(1)
	}()
	abi, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	if errno != 0 {
		return fmt.Errorf("landlock unavailable: %w", errno)
	}
	if abi < minABI {
		return fmt.Errorf("landlock ABI %d is older than %d", abi, minABI)
	}
	if !filepath.IsAbs(arg) || filepath.Clean(arg) != arg {
		return fmt.Errorf("scratch %q is not a clean absolute path", arg)
	}
	for _, dir := range churned {
		if err := os.MkdirAll(filepath.Join(arg, dir), 0o700); err != nil {
			return err
		}
	}
	scratch, err := filepath.EvalSymlinks(arg)
	if err != nil {
		return err
	}
	if scratch == "/" {
		return errors.New("scratch resolves to /")
	}
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("make mounts private: %w", err)
	}
	if err := unix.Mount("proc", "/proc", "proc", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, ""); err != nil {
		return fmt.Errorf("mount /proc: %w", err)
	}
	tree, err := unix.OpenTree(unix.AT_FDCWD, scratch, unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC|unix.AT_RECURSIVE)
	if err != nil {
		return fmt.Errorf("clone scratch mount: %w", err)
	}
	if err := unix.MountSetattr(unix.AT_FDCWD, "/", unix.AT_RECURSIVE, &unix.MountAttr{Attr_set: unix.MOUNT_ATTR_RDONLY}); err != nil {
		return fmt.Errorf("make mounts read-only: %w", err)
	}
	if err := unix.MoveMount(tree, "", unix.AT_FDCWD, scratch, unix.MOVE_MOUNT_F_EMPTY_PATH); err != nil {
		return fmt.Errorf("mount scratch: %w", err)
	}
	unix.Close(tree)
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		return err
	}

	attr := unix.LandlockRulesetAttr{
		Access_fs:  handled,
		Access_net: unix.LANDLOCK_ACCESS_NET_BIND_TCP | unix.LANDLOCK_ACCESS_NET_CONNECT_TCP,
		Scoped:     unix.LANDLOCK_SCOPE_ABSTRACT_UNIX_SOCKET | unix.LANDLOCK_SCOPE_SIGNAL,
	}
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr), 0)
	if errno != 0 {
		return fmt.Errorf("create ruleset: %w", errno)
	}
	r := ruleset(fd)
	if err := r.tree(append(hides, "/dev")); err != nil {
		return err
	}
	for dev, access := range devices {
		if err := r.path(dev, access); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if err := r.path(scratch, read|write); err != nil {
		return err
	}
	for _, dir := range churned {
		if err := r.path(filepath.Join(scratch, dir), read|churn); err != nil {
			return err
		}
	}

	caps := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var none [2]unix.CapUserData
	if _, _, errno := syscall.AllThreadsSyscall(unix.SYS_CAPSET, uintptr(unsafe.Pointer(&caps)), uintptr(unsafe.Pointer(&none[0])), 0); errno != 0 {
		return fmt.Errorf("drop capabilities: %w", errno)
	}
	if _, _, errno := syscall.AllThreadsSyscall(unix.SYS_PRCTL, unix.PR_SET_NO_NEW_PRIVS, 1, 0); errno != 0 {
		return fmt.Errorf("no_new_privs: %w", errno)
	}
	if _, _, errno := unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, fd, unix.LANDLOCK_RESTRICT_SELF_TSYNC, 0); errno != 0 {
		return fmt.Errorf("restrict: %w", errno)
	}
	os.Setenv("TMPDIR", filepath.Join(scratch, "tmp"))
	os.Setenv("XDG_CACHE_HOME", filepath.Join(scratch, "cache"))
	os.Setenv("OPENCODE_SCRATCH", scratch)
	if err := unix.Setrlimit(unix.RLIMIT_CORE, &unix.Rlimit{}); err != nil {
		return fmt.Errorf("disable core dumps: %w", err)
	}
	cmd := &exec.Cmd{Path: bin, Args: argv, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	if err := start(cmd); err != nil {
		return err
	}
	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, 0, nil)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if pid == cmd.Process.Pid {
			exit(status)
		}
	}
}

func start(cmd *exec.Cmd) error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		for sig := range signals {
			cmd.Process.Signal(sig)
		}
	}()
	return nil
}

func exit(status syscall.WaitStatus) {
	if status.Signaled() {
		os.Exit(128 + int(status.Signal()))
	}
	os.Exit(status.ExitStatus())
}

// tree grants reads beside hidden branches because Landlock rules can only add access.
// The root stays unlistable; hidden entry names remain visible in listed ancestors, and new ancestor entries stay unreadable.
func (r ruleset) tree(hides []string) error {
	holes := map[string]bool{}
	for _, hide := range hides {
		abs, err := filepath.Abs(hide)
		if err != nil {
			return err
		}
		real, err := filepath.EvalSymlinks(abs)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		holes[real] = true
	}
	for hole := range holes {
		for p := hole; p != "/"; {
			p = filepath.Dir(p)
			if holes[p] {
				delete(holes, hole)
				break
			}
		}
	}
	parents := map[string]bool{}
	for hole := range holes {
		for p := hole; p != "/"; {
			p = filepath.Dir(p)
			parents[p] = true
		}
	}
	for parent := range parents {
		if parent != "/" {
			if err := r.path(parent, unix.LANDLOCK_ACCESS_FS_READ_DIR); err != nil {
				return err
			}
		}
		entries, err := os.ReadDir(parent)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			child := filepath.Join(parent, entry.Name())
			if holes[child] || parents[child] || entry.Type()&fs.ModeSymlink != 0 {
				continue
			}
			if err := r.path(child, read); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

func (r ruleset) path(path string, access uint64) error {
	fd, err := unix.Openat2(unix.AT_FDCWD, path, &unix.OpenHow{Flags: unix.O_PATH | unix.O_CLOEXEC, Resolve: unix.RESOLVE_NO_SYMLINKS})
	if err != nil {
		return &fs.PathError{Op: "open", Path: path, Err: err}
	}
	defer unix.Close(fd)
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return &fs.PathError{Op: "stat", Path: path, Err: err}
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		access &= file
	}
	rule := unix.LandlockPathBeneathAttr{Allowed_access: access, Parent_fd: int32(fd)}
	if err := r.add(unix.LANDLOCK_RULE_PATH_BENEATH, unsafe.Pointer(&rule)); err != nil {
		return &fs.PathError{Op: "allow", Path: path, Err: err}
	}
	return nil
}

func (r ruleset) add(kind int, rule unsafe.Pointer) error {
	if _, _, errno := unix.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, uintptr(r), uintptr(kind), uintptr(rule), 0, 0, 0); errno != 0 {
		return errno
	}
	return nil
}
