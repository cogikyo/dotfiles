package session

import (
	"fmt"
	"slices"
	"strings"

	"dotfiles/daemons/config"
)

var tabProfileOrder = map[string]int{"editor": 0, "agents": 1, "leadpier": 2}

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

func resolveTabAlias(alias, profileName string) string {
	if !strings.Contains(alias, ":") {
		return alias
	}

	parts := strings.Split(alias, ":")
	idx, ok := tabProfileOrder[profileName]
	if !ok || idx >= len(parts) {
		return parts[0]
	}
	name := parts[idx]
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
		if !tab.IsFocused {
			continue
		}
		for _, pane := range tab.Windows {
			if !pane.IsFocused {
				continue
			}
			return profileTabNameFromID(win.ID, profile.Prefix, pane.Env["KITTY_TAB_ID"])
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

	if hasProfileTab(profile, action) {
		return []string{action}
	}

	suffix := action
	if action == "git" && !hasProfileTab(profile, "git") {
		suffix = "build"
	}

	var matches []string
	for _, tab := range profile.Tabs {
		if strings.HasSuffix(tab.Name, "-"+suffix) {
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
	switch {
	case tabName == "nvim" || strings.HasSuffix(tabName, "-nvim"):
		actions = append(actions, "nvim")
	case tabName == "git":
		actions = append(actions, "git")
	case tabName == "build":
		actions = append(actions, "build")
	case strings.HasSuffix(tabName, "-build"):
		if hasProfileTab(profile, "git") {
			actions = append(actions, "build")
		} else {
			actions = append(actions, "git")
		}
	}
	return actions
}
