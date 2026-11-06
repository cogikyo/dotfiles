// Package vpn manages VPN connections via NetworkManager.
package vpn

import (
	"dotfiles/cmds/internal/config"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	nmService      = "org.freedesktop.NetworkManager"
	nmSettings     = "org.freedesktop.NetworkManager.Settings"
	nmSettingsPath = "/org/freedesktop/NetworkManager/Settings"
	nmConnection   = "org.freedesktop.NetworkManager.Settings.Connection"
	nmUpdateToDisk = 0x1

	systemConnections = "/etc/NetworkManager/system-connections"

	settleTimeout = 30 * time.Second
)

var lockPath = "/run/vpn-keep.lock"

// VPN dispatches VPN subcommands against NetworkManager.
type VPN struct {
	config *config.VPNConfig
	// Secret asks for a missing VPN secret; nil means no terminal can answer.
	Secret func(label string) (string, error)
}

type connection struct {
	Name    string
	Profile string
}

type installOptions struct {
	Replace      bool
	ResetSecrets bool
}

// New creates a VPN command handler bound to the given config.
func New(cfg *config.VPNConfig) *VPN {
	return &VPN{config: cfg}
}

// Execute parses and runs a VPN subcommand.
func (v *VPN) Execute(arg string) (string, error) {
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		return v.list()
	}

	switch fields[0] {
	case "list":
		return v.list()
	case "status":
		if len(fields) == 1 {
			return v.statusAll()
		}
		conn, err := v.resolve(fields[1])
		if err != nil {
			return "", err
		}
		return v.status(conn)
	case "install", "import":
		options, fields := parseInstallOptions(fields)
		if len(fields) == 1 {
			return v.installAll(options)
		}
		conn, err := v.resolveConfigured(fields[1])
		if err != nil {
			return "", err
		}
		return v.install(conn, options)
	case "export":
		if len(fields) < 2 {
			return "", fmt.Errorf("usage: vpn export <connection>")
		}
		conn, err := v.resolve(fields[1])
		if err != nil {
			return "", err
		}
		return v.export(conn)
	}

	conn, err := v.resolve(fields[0])
	if err != nil {
		return "", err
	}
	verb := "toggle"
	if len(fields) > 1 {
		verb = fields[1]
	}
	if next, ok := modes[verb]; ok {
		return v.apply(conn, next)
	}
	switch verb {
	case "status":
		return v.status(conn)
	case "install", "import":
		options, _ := parseInstallOptions(fields[1:])
		return v.install(conn, options)
	case "export":
		return v.export(conn)
	}
	return "", fmt.Errorf("usage: vpn [list|status] | vpn <connection> [toggle|split|full|off|up|status|install|export]")
}

func parseInstallOptions(fields []string) (installOptions, []string) {
	options := installOptions{Replace: true}
	keep := fields[:0]
	for _, field := range fields {
		switch field {
		case "--no-replace":
			options.Replace = false
		case "--reset-secrets":
			options.ResetSecrets = true
		default:
			keep = append(keep, field)
		}
	}
	return options, keep
}

func (v *VPN) resolve(name string) (connection, error) {
	if v.config != nil && v.config.Connections != nil {
		if cfg, ok := v.config.Connections[name]; ok {
			return normalizeConnection(name, cfg)
		}
	}
	return connection{Name: name}, nil
}

func (v *VPN) resolveConfigured(name string) (connection, error) {
	if v.config == nil || v.config.Connections == nil {
		return connection{}, fmt.Errorf("no vpn connections configured")
	}
	cfg, ok := v.config.Connections[name]
	if !ok {
		return connection{}, fmt.Errorf("unknown configured vpn connection: %s", name)
	}
	return normalizeConnection(name, cfg)
}

func normalizeConnection(key string, cfg config.VPNConnection) (connection, error) {
	return connection{
		Name:    key,
		Profile: cfg.Path(key),
	}, nil
}

type profile struct {
	keep  bool
	split bool
}

func (p profile) mode() string {
	switch {
	case !p.keep:
		return "off"
	case p.split:
		return "split"
	}
	return "full"
}

var modes = map[string]func(profile) profile{
	"split":  func(profile) profile { return profile{keep: true, split: true} },
	"down":   func(profile) profile { return profile{keep: true, split: true} },
	"full":   func(profile) profile { return profile{keep: true} },
	"up":     func(profile) profile { return profile{keep: true} },
	"off":    func(p profile) profile { return profile{split: p.split} },
	"toggle": func(p profile) profile { return profile{keep: true, split: p.mode() == "full"} },
}

func readProfile(name string) (profile, error) {
	out, err := nmcliOutput("-g", "connection.autoconnect,ipv4.never-default", "connection", "show", "id", name)
	if err != nil {
		return profile{}, err
	}
	values := strings.Fields(out)
	if len(values) != 2 {
		return profile{}, fmt.Errorf("nmcli: unexpected profile fields for %s: %q", name, out)
	}
	return profile{keep: values[0] == "yes", split: values[1] == "yes"}, nil
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func (v *VPN) apply(conn connection, next func(profile) profile) (string, error) {
	exists, err := connectionExists(conn.Name)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("vpn connection not found: %s", conn.Name)
	}
	h, err := lock()
	if err != nil {
		return "", err
	}
	defer h.unlock()
	have, err := readProfile(conn.Name)
	if err != nil {
		return "", err
	}
	want := next(have)
	if want != have {
		if _, err := h.run("nmcli", "connection", "modify", "id", conn.Name,
			"connection.autoconnect", yesNo(want.keep),
			"ipv4.never-default", yesNo(want.split)); err != nil {
			return "", err
		}
	}
	if err := v.converge(h, conn.Name, want); err != nil {
		return "", err
	}
	return v.status(conn)
}

func (v *VPN) converge(h *held, name string, want profile) error {
	active, err := v.active(name)
	if err != nil {
		return err
	}
	routed := false
	if active {
		out, err := nmcliOutput("-g", "IP4.ROUTE", "connection", "show", "id", name)
		if err != nil {
			return err
		}
		routed = hasDefaultRoute(out)
	}
	down, up := reconcile(want, active, routed)
	if down {
		if _, err := h.run("nmcli", "connection", "down", "id", name); err != nil {
			return err
		}
	}
	if up {
		if err := settle(name); err != nil {
			return err
		}
		if _, err := h.run("nmcli", "connection", "up", "id", name); err != nil {
			return err
		}
	}
	return nil
}

func reconcile(want profile, active, routed bool) (down, up bool) {
	down = active && (!want.keep || want.split == routed)
	up = want.keep && (!active || down)
	return down, up
}

func hasDefaultRoute(routes string) bool {
	for route := range strings.SplitSeq(routes, "|") {
		dst, _, _ := strings.Cut(strings.TrimSpace(route), ",")
		if strings.TrimSpace(dst) == "dst = 0.0.0.0/0" {
			return true
		}
	}
	return false
}

var errUnlocked = errors.New("vpn lock not held")

type held struct{ f *os.File }

func lock() (*held, error) {
	f, err := os.Open(lockPath)
	if err != nil {
		return nil, fmt.Errorf("open vpn lock: %w (installed by system/etc/tmpfiles.d/vpn-keep.conf)", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock %s: %w", lockPath, err)
	}
	return &held{f: f}, nil
}

func (h *held) unlock() {
	if h.f != nil {
		h.f.Close()
		h.f = nil
	}
}

func (h *held) check() error {
	if h == nil || h.f == nil {
		return errUnlocked
	}
	return nil
}

func (h *held) run(name string, args ...string) (string, error) {
	if err := h.check(); err != nil {
		return "", err
	}
	return combined(exec.Command(name, args...))
}

func (h *held) storeSecrets(obj dbus.BusObject, secrets map[string]string) error {
	if err := h.check(); err != nil {
		return err
	}
	var settings map[string]map[string]dbus.Variant
	if err := obj.Call(nmConnection+".GetSettings", 0).Store(&settings); err != nil {
		return fmt.Errorf("read VPN settings: %w", err)
	}
	vpn, ok := settings["vpn"]
	if !ok {
		return errors.New("connection has no vpn setting")
	}
	vpn["secrets"] = dbus.MakeVariant(secrets)
	if err := obj.Call(nmConnection+".Update2", 0, settings, uint32(nmUpdateToDisk), map[string]dbus.Variant{}).Err; err != nil {
		return fmt.Errorf("store VPN secrets: %w", err)
	}
	return nil
}

func authorize() error {
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo -v: %w", err)
	}
	return nil
}

func settle(name string) error {
	out, err := nmcliOutput("-g", "vpn.service-type", "connection", "show", "id", name)
	if err != nil {
		return err
	}
	service := strings.TrimSpace(out)
	bus, err := dbus.ConnectSystemBus()
	if err != nil {
		return fmt.Errorf("connect system bus: %w", err)
	}
	defer bus.Close()
	deadline := time.Now().Add(settleTimeout)
	for {
		var owned bool
		if err := bus.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, service).Store(&owned); err != nil {
			return fmt.Errorf("check vpn plugin %s: %w", service, err)
		}
		if !owned {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("vpn plugin %s still running %s after disconnecting %s", service, settleTimeout, name)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func (v *VPN) status(conn connection) (string, error) {
	p, err := readProfile(conn.Name)
	if err != nil {
		return "", err
	}
	active, err := v.active(conn.Name)
	if err != nil {
		return "", err
	}
	state := "disconnected"
	if active {
		state = "connected"
	}
	return fmt.Sprintf("vpn %s: %s (%s)", conn.Name, p.mode(), state), nil
}

func (v *VPN) statusAll() (string, error) {
	out, err := nmcliOutput("-t", "-f", "TYPE,NAME", "connection", "show", "--active")
	if err != nil {
		return "", err
	}
	var active []string
	for line := range strings.Lines(out) {
		typ, name, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && typ == "vpn" {
			active = append(active, name)
		}
	}
	if len(active) == 0 {
		return "vpn disconnected", nil
	}
	sort.Strings(active)
	return "vpn connected: " + strings.Join(active, ", "), nil
}

func (v *VPN) list() (string, error) {
	out, err := nmcliOutput("-t", "-f", "NAME,TYPE", "connection", "show")
	if err != nil {
		return "", err
	}

	var lines []string
	for line := range strings.Lines(out) {
		name, typ, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok || typ != "vpn" {
			continue
		}
		lines = append(lines, name)
	}
	if len(lines) == 0 {
		return "vpn connections: (none)", nil
	}
	sort.Strings(lines)
	return "vpn connections:\n" + strings.Join(lines, "\n"), nil
}

func (v *VPN) install(conn connection, options installOptions) (string, error) {
	if conn.Profile == "" {
		return "", fmt.Errorf("vpn.%s.profile is required", conn.Name)
	}
	if _, err := os.Stat(conn.Profile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("profile not found: %s (run dctl secrets decrypt %s first)", conn.Profile, secretName(conn))
		}
		return "", err
	}
	uuid, completeProfile, err := readKeyfile(conn.Profile)
	if err != nil {
		return "", err
	}
	if completeProfile && uuid == "" {
		return "", fmt.Errorf("profile has no [connection] uuid: %s", conn.Profile)
	}
	if completeProfile {
		if err := authorize(); err != nil {
			return "", err
		}
	}
	h, err := lock()
	if err != nil {
		return "", err
	}
	defer h.unlock()
	exists, err := connectionExists(conn.Name)
	if err != nil {
		return "", err
	}
	if !completeProfile && !exists {
		return "", fmt.Errorf("profile is not a complete NetworkManager keyfile and %q is not installed: %s", conn.Name, conn.Profile)
	}
	if exists && completeProfile && !options.Replace {
		return "", fmt.Errorf("connection already exists: %s", conn.Name)
	}

	system := filepath.Join(systemConnections, conn.Name+".nmconnection")
	loaded := false
	undo := func(err error) error {
		if !completeProfile || exists {
			return err
		}
		var deleteErr error
		if loaded {
			_, deleteErr = h.run("sudo", "nmcli", "connection", "delete", "uuid", uuid)
		}
		_, removeErr := h.run("sudo", "rm", "-f", system)
		return errors.Join(err, deleteErr, removeErr)
	}
	if completeProfile {
		if _, err := h.run("sudo", "install", "-m", "600", "-o", "root", "-g", "root", conn.Profile, system); err != nil {
			return "", undo(err)
		}
		out, err := h.run("sudo", "nmcli", "connection", "load", system)
		if err != nil {
			return "", undo(err)
		}
		if _, err := nmcliOutput("-g", "connection.uuid", "connection", "show", "uuid", uuid); err != nil {
			msg := strings.TrimSpace(out)
			if msg == "" {
				msg = err.Error()
			}
			return "", undo(fmt.Errorf("NetworkManager did not load %s as connection %s: %s (see journalctl -u NetworkManager)", system, uuid, msg))
		}
		loaded = true
	}
	if uuid == "" {
		out, err := nmcliOutput("-g", "connection.uuid", "connection", "show", "id", conn.Name)
		if err != nil {
			return "", undo(err)
		}
		uuid = strings.TrimSpace(out)
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("vpn installed: %s from %s", conn.Name, conn.Profile))
	if completeProfile {
		lines = append(lines, "profile loaded from "+system)
	} else {
		lines = append(lines, "profile incomplete; using installed NetworkManager connection as base")
	}

	if err := v.ensureSecrets(h, conn.Name, uuid, options.ResetSecrets); err != nil {
		return "", undo(err)
	}
	lines = append(lines, "VPN secrets stored in NetworkManager")
	if err := os.Remove(conn.Profile); err != nil {
		return "", fmt.Errorf("remove staged profile: %w", err)
	}
	lines = append(lines, "staged profile removed")
	want, err := readProfile(conn.Name)
	if err != nil {
		return "", err
	}
	if err := v.converge(h, conn.Name, want); err != nil {
		return "", fmt.Errorf("vpn %s installed, but reaching %s failed: %w", conn.Name, want.mode(), err)
	}
	status, err := v.status(conn)
	if err != nil {
		return "", err
	}
	lines = append(lines, status)
	return strings.Join(lines, "\n"), nil
}

// readKeyfile reports the [connection] uuid and whether the section declares a type, which marks a complete keyfile.
func readKeyfile(path string) (uuid string, complete bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	uuid, complete = parseKeyfile(string(data))
	return uuid, complete, nil
}

func parseKeyfile(data string) (uuid string, complete bool) {
	inConnection := false
	for line := range strings.Lines(data) {
		line = strings.TrimSpace(line)
		switch {
		case line == "[connection]":
			inConnection = true
		case strings.HasPrefix(line, "["):
			inConnection = false
		case inConnection && strings.HasPrefix(line, "type="):
			complete = true
		case inConnection && strings.HasPrefix(line, "uuid="):
			uuid = strings.TrimSpace(strings.TrimPrefix(line, "uuid="))
		}
	}
	return uuid, complete
}

func autoconnectOff(data string) string {
	var out strings.Builder
	inConnection := false
	for line := range strings.Lines(data) {
		trimmed := strings.TrimSpace(line)
		if inConnection && strings.HasPrefix(trimmed, "autoconnect=") {
			continue
		}
		out.WriteString(line)
		if strings.HasPrefix(trimmed, "[") {
			inConnection = trimmed == "[connection]"
			if inConnection {
				if !strings.HasSuffix(line, "\n") {
					out.WriteString("\n")
				}
				out.WriteString("autoconnect=false\n")
			}
		}
	}
	return out.String()
}

func (v *VPN) ensureSecrets(h *held, name, uuid string, reset bool) error {
	bus, err := dbus.ConnectSystemBus()
	if err != nil {
		return fmt.Errorf("connect system bus: %w", err)
	}
	defer bus.Close()
	path, err := connectionPath(bus.Object(nmService, nmSettingsPath), uuid)
	if err != nil {
		return err
	}
	obj := bus.Object(nmService, path)
	secrets, err := vpnSecrets(obj)
	if err != nil {
		return err
	}
	required := []string{"password", "ipsec-psk"}
	changed := false
	for _, key := range required {
		if !reset && secrets[key] != "" {
			continue
		}
		secret, err := v.prompt(name, key)
		if err != nil {
			return err
		}
		secrets[key] = secret
		changed = true
	}
	if changed {
		if err := h.storeSecrets(obj, secrets); err != nil {
			return err
		}
	}
	stored, err := vpnSecrets(obj)
	if err != nil {
		return err
	}
	for _, key := range required {
		if stored[key] != secrets[key] {
			return fmt.Errorf("NetworkManager did not persist VPN %s for %s", key, name)
		}
	}
	return nil
}

func connectionPath(settings dbus.BusObject, uuid string) (dbus.ObjectPath, error) {
	var path dbus.ObjectPath
	if err := settings.Call(nmSettings+".GetConnectionByUuid", 0, uuid).Store(&path); err != nil {
		return "", fmt.Errorf("find NetworkManager connection %s: %w", uuid, err)
	}
	return path, nil
}

func vpnSecrets(obj dbus.BusObject) (map[string]string, error) {
	var settings map[string]map[string]dbus.Variant
	if err := obj.Call(nmConnection+".GetSecrets", 0, "vpn").Store(&settings); err != nil {
		return nil, fmt.Errorf("read VPN secrets: %w", err)
	}
	secrets := map[string]string{}
	if value, ok := settings["vpn"]["secrets"]; ok {
		if err := value.Store(&secrets); err != nil {
			return nil, fmt.Errorf("decode VPN secrets: %w", err)
		}
	}
	return secrets, nil
}

func (v *VPN) prompt(name, key string) (string, error) {
	if v.Secret == nil {
		return "", fmt.Errorf("vpn secret %s missing for %s and no terminal can answer", key, name)
	}
	secret, err := v.Secret(fmt.Sprintf("VPN %s for %s:", key, name))
	if err != nil {
		return "", err
	}
	if secret == "" {
		return "", fmt.Errorf("vpn secret %s for %s is empty", key, name)
	}
	return secret, nil
}

func (v *VPN) installAll(options installOptions) (string, error) {
	conns, err := v.configuredConnections()
	if err != nil {
		return "", err
	}

	var lines []string
	for _, conn := range conns {
		msg, err := v.install(conn, options)
		if err != nil {
			if !options.Replace && strings.Contains(err.Error(), "connection already exists") {
				lines = append(lines, fmt.Sprintf("vpn already installed: %s", conn.Name))
				continue
			}
			return "", err
		}
		lines = append(lines, msg)
	}
	return strings.Join(lines, "\n"), nil
}

func (v *VPN) configuredConnections() ([]connection, error) {
	if v.config == nil || len(v.config.Connections) == 0 {
		return nil, fmt.Errorf("no vpn connections configured")
	}

	names := make([]string, 0, len(v.config.Connections))
	for name := range v.config.Connections {
		names = append(names, name)
	}
	sort.Strings(names)

	conns := make([]connection, 0, len(names))
	for _, name := range names {
		conn, err := v.resolveConfigured(name)
		if err != nil {
			return nil, err
		}
		conns = append(conns, conn)
	}
	return conns, nil
}

func (v *VPN) export(conn connection) (string, error) {
	if conn.Profile == "" {
		return "", fmt.Errorf("vpn.%s.profile is required", conn.Name)
	}
	exists, err := connectionExists(conn.Name)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("vpn connection not found: %s", conn.Name)
	}
	system := filepath.Join(systemConnections, conn.Name+".nmconnection")
	out, err := exec.Command("sudo", "cat", system).Output()
	if err != nil {
		msg := err.Error()
		if exit, ok := errors.AsType[*exec.ExitError](err); ok && len(exit.Stderr) > 0 {
			msg = strings.TrimSpace(string(exit.Stderr))
		}
		return "", fmt.Errorf("sudo cat %s: %s", system, msg)
	}
	data := string(out)
	clear(out)
	if uuid, complete := parseKeyfile(data); !complete || uuid == "" {
		return "", fmt.Errorf("%s is not a complete NetworkManager keyfile with a uuid", system)
	}
	dir := filepath.Dir(conn.Profile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(conn.Profile)+".*")
	if err != nil {
		return "", err
	}
	_, err = tmp.WriteString(autoconnectOff(data))
	err = errors.Join(err, tmp.Sync(), tmp.Close())
	if err == nil {
		err = os.Rename(tmp.Name(), conn.Profile)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return fmt.Sprintf("vpn exported: %s from %s\nprofile: %s (autoconnect off)\nnext: run dctl secrets sync for '%s', then delete the staged profile", conn.Name, system, conn.Profile, manifestTarget(conn.Profile)), nil
}

func (v *VPN) active(name string) (bool, error) {
	out, err := nmcliOutput("-t", "-f", "TYPE,NAME", "connection", "show", "--active")
	if err != nil {
		return false, err
	}
	for line := range strings.Lines(out) {
		typ, activeName, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && typ == "vpn" && activeName == name {
			return true, nil
		}
	}
	return false, nil
}

// connectionExists rejects a name shared by several NetworkManager connections, since nmcli would act on an arbitrary one.
func connectionExists(name string) (bool, error) {
	out, err := nmcliOutput("-t", "-f", "NAME", "connection", "show")
	if err != nil {
		return false, err
	}
	unescape := strings.NewReplacer(`\\`, `\`, `\:`, ":")
	matches := 0
	for line := range strings.Lines(out) {
		if unescape.Replace(strings.TrimSuffix(line, "\n")) == name {
			matches++
		}
	}
	if matches > 1 {
		return false, fmt.Errorf("vpn connection name %q matches %d NetworkManager connections; rename or delete the duplicates", name, matches)
	}
	return matches == 1, nil
}

func secretName(conn connection) string {
	return conn.Name + ".nmconnection"
}

func manifestTarget(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if rel, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(rel, "..") {
		return "~/" + rel
	}
	return path
}

func nmcliOutput(args ...string) (string, error) {
	return combined(exec.Command("nmcli", args...))
}

func combined(cmd *exec.Cmd) (string, error) {
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s: %s", strings.Join(cmd.Args, " "), msg)
	}
	return string(out), nil
}
