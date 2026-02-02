package commands

// ComputeGeometry calculates monitor geometry from dimensions, reserved areas, and monocle ratios.
func ComputeGeometry(width, height, reservedTop, reservedBot, reservedLeft int, monocleWidthRatio, monocleHeightRatio float64) *MonitorGeometry {
	usableH := height - reservedTop - reservedBot
	return &MonitorGeometry{
		Width:        width,
		Height:       height,
		ReservedTop:  reservedTop,
		ReservedBot:  reservedBot,
		ReservedLeft: reservedLeft,
		UsableHeight: usableH,
		MonocleW:     int(float64(width) * monocleWidthRatio),
		MonocleH:     int(float64(usableH) * monocleHeightRatio),
	}
}
