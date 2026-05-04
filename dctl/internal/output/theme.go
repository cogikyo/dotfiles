package output

import "charm.land/lipgloss/v2"

var (
	styleOK     = lipgloss.NewStyle().Background(lipgloss.Color("2")).Foreground(lipgloss.Color("0")).Padding(0, 1).Bold(true)
	styleInfo   = lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("0")).Padding(0, 1).Bold(true)
	styleWarn   = lipgloss.NewStyle().Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0")).Padding(0, 1).Bold(true)
	styleErr    = lipgloss.NewStyle().Background(lipgloss.Color("1")).Foreground(lipgloss.Color("0")).Padding(0, 1).Bold(true)
	styleStep   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	styleHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	styleDim    = lipgloss.NewStyle().Faint(true)
)
