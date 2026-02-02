package providers

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"dotfiles/cmd/ewwd/config"
)

// NetworkState holds network speed and connection information.
type NetworkState struct {
	Type     string `json:"type"`
	Icon     string `json:"icon"`
	Name     string `json:"name"`
	Iface    string `json:"iface"`
	VPN      bool   `json:"vpn"`
	Down     int    `json:"down"`
	Up       int    `json:"up"`
	DownRamp int    `json:"down_ramp"`
	UpRamp   int    `json:"up_ramp"`
	DownFmt  string `json:"down_fmt"`
	UpFmt    string `json:"up_fmt"`
}

// Network provides network speed monitoring via nmcli and sysfs.
type Network struct {
	state  StateSetter
	config config.NetworkConfig
	done   chan struct{}
	active bool

	// Previous byte counts for calculating speed
	prevRx int64
	prevTx int64
}

// NewNetwork creates a Network provider.
func NewNetwork(state StateSetter, cfg config.NetworkConfig) Provider {
	return &Network{
		state:  state,
		config: cfg,
		done:   make(chan struct{}),
	}
}

// Name returns the provider identifier.
func (n *Network) Name() string {
	return "network"
}

// Start begins polling network statistics at configured intervals.
func (n *Network) Start(ctx context.Context, notify func(data any)) error {
	n.active = true
	ticker := time.NewTicker(n.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-n.done:
			return nil
		case <-ticker.C:
			if state := n.read(); state != nil {
				n.state.Set("network", state)
				notify(state)
			}
		}
	}
}

// Stop gracefully shuts down the network provider.
func (n *Network) Stop() error {
	if n.active {
		close(n.done)
		n.active = false
	}
	return nil
}

func (n *Network) read() *NetworkState {
	// Get active connections from nmcli
	out, err := exec.Command("nmcli", "-t", "-f", "TYPE,NAME,DEVICE", "connection", "show", "--active").Output()
	if err != nil {
		return &NetworkState{
			Type:     "none",
			Icon:     "󰖪",
			Name:     "error",
			Iface:    "lo",
			DownFmt:  "000<sub>K</sub>",
			UpFmt:    "000<sub>K</sub>",
			DownRamp: 1,
			UpRamp:   1,
		}
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	// Find ethernet or wireless connection
	var connType, connName, iface string
	var hasVPN bool

	for _, line := range lines {
		if strings.HasPrefix(line, "vpn") {
			hasVPN = true
			continue
		}
		if strings.Contains(line, "ethernet") || strings.Contains(line, "wireless") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				connType = parts[0]
				connName = parts[1]
				iface = parts[2]
				break
			}
		}
	}

	// Handle no connection
	if iface == "" {
		return &NetworkState{
			Type:     "none",
			Icon:     "󰖪",
			Name:     "disconnected",
			Iface:    "lo",
			VPN:      hasVPN,
			DownFmt:  "000<sub>K</sub>",
			UpFmt:    "000<sub>K</sub>",
			DownRamp: 1,
			UpRamp:   1,
		}
	}

	// Determine icon and display name
	icon := ""
	name := iface
	if connType == "802-11-wireless" {
		icon = "󰖩"
		name = connName
	}

	// Read byte counts
	rx := readInt64File(fmt.Sprintf("/sys/class/net/%s/statistics/rx_bytes", iface))
	tx := readInt64File(fmt.Sprintf("/sys/class/net/%s/statistics/tx_bytes", iface))

	// Calculate speed (KB/s)
	var downKB, upKB int
	if n.prevRx > 0 {
		downKB = int((rx - n.prevRx) / 1024)
		upKB = int((tx - n.prevTx) / 1024)
		if downKB < 0 {
			downKB = 0
		}
		if upKB < 0 {
			upKB = 0
		}
	}
	n.prevRx = rx
	n.prevTx = tx

	return &NetworkState{
		Type:     connType,
		Icon:     icon,
		Name:     name,
		Iface:    iface,
		VPN:      hasVPN,
		Down:     downKB,
		Up:       upKB,
		DownRamp: getRamp(downKB),
		UpRamp:   getRamp(upKB),
		DownFmt:  fmtSpeed(downKB),
		UpFmt:    fmtSpeed(upKB),
	}
}

func readInt64File(path string) int64 {
	data := readFile(path)
	v, _ := strconv.ParseInt(strings.TrimSpace(data), 10, 64)
	return v
}

// getRamp returns a value 1-12 based on KB/s speed.
func getRamp(kb int) int {
	switch {
	case kb < 5:
		return 1
	case kb < 20:
		return 2
	case kb < 50:
		return 3
	case kb < 300:
		return 4
	case kb < 500:
		return 5
	case kb < 1000:
		return 6
	case kb < 2500:
		return 7
	case kb < 5000:
		return 8
	case kb < 10000:
		return 9
	case kb < 25000:
		return 10
	case kb < 50000:
		return 11
	default:
		return 12
	}
}

// fmtSpeed formats KB/s into display string.
func fmtSpeed(kb int) string {
	if kb < 1000 {
		return fmt.Sprintf("%03d<sub>K</sub>", kb)
	} else if kb < 10240 {
		return fmt.Sprintf("%04.1f<sub>M</sub>", float64(kb)/1024)
	} else {
		return fmt.Sprintf("%03.0f<sub>M</sub>", float64(kb)/1024)
	}
}
