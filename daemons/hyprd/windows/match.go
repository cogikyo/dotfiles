package windows

// match.go implements class/title matching used by focus, three-body, and session targeting flows.

import (
	"strings"

	"dotfiles/daemons/hyprd/hypr"
)

// MatchesTarget reports whether w matches class (case-insensitive, required) and title.
// Empty title matches any; otherwise title is exact-matched against Title or InitialTitle.
func MatchesTarget(w *hypr.Window, class, title string) bool {
	if class == "" {
		return false
	}
	if !strings.EqualFold(w.Class, class) {
		return false
	}
	if title == "" {
		return true
	}
	return w.Title == title || w.InitialTitle == title
}
