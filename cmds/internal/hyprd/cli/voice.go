package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dotfiles/cmds/internal/config"
)

func Voice() {
	// The bus wall's resume contract requires this scope name.
	cmd := exec.Command("systemd-run", "--user", "--scope", "--collect", "--unit=cogikyo-voice", "cogictl", "voice", "on")
	if organStatus("cochlea") == "listening" && organStatus("thalamus") != "hushed" {
		cmd = exec.Command("cogictl", "voice", "off")
	}
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil && bytes.Contains(stderr.Bytes(), []byte("cogikyo-voice.scope was already loaded")) {
		// An earlier resume still holds the scope; ignore the repeat.
		return
	}
	os.Stderr.Write(stderr.Bytes())
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd voice: %v\n", err)
		os.Exit(1)
	}
}

func organStatus(organ string) string {
	out, _ := exec.Command(filepath.Join(config.CogikyoBin, organ), "status").Output()
	return strings.TrimSpace(string(out))
}
