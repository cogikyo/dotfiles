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

	"github.com/godbus/dbus/v5"
)

const (
	nmService      = "org.freedesktop.NetworkManager"
	nmSettings     = "org.freedesktop.NetworkManager.Settings"
	nmSettingsPath = "/org/freedesktop/NetworkManager/Settings"
	nmConnection   = "org.freedesktop.NetworkManager.Settings.Connection"
	nmUpdateToDisk = 0x1
)

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
	case "up", "connect":
		if len(fields) < 2 {
			return "", fmt.Errorf("usage: vpn up <connection>")
		}
		conn, err := v.resolve(fields[1])
		if err != nil {
			return "", err
		}
		return v.up(conn)
	case "down", "disconnect":
		if len(fields) < 2 {
			return "", fmt.Errorf("usage: vpn down <connection>")
		}
		conn, err := v.resolve(fields[1])
		if err != nil {
			return "", err
		}
		return v.down(conn)
	case "toggle":
		if len(fields) < 2 {
			return "", fmt.Errorf("usage: vpn toggle <connection>")
		}
		conn, err := v.resolve(fields[1])
		if err != nil {
			return "", err
		}
		return v.toggle(conn)
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
	default:
		conn, err := v.resolve(fields[0])
		if err != nil {
			return "", err
		}
		if len(fields) > 1 {
			switch fields[1] {
			case "up", "connect":
				return v.up(conn)
			case "down", "disconnect":
				return v.down(conn)
			case "status":
				return v.status(conn)
			case "toggle":
				return v.toggle(conn)
			case "install", "import":
				options, _ := parseInstallOptions(fields[1:])
				return v.install(conn, options)
			case "export":
				return v.export(conn)
			default:
				return "", fmt.Errorf("usage: vpn [list|status] | vpn <connection> [toggle|up|down|status|install|export]")
			}
		}
		return v.toggle(conn)
	}
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

func (v *VPN) toggle(conn connection) (string, error) {
	active, err := v.active(conn.Name)
	if err != nil {
		return "", err
	}
	if active {
		return v.down(conn)
	}
	return v.up(conn)
}

func (v *VPN) up(conn connection) (string, error) {
	if _, err := connectionExists(conn.Name); err != nil {
		return "", err
	}
	if err := runNMCLI("connection", "up", "id", conn.Name); err != nil {
		return "", err
	}
	return fmt.Sprintf("vpn connected: %s", conn.Name), nil
}

func (v *VPN) down(conn connection) (string, error) {
	if _, err := connectionExists(conn.Name); err != nil {
		return "", err
	}
	if err := runNMCLI("connection", "down", "id", conn.Name); err != nil {
		return "", err
	}
	return fmt.Sprintf("vpn disconnected: %s", conn.Name), nil
}

func (v *VPN) status(conn connection) (string, error) {
	active, err := v.active(conn.Name)
	if err != nil {
		return "", err
	}
	if active {
		return fmt.Sprintf("vpn connected: %s", conn.Name), nil
	}
	return fmt.Sprintf("vpn disconnected: %s", conn.Name), nil
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

	undo := func(err error) error {
		if completeProfile && !exists {
			return errors.Join(err, sudoNMCLI("connection", "delete", "id", conn.Name))
		}
		return err
	}
	if completeProfile {
		if err := sudoNMCLI("connection", "load", conn.Profile); err != nil {
			return "", err
		}
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
		lines = append(lines, "profile loaded")
	} else {
		lines = append(lines, "profile incomplete; using installed NetworkManager connection as base")
	}

	if err := v.ensureSecrets(conn.Name, uuid, options.ResetSecrets); err != nil {
		return "", undo(err)
	}
	lines = append(lines, "VPN secrets stored in NetworkManager")
	if err := os.Remove(conn.Profile); err != nil {
		return "", fmt.Errorf("remove staged profile: %w", err)
	}
	lines = append(lines, "staged profile removed")
	return strings.Join(lines, "\n"), nil
}

// readKeyfile reports the [connection] uuid and whether the section declares a type, which marks a complete keyfile.
func readKeyfile(path string) (uuid string, complete bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	inConnection := false
	for line := range strings.Lines(string(data)) {
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
	return uuid, complete, nil
}

func (v *VPN) ensureSecrets(name, uuid string, reset bool) error {
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
		if err := storeVPNSecrets(obj, secrets); err != nil {
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

func storeVPNSecrets(obj dbus.BusObject, secrets map[string]string) error {
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
	if err := os.MkdirAll(filepath.Dir(conn.Profile), 0o700); err != nil {
		return "", err
	}
	data, err := nmcliOutput("connection", "export", conn.Name)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(conn.Profile, []byte(data), 0o600); err != nil {
		return "", err
	}
	return fmt.Sprintf("vpn exported: %s\nprofile: %s\nwarning: NetworkManager exports can omit keyring secrets; verify the profile before syncing it\nnext: add '%s:%s:600:staged' to secrets/manifest, then run dctl secrets sync", conn.Name, conn.Profile, secretName(conn), manifestTarget(conn.Profile)), nil
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

func runNMCLI(args ...string) error {
	cmd := exec.Command("nmcli", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("nmcli %s: %s", strings.Join(args, " "), msg)
	}
	return nil
}

func sudoNMCLI(args ...string) error {
	cmd := exec.Command("sudo", append([]string{"nmcli"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("sudo nmcli %s: %s", strings.Join(args, " "), msg)
	}
	return nil
}

func nmcliOutput(args ...string) (string, error) {
	cmd := exec.Command("nmcli", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("nmcli %s: %s", strings.Join(args, " "), msg)
	}
	return string(out), nil
}
