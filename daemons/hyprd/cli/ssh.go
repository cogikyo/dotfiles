package cli

// ssh.go implements PAM-driven secret loading: SSH keys via ssh-agent and gnome-keyring unlock.

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const pamLoadFlag = "hyprd-ssh-pam-load"

// SSH dispatches SSH subcommands (currently only pam-load).
func SSH() {
	if len(os.Args) < 3 || os.Args[2] != "pam-load" {
		fmt.Fprintln(os.Stderr, "usage: hyprd ssh pam-load")
		os.Exit(1)
	}
	pamLoad()
}

func pamLoad() {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	if _, err := os.Stat(filepath.Join(runtimeDir, pamLoadFlag)); err != nil {
		return
	}

	authtok, err := readPAMAuthToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd ssh pam-load: %v\n", err)
		os.Exit(1)
	}
	defer os.Unsetenv("_SSH_AUTHTOK")

	unlockKeyring(authtok, runtimeDir)

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd ssh pam-load: %v\n", err)
		os.Exit(1)
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd ssh pam-load: %v\n", err)
		os.Exit(1)
	}

	keys := []string{
		home + "/.ssh/cogikyo",
		home + "/.ssh/trend",
		home + "/.ssh/cullyn",
	}
	env := append(os.Environ(),
		"_SSH_AUTHTOK="+authtok,
		"HYPRD_SSH_ASKPASS=1",
		"SSH_ASKPASS="+exe,
		"SSH_ASKPASS_REQUIRE=force",
		"SSH_AUTH_SOCK="+runtimeDir+"/ssh-agent.socket",
	)
	if os.Getenv("DISPLAY") == "" {
		env = append(env, "DISPLAY=:0")
	}

	for _, key := range keys {
		if _, err := os.Stat(key); err != nil {
			continue
		}
		cmd := exec.Command("ssh-add", key)
		cmd.Env = env
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		_ = cmd.Run()
	}
}

func readPAMAuthToken() (string, error) {
	var b strings.Builder
	buf := make([]byte, 128)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			b.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	tok := strings.TrimRight(b.String(), "\r\n")
	if tok == "" {
		return "", fmt.Errorf("empty PAM auth token")
	}
	return tok, nil
}

// unlockKeyring unlocks the gnome-keyring login keyring via the daemon's control socket protocol.
func unlockKeyring(password, runtimeDir string) {
	conn, err := net.Dial("unix", filepath.Join(runtimeDir, "keyring", "control"))
	if err != nil {
		return
	}
	defer conn.Close()

	const opUnlock = 2
	buf := make([]byte, 12+len(password))
	binary.BigEndian.PutUint32(buf[0:], opUnlock)
	binary.BigEndian.PutUint32(buf[4:], 1)
	binary.BigEndian.PutUint32(buf[8:], uint32(len(password)))
	copy(buf[12:], password)
	conn.Write(buf)
}
