// Package vpn manages VPN connections via NetworkManager.
//
// Responsibilities:
// - Resolve configured NetworkManager connection profiles.
// - Load and export staged .nmconnection profiles.
// - Run connect, disconnect, status, and toggle commands.
package vpn

// vpn.go defines the config-backed VPN command dispatcher and NetworkManager operations.

import (
	"dotfiles/cmds/internal/config"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/term"
)

// VPN dispatches VPN subcommands against NetworkManager.
type VPN struct {
	config *config.VPNConfig
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
	profile := cfg.Profile
	if profile == "" {
		profile = fmt.Sprintf("~/.local/share/dotfiles/vpn/%s.nmconnection", key)
	}
	return connection{
		Name:    key,
		Profile: config.ExpandPath(profile),
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
	if err := runNMCLI("connection", "up", conn.Name); err != nil {
		return "", err
	}
	return fmt.Sprintf("vpn connected: %s", conn.Name), nil
}

func (v *VPN) down(conn connection) (string, error) {
	if err := runNMCLI("connection", "down", conn.Name); err != nil {
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
			return "", fmt.Errorf("profile not found: %s (run ./install.sh secrets first)", conn.Profile)
		}
		return "", err
	}
	completeProfile, err := isNetworkManagerKeyfile(conn.Profile)
	if err != nil {
		return "", err
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

	if completeProfile {
		if err := runSudoNMCLI("connection", "load", conn.Profile); err != nil {
			return "", err
		}
	}
	if exists, err = connectionExists(conn.Name); err != nil {
		return "", err
	} else if !exists {
		return "", fmt.Errorf("profile loaded but NetworkManager connection %q is missing", conn.Name)
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("vpn installed: %s from %s", conn.Name, conn.Profile))
	if completeProfile {
		lines = append(lines, "profile loaded")
	} else {
		lines = append(lines, "profile incomplete; using installed NetworkManager connection as base")
	}

	if err := ensureVPNSecrets(conn.Name, options.ResetSecrets); err != nil {
		return "", err
	}
	lines = append(lines, "VPN secrets stored in NetworkManager")
	if err := copyConnectionKeyfile(conn); err != nil {
		return "", err
	}
	lines = append(lines, "profile updated; run secrets sync --force to encrypt it")
	return strings.Join(lines, "\n"), nil
}

func isNetworkManagerKeyfile(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
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
			return true, nil
		}
	}
	return false, nil
}

func vpnSecrets(name string) (map[string]string, error) {
	out, err := nmcliOutput("--show-secrets", "-g", "vpn.secrets", "connection", "show", name)
	if err != nil {
		return nil, err
	}
	secrets := map[string]string{}
	for _, field := range strings.FieldsFunc(out, func(r rune) bool { return r == ',' || r == '\n' }) {
		key, value, ok := strings.Cut(strings.TrimSpace(field), "=")
		if ok {
			secrets[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return secrets, nil
}

func ensureVPNSecrets(name string, reset bool) error {
	secrets, err := vpnSecrets(name)
	if err != nil {
		return err
	}
	required := []string{"password", "ipsec-psk"}
	for _, key := range required {
		if !reset && secrets[key] != "" {
			continue
		}
		secret, err := promptSecret(name, key)
		if err != nil {
			return err
		}
		if err := runSudoNMCLI("connection", "modify", name, "+vpn.secrets", key+"="+secret); err != nil {
			return err
		}
	}
	return nil
}

func promptSecret(name, key string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("vpn secret %s missing for %s and stdin is not a terminal", key, name)
	}
	fmt.Fprintf(os.Stderr, "VPN %s for %s: ", key, name)
	secret, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if len(secret) == 0 {
		return "", fmt.Errorf("vpn secret %s for %s is empty", key, name)
	}
	return string(secret), nil
}

func copyConnectionKeyfile(conn connection) error {
	source, err := connectionKeyfile(conn.Name)
	if err != nil {
		return err
	}
	if source == "" {
		return fmt.Errorf("NetworkManager did not report a keyfile for %s", conn.Name)
	}
	if err := os.MkdirAll(filepath.Dir(conn.Profile), 0o700); err != nil {
		return err
	}
	current, err := user.Current()
	if err != nil {
		return err
	}
	return runSudo("install", "-m", "600", "-o", current.Uid, "-g", current.Gid, source, conn.Profile)
}

func connectionKeyfile(name string) (string, error) {
	out, err := nmcliOutput("-t", "-f", "NAME,FILENAME", "connection", "show")
	if err != nil {
		return "", err
	}
	for line := range strings.Lines(out) {
		connName, filename, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && connName == name {
			return filename, nil
		}
	}
	return "", nil
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
	return fmt.Sprintf("vpn exported: %s\nprofile: %s\nwarning: NetworkManager exports can omit keyring secrets; verify the profile before syncing it\nnext: add '%s:%s:600' to etc/secrets/manifest, then run secrets sync", conn.Name, conn.Profile, secretName(conn), manifestTarget(conn.Profile)), nil
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

func connectionExists(name string) (bool, error) {
	err := exec.Command("nmcli", "-t", "connection", "show", name).Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, err
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
		return fmt.Errorf("nmcli %s: %s", commandString(args), msg)
	}
	return nil
}

func runSudoNMCLI(args ...string) error {
	return runSudo(append([]string{"nmcli"}, args...)...)
}

func runSudo(args ...string) error {
	cmd := exec.Command("sudo", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("sudo %s: %s", commandString(args), msg)
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
		return "", fmt.Errorf("nmcli %s: %s", commandString(args), msg)
	}
	return string(out), nil
}

func commandString(args []string) string {
	redacted := slices.Clone(args)
	for i, arg := range redacted {
		key, _, ok := strings.Cut(arg, "=")
		if ok && strings.Contains(strings.ToLower(key), "password") {
			redacted[i] = key + "=<redacted>"
		}
	}
	return strings.Join(redacted, " ")
}
