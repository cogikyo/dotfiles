// Package cli implements CLI-only commands that run directly without the daemon socket.
package cli

// screenshot.go implements screenshot commands (wayfreeze + grim + satty).

import (
	"fmt"
	"os"
	"os/exec"
)

// Screenshot runs the region-screenshot flow: freeze, capture, then copy or annotate.
func Screenshot() {
	mode := "clipboard"
	if len(os.Args) > 2 {
		mode = os.Args[2]
	}
	if mode != "clipboard" && mode != "annotate" {
		fmt.Fprintln(os.Stderr, "usage: hyprd screenshot [annotate]")
		os.Exit(1)
	}

	imgPath, err := freezeAndCapture()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprd screenshot: %v\n", err)
		os.Exit(1)
	}

	var modeErr error
	switch mode {
	case "clipboard":
		modeErr = toClipboard(imgPath)
	case "annotate":
		modeErr = annotate(imgPath)
	}

	os.Remove(imgPath)
	if modeErr != nil {
		fmt.Fprintf(os.Stderr, "hyprd screenshot: %v\n", modeErr)
		os.Exit(1)
	}
}

// freezeAndCapture runs grim inside the wayfreeze hold so the capture matches the frozen frame.
func freezeAndCapture() (string, error) {
	f, err := os.CreateTemp("", "screenshot-*.png")
	if err != nil {
		return "", err
	}
	imgPath := f.Name()
	f.Close()

	afterCmd := fmt.Sprintf(`GEOM=$(slurp); [ -n "$GEOM" ] && grim -g "$GEOM" '%s'; kill $PPID`, imgPath)
	exec.Command("wayfreeze", "--after-freeze-cmd", afterCmd).Run()

	info, err := os.Stat(imgPath)
	if err != nil || info.Size() == 0 {
		os.Remove(imgPath)
		return "", fmt.Errorf("no region selected")
	}
	return imgPath, nil
}

func toClipboard(imgPath string) error {
	f, err := os.Open(imgPath)
	if err != nil {
		return err
	}
	defer f.Close()
	cmd := exec.Command("wl-copy", "--type", "image/png")
	cmd.Stdin = f
	return cmd.Run()
}

func annotate(imgPath string) error {
	return exec.Command("satty",
		"--filename", imgPath,
		"--copy-command", "wl-copy",
		"--actions-on-enter", "save-to-clipboard",
		"--actions-on-enter", "exit",
	).Run()
}
