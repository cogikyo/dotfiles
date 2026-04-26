// Package vpn manages VPN connections via NetworkManager.
package vpn

import (
	"dotfiles/daemons/config"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// VPN dispatches VPN subcommands against config-backed aliases.
type VPN struct {
	config *config.VPNConfig
}

type connection struct {
	Alias   string
	Name    string
	Type    string
	Profile string
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
			return "", fmt.Errorf("usage: vpn up <alias|connection>")
		}
		conn, err := v.resolve(fields[1])
		if err != nil {
			return "", err
		}
		return v.up(conn)
	case "down", "disconnect":
		if len(fields) < 2 {
			return "", fmt.Errorf("usage: vpn down <alias|connection>")
		}
		conn, err := v.resolve(fields[1])
		if err != nil {
			return "", err
		}
		return v.down(conn)
	case "toggle":
		if len(fields) < 2 {
			return "", fmt.Errorf("usage: vpn toggle <alias|connection>")
		}
		conn, err := v.resolve(fields[1])
		if err != nil {
			return "", err
		}
		return v.toggle(conn)
	case "install", "import":
		if len(fields) < 2 {
			return "", fmt.Errorf("usage: vpn install <alias>")
		}
		conn, err := v.resolveConfigured(fields[1])
		if err != nil {
			return "", err
		}
		replace := len(fields) > 2 && fields[2] == "--replace"
		return v.install(conn, replace)
	case "export":
		if len(fields) < 2 {
			return "", fmt.Errorf("usage: vpn export <alias|connection>")
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
				replace := len(fields) > 2 && fields[2] == "--replace"
				return v.install(conn, replace)
			case "export":
				return v.export(conn)
			default:
				return "", fmt.Errorf("usage: vpn [list|status] | vpn <alias|connection> [toggle|up|down|status|install|export]")
			}
		}
		return v.toggle(conn)
	}
}

func (v *VPN) resolve(name string) (connection, error) {
	if v.config != nil && v.config.Connections != nil {
		if cfg, ok := v.config.Connections[name]; ok {
			return normalizeConnection(name, cfg)
		}
		for alias, cfg := range v.config.Connections {
			conn, err := normalizeConnection(alias, cfg)
			if err == nil && conn.Name == name {
				return conn, nil
			}
		}
	}
	return connection{Alias: name, Name: name, Type: "vpn"}, nil
}

func (v *VPN) resolveConfigured(alias string) (connection, error) {
	if v.config == nil || v.config.Connections == nil {
		return connection{}, fmt.Errorf("no vpn connections configured")
	}
	cfg, ok := v.config.Connections[alias]
	if !ok {
		return connection{}, fmt.Errorf("unknown vpn alias: %s", alias)
	}
	return normalizeConnection(alias, cfg)
}

func normalizeConnection(alias string, cfg config.VPNConnection) (connection, error) {
	if cfg.Name == "" {
		return connection{}, fmt.Errorf("vpn.%s.name is required", alias)
	}
	typ := cfg.Type
	if typ == "" {
		typ = "vpn"
	}
	profile := cfg.Profile
	if profile == "" {
		profile = fmt.Sprintf("~/.local/share/dotfiles/vpn/%s.nmconnection", alias)
	}
	return connection{
		Alias:   alias,
		Name:    cfg.Name,
		Type:    typ,
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
	return fmt.Sprintf("vpn connected: %s", label(conn)), nil
}

func (v *VPN) down(conn connection) (string, error) {
	if err := runNMCLI("connection", "down", conn.Name); err != nil {
		return "", err
	}
	return fmt.Sprintf("vpn disconnected: %s", label(conn)), nil
}

func (v *VPN) status(conn connection) (string, error) {
	active, err := v.active(conn.Name)
	if err != nil {
		return "", err
	}
	if active {
		return fmt.Sprintf("vpn connected: %s", label(conn)), nil
	}
	return fmt.Sprintf("vpn disconnected: %s", label(conn)), nil
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

	aliases := map[string]string{}
	if v.config != nil {
		for alias, cfg := range v.config.Connections {
			conn, err := normalizeConnection(alias, cfg)
			if err == nil {
				aliases[conn.Name] = alias
			}
		}
	}

	var lines []string
	for line := range strings.Lines(out) {
		name, typ, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok || typ != "vpn" {
			continue
		}
		if alias := aliases[name]; alias != "" {
			lines = append(lines, fmt.Sprintf("%s -> %s", alias, name))
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

func (v *VPN) install(conn connection, replace bool) (string, error) {
	if conn.Profile == "" {
		return "", fmt.Errorf("vpn.%s.profile is required", conn.Alias)
	}
	if _, err := os.Stat(conn.Profile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("profile not found: %s (run ./install.sh secrets or hyprd vpn export %s first)", conn.Profile, conn.Alias)
		}
		return "", err
	}
	if exists, err := connectionExists(conn.Name); err != nil {
		return "", err
	} else if exists {
		if !replace {
			return "", fmt.Errorf("connection already exists: %s (use --replace to re-import)", conn.Name)
		}
		if err := runNMCLI("connection", "delete", conn.Name); err != nil {
			return "", err
		}
	}

	if err := runSudoNMCLI("connection", "import", "type", conn.Type, "file", conn.Profile); err != nil {
		return "", err
	}
	return fmt.Sprintf("vpn installed: %s from %s", label(conn), conn.Profile), nil
}

func (v *VPN) export(conn connection) (string, error) {
	if conn.Profile == "" {
		return "", fmt.Errorf("vpn.%s.profile is required", conn.Alias)
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
	return fmt.Sprintf("vpn exported: %s\nprofile: %s\nnext: add '%s:%s:600' to etc/secrets/manifest, then run secrets sync", label(conn), conn.Profile, secretName(conn), manifestTarget(conn.Profile)), nil
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

func label(conn connection) string {
	if conn.Alias != "" && conn.Alias != conn.Name {
		return fmt.Sprintf("%s -> %s", conn.Alias, conn.Name)
	}
	return conn.Name
}

func secretName(conn connection) string {
	if conn.Alias == "" {
		return conn.Name + ".nmconnection"
	}
	return conn.Alias + "-vpn.nmconnection"
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

func runSudoNMCLI(args ...string) error {
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
