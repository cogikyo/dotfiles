package browser

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (b *Browser) executeProfileRefresh(args []string) (string, error) {
	var (
		force      bool
		dryRun     bool
		positional []string
	)
	for _, arg := range args {
		switch {
		case arg == "--force":
			force = true
		case arg == "--dry-run":
			dryRun = true
		case strings.HasPrefix(arg, "-"):
			return "", fmt.Errorf(browserProfileUsage)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) != 1 {
		return "", fmt.Errorf(browserProfileUsage)
	}

	name := positional[0]
	slug, err := slugifySnapshotName(name)
	if err != nil {
		return "", err
	}
	if slug == mainFirefoxSnapshot {
		return "", fmt.Errorf("%q uses the main Firefox profile; refresh a layout profile instead", name)
	}
	dir, _, err := b.loadSnapshotSession(name)
	if err != nil {
		return "", err
	}
	main, err := discoverFirefoxProfile("")
	if err != nil {
		return "", err
	}
	main.Main = true
	profile, err := layoutFirefoxProfile(name, main, false)
	if err != nil {
		return "", err
	}
	payload, err := buildSessionPayload(dir)
	if err != nil {
		return "", err
	}

	if dryRun {
		return fmt.Sprintf("would refresh %s from %s\nwould inject %d bytes into %s", profile.Root, main.Root, len(payload), filepath.Join(profile.Root, "sessionstore.jsonlz4")), nil
	}
	if err := stopFirefoxProfile(profile, force); err != nil {
		return "", err
	}
	if err := refreshFirefoxProfile(main, profile); err != nil {
		return "", err
	}
	backupDir, err := injectSessionPayload(profile, payload)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("refreshed %s from %s\ninjected %d windows\nbackup: %s", profile.Root, main.Root, countPayloadWindows(payload), backupDir), nil
}
