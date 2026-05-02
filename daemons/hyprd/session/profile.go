package session

// profile.go contains tab-profile detection, alias expansion, and semantic candidate selection helpers.

import (
	"fmt"
	"slices"
	"strings"

	"dotfiles/daemons/config"
)

// detectTabProfile infers the tab profile by matching KITTY_TAB_ID prefixes; falls back to "editor".
func detectTabProfile(cfg *config.HyprConfig, win KittyOSWindow) string {
	if cfg == nil || cfg.Tabs == nil {
		return "editor"
	}

	windowPrefix := fmt.Sprintf("%d-", win.ID)
	for _, tab := range win.Tabs {
		for _, pane := range tab.Windows {
			id := pane.Env["KITTY_TAB_ID"]
			if id == "" || !strings.HasPrefix(id, windowPrefix) {
				continue
			}

			suffix := id[len(windowPrefix):]
			for name, profile := range cfg.Tabs {
				if profile.Prefix != "" && strings.HasPrefix(suffix, profile.Prefix) {
					return name
				}
			}
		}
	}

	return "editor"
}

// resolveTabAlias picks the profile-specific name from a colon-separated alias using profile.Order.
func resolveTabAlias(cfg *config.HyprConfig, alias, profileName string) string {
	if !strings.Contains(alias, ":") {
		return alias
	}

	parts := strings.Split(alias, ":")
	if cfg == nil || cfg.Tabs == nil {
		return parts[0]
	}
	profile, ok := cfg.Tabs[profileName]
	if !ok || profile.Order >= len(parts) {
		return parts[0]
	}
	name := parts[profile.Order]
	if name == "" {
		return parts[0]
	}
	return name
}

func baseTabName(nameOrAlias string) string {
	name, _, ok := strings.Cut(nameOrAlias, ":")
	if !ok {
		return nameOrAlias
	}
	return name
}

func tabProfilePrefix(cfg *config.HyprConfig, profileName string) string {
	if cfg == nil || cfg.Tabs == nil {
		return ""
	}
	if profile, ok := cfg.Tabs[profileName]; ok {
		return profile.Prefix
	}
	return ""
}

func profileTab(cfg *config.HyprConfig, profileName, tabName string) *config.TabDef {
	if cfg == nil || cfg.Tabs == nil {
		return nil
	}
	profile, ok := cfg.Tabs[profileName]
	if !ok {
		return nil
	}
	for i := range profile.Tabs {
		if profile.Tabs[i].Name == tabName {
			return &profile.Tabs[i]
		}
	}
	return nil
}

func activeProfileTabName(cfg *config.HyprConfig, profileName string, win KittyOSWindow) string {
	profile, ok := cfg.Tabs[profileName]
	if !ok {
		return ""
	}

	for _, tab := range win.Tabs {
		if !tabSelected(tab) {
			continue
		}
		for _, pane := range tab.Windows {
			if !paneSelected(pane) {
				continue
			}
			name := profileTabNameFromID(win.ID, profile.Prefix, pane.Env["KITTY_TAB_ID"])
			return normalizeProfileTabName(&profile, name)
		}
	}
	return ""
}

func profileTabNameFromID(windowID int, prefix, tabID string) string {
	if prefix == "" || tabID == "" {
		return ""
	}
	fullPrefix := fmt.Sprintf("%d-%s", windowID, prefix)
	if !strings.HasPrefix(tabID, fullPrefix) {
		return ""
	}
	return tabID[len(fullPrefix):]
}

func normalizeProfileTabName(profile *config.TabProfile, tabName string) string {
	if profile == nil || tabName == "" {
		return tabName
	}
	if hasProfileTab(profile, tabName) {
		return tabName
	}
	if tabName == "term" && hasProfileTab(profile, "nvim") {
		return "nvim"
	}
	return tabName
}

func runtimeTabID(win KittyOSWindow, profile *config.TabProfile, targetTab string) string {
	if profile == nil {
		return ""
	}
	for _, tab := range win.Tabs {
		for _, pane := range tab.Windows {
			tabID := pane.Env["KITTY_TAB_ID"]
			if tabID == "" {
				continue
			}
			name := profileTabNameFromID(win.ID, profile.Prefix, tabID)
			if normalizeProfileTabName(profile, name) == targetTab {
				return tabID
			}
		}
	}
	return fmt.Sprintf("%d-%s%s", win.ID, profile.Prefix, targetTab)
}

func normalizeTabAction(nameOrAlias string) string {
	switch baseTabName(nameOrAlias) {
	case "nvimtree":
		return "nvim"
	default:
		return baseTabName(nameOrAlias)
	}
}

func semanticCandidates(profile *config.TabProfile, action string) []string {
	if profile == nil {
		return nil
	}

	var matches []string
	if hasProfileTab(profile, action) {
		matches = append(matches, action)
	}
	for _, tab := range profile.Tabs {
		if _, ok := tab.Actions[action]; ok && !slices.Contains(matches, tab.Name) {
			matches = append(matches, tab.Name)
		}
	}
	suffix := action
	if action == "git" && !hasProfileTab(profile, "git") {
		suffix = "build"
	}
	for _, tab := range profile.Tabs {
		if strings.HasSuffix(tab.Name, "-"+suffix) && !slices.Contains(matches, tab.Name) {
			matches = append(matches, tab.Name)
		}
	}
	return matches
}

func hasProfileTab(profile *config.TabProfile, name string) bool {
	return findProfileTab(profile, name) != nil
}

func findProfileTab(profile *config.TabProfile, name string) *config.TabDef {
	if profile == nil {
		return nil
	}
	for i := range profile.Tabs {
		if profile.Tabs[i].Name == name {
			return &profile.Tabs[i]
		}
	}
	return nil
}

// pickSemanticTab chooses among multiple candidates for an action, preferring remembered > current context > focus context.
func pickSemanticTab(profile *config.TabProfile, action, currentTab, rememberedTab, rememberedContext string) string {
	candidates := semanticCandidates(profile, action)
	if len(candidates) == 0 {
		return ""
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	if rememberedTab != "" && slices.Contains(candidates, rememberedTab) {
		return rememberedTab
	}

	for _, context := range []string{tabContext(currentTab), rememberedContext, tabContext(profile.Focus)} {
		if match := candidateWithContext(candidates, context); match != "" {
			return match
		}
	}
	return candidates[0]
}

func tabContext(tabName string) string {
	prefix, _, ok := strings.Cut(tabName, "-")
	if !ok {
		return ""
	}
	return prefix
}

func candidateWithContext(candidates []string, context string) string {
	if context == "" {
		return ""
	}
	prefix := context + "-"
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate, prefix) {
			return candidate
		}
	}
	return ""
}

func actionKeysForTab(profile *config.TabProfile, tabName string) []string {
	if tabName == "" {
		return nil
	}

	var actions []string
	if tab := findProfileTab(profile, tabName); tab != nil {
		for action := range tab.Actions {
			actions = append(actions, action)
		}
	}
	switch {
	case tabName == "nvim" || strings.HasSuffix(tabName, "-nvim"):
		if !slices.Contains(actions, "nvim") {
			actions = append(actions, "nvim")
		}
	case tabName == "git":
		if !slices.Contains(actions, "git") {
			actions = append(actions, "git")
		}
	case tabName == "build":
		if !slices.Contains(actions, "build") {
			actions = append(actions, "build")
		}
	case strings.HasSuffix(tabName, "-build"):
		if hasProfileTab(profile, "git") {
			if !slices.Contains(actions, "build") {
				actions = append(actions, "build")
			}
		} else {
			if !slices.Contains(actions, "git") {
				actions = append(actions, "git")
			}
		}
	}
	return actions
}

func tabAction(profile *config.TabProfile, tabName, action string) (config.TabAction, bool) {
	tab := findProfileTab(profile, tabName)
	if tab == nil || tab.Actions == nil {
		return config.TabAction{}, false
	}
	actionConfig, ok := tab.Actions[action]
	return actionConfig, ok
}
