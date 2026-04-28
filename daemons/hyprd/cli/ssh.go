package cli

// ssh.go implements PAM-driven secret loading: SSH keys via ssh-agent and gnome-keyring unlock.

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const pamLoadFlag = "hyprd-ssh-pam-load"
const debugLogPath = "/tmp/hyprd-pam-debug.log"

// SSH dispatches SSH subcommands (currently only pam-load).
func SSH() {
	if len(os.Args) < 3 || os.Args[2] != "pam-load" {
		fmt.Fprintln(os.Stderr, "usage: hyprd ssh pam-load")
		os.Exit(1)
	}
	pamLoad()
}

func debugLog(format string, args ...any) {
	f, err := os.OpenFile(debugLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] ", time.Now().Format("15:04:05.000"))
	fmt.Fprintf(f, format, args...)
	fmt.Fprintln(f)
}

func pamLoad() {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	debugLog("=== pam-load invoked ===")
	debugLog("argv=%v pid=%d ppid=%d uid=%d", os.Args, os.Getpid(), os.Getppid(), os.Getuid())
	debugLog("runtimeDir=%s flag-exists=%v", runtimeDir, fileExists(filepath.Join(runtimeDir, pamLoadFlag)))
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "PAM_") || strings.HasPrefix(e, "HYPRD_") {
			debugLog("env: %s", e)
		}
	}
	if _, err := os.Stat(filepath.Join(runtimeDir, pamLoadFlag)); err != nil {
		debugLog("flag missing, returning quietly: %v", err)
		return
	}

	authtok, err := readPAMAuthToken()
	if err != nil {
		debugLog("readPAMAuthToken error: %v", err)
		fmt.Fprintf(os.Stderr, "hyprd ssh pam-load: %v\n", err)
		os.Exit(1)
	}
	debugLog("authtok received: len=%d", len(authtok))

	unlockKeyring(authtok, runtimeDir)

	// pam_exec runs without $HOME; resolve via uid lookup instead.
	home, err := resolveHome()
	if err != nil {
		debugLog("resolveHome error: %v", err)
		fmt.Fprintf(os.Stderr, "hyprd ssh pam-load: %v\n", err)
		os.Exit(1)
	}
	debugLog("home=%s", home)

	keys := []string{
		home + "/.ssh/cogikyo",
		home + "/.ssh/trend",
		home + "/.ssh/cullyn",
	}
	env := append(os.Environ(),
		"SSH_ASKPASS_REQUIRE=never",
		"SSH_AUTH_SOCK="+runtimeDir+"/ssh-agent.socket",
	)

	for _, key := range keys {
		if _, err := os.Stat(key); err != nil {
			debugLog("skip key (stat err): %s: %v", key, err)
			continue
		}
		// pam_exec strips PATH; use absolute path.
		cmd := exec.Command("/usr/bin/ssh-add", key)
		cmd.Env = env
		cmd.Stdin = strings.NewReader(authtok + "\n")
		var outBuf, errBuf strings.Builder
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		runErr := cmd.Run()
		debugLog("ssh-add %s: err=%v stdout=%q stderr=%q", key, runErr, outBuf.String(), errBuf.String())
	}
	debugLog("=== pam-load complete ===")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func resolveHome() (string, error) {
	if h := os.Getenv("HOME"); h != "" {
		return h, nil
	}
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}

func readPAMAuthToken() (string, error) {
	raw, err := io.ReadAll(os.Stdin)
	debugLog("stdin: read=%d bytes err=%v", len(raw), err)
	if len(raw) > 0 {
		preview := raw
		if len(preview) > 64 {
			preview = preview[:64]
		}
		debugLog("stdin hex: %s", hex.EncodeToString(preview))
		debugLog("stdin trailing-byte=0x%02x", raw[len(raw)-1])
	}
	s := string(raw)
	tok := strings.TrimRight(s, "\r\n\x00")
	debugLog("after trim: len=%d", len(tok))
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

	// Credential byte — server verifies UID via SO_PEERCRED.
	conn.Write([]byte{0})

	const opUnlock = 1
	packetLen := 4 + 4 + 4 + len(password)
	buf := make([]byte, packetLen)
	binary.BigEndian.PutUint32(buf[0:], uint32(packetLen))
	binary.BigEndian.PutUint32(buf[4:], opUnlock)
	binary.BigEndian.PutUint32(buf[8:], uint32(len(password)))
	copy(buf[12:], password)
	conn.Write(buf)

	var resp [8]byte
	conn.Read(resp[:])
}
