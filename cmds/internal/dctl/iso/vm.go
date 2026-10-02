package iso

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"dotfiles/cmds/internal/dctl/doctor"
	"dotfiles/cmds/internal/dctl/execx"
	"dotfiles/cmds/internal/dctl/ui"
)

type TestCmd struct {
	ISO    string `arg:"" type:"existingfile" help:"ISO from dctl iso build."`
	Dctl   string `type:"existingfile" help:"dctl binary that replaces the ISO's."`
	Bundle string `type:"existingfile" help:"git bundle that replaces the ISO's."`
	Keep   bool   `help:"Keep the VM disk and firmware variables."`
}

const (
	ovmfCode = "/usr/share/edk2/x64/OVMF_CODE.secboot.4m.fd"
	ovmfVars = "/usr/share/edk2/x64/OVMF_VARS.4m.fd"

	luks   = "dctlunlock"
	serial = "DCTLTEST0"
	Label  = "DCTLTEST"

	installPrefix = `{"dctltest":`
	doctorPrefix  = `[{"group":`
)

var required = []string{"system", "packages", "home", "secureboot-keys"}

type Answers struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Zone     string `json:"zone"`
	LUKS     string `json:"luks"`
	Serial   string `json:"disk_serial"`
}

type Phase struct {
	Name    string  `json:"name"`
	Seconds float64 `json:"seconds"`
}

type Report struct {
	Event  string  `json:"dctltest"`
	OK     bool    `json:"ok"`
	Error  string  `json:"error,omitempty"`
	Total  float64 `json:"total"`
	Phases []Phase `json:"phases"`
}

type test struct {
	u       *ui.UI
	dir     string
	iso     string
	mark    int64
	install *Report
	times   []Phase
	doctor  map[string][]doctor.Result
}

func (t *test) file(name string) string { return filepath.Join(t.dir, name) }

func (c TestCmd) Run(ctx context.Context, u *ui.UI) error {
	dir, err := os.MkdirTemp("/var/tmp", "dctl-iso-test-")
	if err != nil {
		return err
	}
	u.KV("run", dir)
	t := &test{u: u, dir: dir, iso: c.ISO, doctor: map[string][]doctor.Result{}}
	err = t.run(ctx, c)
	if t.install != nil {
		u.Header("Install phases")
		for _, p := range t.install.Phases {
			u.KV(p.Name, fmt.Sprintf("%.1fs", p.Seconds))
		}
	}
	u.Header("Harness phases")
	for _, p := range t.times {
		u.KV(p.Name, fmt.Sprintf("%.1fs", p.Seconds))
	}
	errs := []error{err, t.save(), os.Remove(t.file("dctltest.img")), os.Remove(t.file("qmp.sock"))}
	if !c.Keep {
		errs = append(errs, os.Remove(t.file("disk.qcow2")), os.Remove(t.file("vars.fd")))
	}
	return errors.Join(slices.DeleteFunc(errs, func(e error) bool { return errors.Is(e, os.ErrNotExist) })...)
}

func (t *test) run(ctx context.Context, c TestCmd) error {
	answers, _ := json.Marshal(Answers{User: "cullyn", Password: "dctltest", Host: "dctltest", Zone: "America/Los_Angeles", LUKS: luks, Serial: serial})
	if err := os.WriteFile(t.file("answers.json"), answers, 0o600); err != nil {
		return err
	}
	files := map[string]string{"answers.json": t.file("answers.json")}
	if c.Dctl != "" {
		files["dctl"] = c.Dctl
	}
	if c.Bundle != "" {
		files["dotfiles.bundle"] = c.Bundle
	}
	if err := drive(ctx, t.file("dctltest.img"), files); err != nil {
		return err
	}
	if err := cmd(ctx, "cp", ovmfVars, t.file("vars.fd")); err != nil {
		return err
	}
	if err := cmd(ctx, "qemu-img", "create", "-q", "-f", "qcow2", t.file("disk.qcow2"), "32G"); err != nil {
		return err
	}

	v, err := t.start(ctx, true)
	if err != nil {
		return err
	}
	err = t.phase(ctx, v, "install", 45*time.Minute, func(ctx context.Context) error {
		line, err := t.await(ctx, v, installPrefix)
		if err != nil {
			return err
		}
		if t.install, err = parseInstall(line); err != nil {
			return err
		}
		if !t.install.OK {
			return fmt.Errorf("installer failed: %s", t.install.Error)
		}
		if err := v.wait(ctx); err != nil {
			return fmt.Errorf("installer reported success but did not reboot: %w", err)
		}
		return nil
	})
	v.close()
	if err != nil {
		return err
	}

	for _, n := range []string{"1", "2"} {
		v, err := t.start(ctx, false)
		if err != nil {
			return err
		}
		err = t.boot(ctx, v, n)
		v.close()
		if err != nil {
			return err
		}
	}
	if err := errors.Join(accept("1", t.doctor["1"]), accept("2", t.doctor["2"])); err != nil {
		return err
	}
	t.u.OK("installed, unlocked, booted twice; greeter at %s", t.file("greeter.png"))
	return nil
}

func (t *test) boot(ctx context.Context, v *vm, n string) error {
	var base int64
	err := t.phase(ctx, v, "boot "+n, 3*time.Minute, func(ctx context.Context) (err error) {
		base, err = v.quiet(ctx)
		return err
	})
	if err != nil {
		return err
	}
	err = t.phase(ctx, v, "unlock "+n, 2*time.Minute, func(ctx context.Context) error {
		if err := v.typeLine(ctx, luks); err != nil {
			return err
		}
		return v.poll(ctx, time.Second, func() (bool, error) {
			n, err := v.reads(ctx)
			return n >= base+32<<20, err
		})
	})
	if err != nil {
		return err
	}
	err = t.phase(ctx, v, "doctor "+n, 5*time.Minute, func(ctx context.Context) error {
		line, err := t.await(ctx, v, doctorPrefix)
		if err != nil {
			return err
		}
		var rs []doctor.Result
		err = json.Unmarshal([]byte(line), &rs)
		t.doctor[n] = rs
		return err
	})
	if err != nil {
		return err
	}
	if n == "1" {
		return t.phase(ctx, v, "poweroff "+n, 2*time.Minute, func(ctx context.Context) error {
			if _, err := v.qmp.execute(ctx, "system_powerdown", nil); err != nil {
				return err
			}
			return v.wait(ctx)
		})
	}
	return t.phase(ctx, v, "greeter", time.Minute, func(ctx context.Context) error {
		select {
		case <-time.After(20 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
		return v.screendump(ctx, t.file("greeter.png"))
	})
}

func (t *test) phase(ctx context.Context, v *vm, name string, limit time.Duration, do func(context.Context) error) error {
	t.u.Step("%s", name)
	pctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	start := time.Now()
	err := do(pctx)
	t.times = append(t.times, Phase{name, time.Since(start).Seconds()})
	if err == nil {
		return nil
	}
	shot, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if v.screendump(shot, t.file("failure.png")) == nil {
		t.u.KV("screen", t.file("failure.png"))
	}
	return fmt.Errorf("%s: %w\nlast serial lines:\n%s", name, err, t.tail())
}

func accept(boot string, results []doctor.Result) error {
	var errs []error
	for _, g := range required {
		if !slices.ContainsFunc(results, func(r doctor.Result) bool { return r.Group == g || r.Check == g }) {
			errs = append(errs, fmt.Errorf("boot %s: doctor reported no %s checks", boot, g))
		}
	}
	for _, r := range results {
		if !r.Healthy() && (slices.Contains(required, r.Group) || slices.Contains(required, r.Check)) {
			errs = append(errs, fmt.Errorf("boot %s: %s %s: %s", boot, r.Check, r.Status, r.Detail))
		}
	}
	return errors.Join(errs...)
}

func (t *test) save() error {
	timings, err := json.MarshalIndent(struct {
		Install *Report `json:"install"`
		Harness []Phase `json:"harness"`
	}{t.install, t.times}, "", "  ")
	if err != nil {
		return err
	}
	doc, err := json.MarshalIndent(t.doctor, "", "  ")
	if err != nil {
		return err
	}
	return errors.Join(os.WriteFile(t.file("timings.json"), timings, 0o644), os.WriteFile(t.file("doctor.json"), doc, 0o644))
}

func (t *test) argv(install bool) []string {
	argv := []string{"qemu-system-x86_64",
		"-machine", "q35,smm=on,accel=kvm", "-cpu", "host", "-smp", "4", "-m", "8G",
		"-global", "driver=cfi.pflash01,property=secure,value=on",
		"-drive", "if=pflash,format=raw,unit=0,readonly=on,file=" + ovmfCode,
		"-drive", "if=pflash,format=raw,unit=1,file=" + t.file("vars.fd"),
		"-drive", "if=none,id=disk,format=qcow2,file=" + t.file("disk.qcow2"),
		"-device", "virtio-blk-pci,drive=disk,serial=" + serial,
		"-nic", "none",
		"-display", "none",
		"-chardev", "file,id=serial,append=on,path=" + t.file("serial.log"),
		"-serial", "chardev:serial",
		"-qmp", "unix:" + t.file("qmp.sock") + ",server=on,wait=off",
	}
	if install {
		argv = append(argv, "-no-reboot",
			"-drive", "media=cdrom,readonly=on,file="+t.iso,
			"-drive", "if=virtio,format=raw,readonly=on,file="+t.file("dctltest.img"))
	}
	return argv
}

func drive(ctx context.Context, img string, files map[string]string) error {
	size := int64(16 << 20)
	for _, src := range files {
		st, err := os.Stat(src)
		if err != nil {
			return err
		}
		size += st.Size() + st.Size()/16
	}
	f, err := os.Create(img)
	if err != nil {
		return err
	}
	if err := errors.Join(f.Truncate(size), f.Close()); err != nil {
		return err
	}
	if err := cmd(ctx, "mkfs.vfat", "-n", Label, img); err != nil {
		return err
	}
	for name, src := range files {
		if err := cmd(ctx, "mcopy", "-i", img, src, "::"+name); err != nil {
			return err
		}
	}
	return nil
}

func cmd(ctx context.Context, name string, args ...string) error {
	_, err := execx.OSRunner{}.Output(ctx, "", name, args...)
	return err
}

func find(data []byte, prefix string) (string, bool) {
	end := bytes.LastIndexByte(data, '\n')
	for l := range strings.Lines(string(data[:end+1])) {
		if i := strings.Index(l, prefix); i >= 0 {
			return strings.TrimSpace(l[i:]), true
		}
	}
	return "", false
}

func parseInstall(line string) (*Report, error) {
	var r Report
	if err := json.Unmarshal([]byte(line), &r); err != nil || r.Event != "install" {
		return nil, fmt.Errorf("installer serial line %q: %v", line, err)
	}
	return &r, nil
}

func (t *test) serial() []byte {
	data, _ := os.ReadFile(t.file("serial.log"))
	return data[min(t.mark, int64(len(data))):]
}

func (t *test) tail() string {
	data, _ := os.ReadFile(t.file("serial.log"))
	ls := strings.Split(strings.TrimSpace(string(data)), "\n")
	return strings.Join(ls[max(0, len(ls)-20):], "\n")
}

func (t *test) await(ctx context.Context, v *vm, prefix string) (line string, err error) {
	err = v.poll(ctx, 500*time.Millisecond, func() (ok bool, _ error) {
		line, ok = find(t.serial(), prefix)
		return ok, nil
	})
	return line, err
}

type vm struct {
	qmp    *qmp
	done   chan struct{}
	err    error
	stderr bytes.Buffer
	cancel context.CancelFunc
}

func (t *test) start(ctx context.Context, install bool) (*vm, error) {
	if st, err := os.Stat(t.file("serial.log")); err == nil {
		t.mark = st.Size()
	}
	sock := t.file("qmp.sock")
	if err := os.Remove(sock); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	v := spawn(ctx, t.argv(install))
	startup, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	err := v.poll(startup, 100*time.Millisecond, func() (bool, error) {
		conn, err := net.Dial("unix", sock)
		if err != nil {
			return false, nil
		}
		v.qmp, err = dial(startup, conn)
		return true, err
	})
	if err != nil {
		v.close()
		return nil, fmt.Errorf("qemu startup: %w", err)
	}
	return v, nil
}

func spawn(ctx context.Context, argv []string) *vm {
	ctx, cancel := context.WithCancel(ctx)
	v := &vm{done: make(chan struct{}), cancel: cancel}
	c := execx.Grouped(ctx, argv)
	c.Stderr = &v.stderr
	go func() {
		v.err = execx.Reap(ctx, c)
		close(v.done)
	}()
	return v
}

func (v *vm) close() {
	v.cancel()
	<-v.done
	if v.qmp != nil {
		v.qmp.conn.Close()
	}
}

func (v *vm) wait(ctx context.Context) error {
	select {
	case <-v.done:
		if v.err != nil {
			return fmt.Errorf("qemu exited: %w: %s", v.err, strings.TrimSpace(v.stderr.String()))
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (v *vm) poll(ctx context.Context, every time.Duration, done func() (bool, error)) error {
	for {
		if ok, err := done(); ok || err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-v.done:
			if ok, _ := done(); ok {
				return nil
			}
			return fmt.Errorf("qemu exited (%v): %s", v.err, strings.TrimSpace(v.stderr.String()))
		case <-time.After(every):
		}
	}
}

func (v *vm) screendump(ctx context.Context, file string) error {
	_, err := v.qmp.execute(ctx, "screendump", map[string]string{"filename": file, "format": "png"})
	return err
}

func (v *vm) typeLine(ctx context.Context, s string) error {
	for _, r := range s + "\n" {
		key := string(r)
		if r == '\n' {
			key = "ret"
		}
		args := map[string]any{"keys": []map[string]string{{"type": "qcode", "data": key}}}
		if _, err := v.qmp.execute(ctx, "send-key", args); err != nil {
			return err
		}
	}
	return nil
}

func (v *vm) reads(ctx context.Context) (int64, error) {
	raw, err := v.qmp.execute(ctx, "query-blockstats", nil)
	if err != nil {
		return 0, err
	}
	var stats []struct {
		Stats struct {
			Read int64 `json:"rd_bytes"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(raw, &stats); err != nil {
		return 0, err
	}
	var total int64
	for _, s := range stats {
		total += s.Stats.Read
	}
	return total, nil
}

func (v *vm) quiet(ctx context.Context) (int64, error) {
	var last int64
	var since time.Time
	err := v.poll(ctx, time.Second, func() (bool, error) {
		n, err := v.reads(ctx)
		if n != last {
			last, since = n, time.Now()
		}
		return n >= 8<<20 && time.Since(since) >= 5*time.Second, err
	})
	if err != nil {
		return 0, fmt.Errorf("disk reads never settled at a passphrase prompt (%d bytes read): %w", last, err)
	}
	return last, nil
}

type qmp struct {
	conn net.Conn
	dec  *json.Decoder
	dead error
}

type reply struct {
	Return json.RawMessage `json:"return"`
	Event  string          `json:"event"`
	Error  *struct {
		Class string `json:"class"`
		Desc  string `json:"desc"`
	} `json:"error"`
}

func dial(ctx context.Context, conn net.Conn) (*qmp, error) {
	q := &qmp{conn: conn, dec: json.NewDecoder(conn)}
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { conn.SetDeadline(time.Now()) })
	var greeting struct {
		QMP json.RawMessage `json:"QMP"`
	}
	err := q.dec.Decode(&greeting)
	stop()
	if err == nil && greeting.QMP == nil {
		err = errors.New("no greeting")
	}
	if err == nil {
		_, err = q.execute(ctx, "qmp_capabilities", nil)
	}
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("qmp: %w", err)
	}
	return q, nil
}

func (q *qmp) execute(ctx context.Context, command string, args any) (json.RawMessage, error) {
	if q.dead != nil {
		return nil, q.dead
	}
	deadline, _ := ctx.Deadline()
	q.conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { q.conn.SetDeadline(time.Now()) })
	msg, err := q.roundtrip(command, args)
	if !stop() && err == nil {
		err = ctx.Err()
	}
	if err != nil {
		q.dead = fmt.Errorf("qmp %s: %w", command, err)
		return nil, q.dead
	}
	q.conn.SetDeadline(time.Time{})
	if msg.Error != nil {
		return nil, fmt.Errorf("qmp %s: %s: %s", command, msg.Error.Class, msg.Error.Desc)
	}
	return msg.Return, nil
}

func (q *qmp) roundtrip(command string, args any) (reply, error) {
	req := map[string]any{"execute": command}
	if args != nil {
		req["arguments"] = args
	}
	if err := json.NewEncoder(q.conn).Encode(req); err != nil {
		return reply{}, err
	}
	for {
		var msg reply
		if err := q.dec.Decode(&msg); err != nil || msg.Event == "" {
			return msg, err
		}
	}
}
