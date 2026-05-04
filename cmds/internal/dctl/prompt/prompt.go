// Package prompt contains the small interactive prompts allowed by dctl.
//
// Responsibilities:
// - Refuse confirmations without a real terminal.
// - Provide deterministic defaults for non-interactive selection.
// - Keep hidden passphrase input out of command packages.
package prompt

// prompt.go defines terminal detection, confirmations, selection, and hidden input.

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

func Interactive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func Confirm(question string, defaultYes bool) (bool, error) {
	if !Interactive() {
		return false, fmt.Errorf("confirmation requires an interactive terminal")
	}
	suffix := "[y/N]"
	if defaultYes {
		suffix = "[Y/n]"
	}
	fmt.Fprintf(os.Stdout, "  ?  %s %s ", question, suffix)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultYes, nil
	}
	return strings.HasPrefix(strings.ToLower(line), "y"), nil
}

func Hidden(question string) (string, error) {
	fmt.Fprint(os.Stdout, question)
	data, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stdout)
	return string(data), err
}
