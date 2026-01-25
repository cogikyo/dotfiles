package commands

// Monitor geometry constants (3840x2160 with Hyprland gaps)
const (
	MonitorWidth   = 3840
	MonitorHeight  = 2160
	ReservedTop    = 86
	ReservedBottom = 32
	ReservedRight  = 85
	UsableHeight   = MonitorHeight - ReservedTop - ReservedBottom // 2042
)

// Monocle workspace and size
const (
	MonocleWS     = 6
	MonocleWidth  = 3190
	MonocleHeight = 1920
)

// Split ratio values (master factor)
const (
	SplitXS      = "0.37"
	SplitDefault = "0.4942"
	SplitLG      = "0.77"
)

// Border/shadow colors
const (
	BorderDefault = "rgb(f2a170)"
	ShadowDefault = "rgba(e56b2c32)"
	BorderMonocle = "rgb(5aba6d)"
	ShadowMonocle = "rgba(2d9a4342)"
)

// IgnoredClasses are window classes excluded from tiling operations.
var IgnoredClasses = []string{"GLava"}
