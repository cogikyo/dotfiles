package windows

import (
	"strings"

	"dotfiles/daemons/hyprd/hypr"
)

// MatchesTarget returns true if a window matches the requested class/title criteria.
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
