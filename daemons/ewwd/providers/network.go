package providers

// network.go reads link status from nmcli and computes traffic rates from sysfs counters.
import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"dotfiles/daemons/config"
)

type NetworkState struct {
	Type     string `json:"type"`
	Icon     string `json:"icon"`
	Name     string `json:"name"`
	Iface    string `json:"iface"`
	VPN      bool   `json:"vpn"`
	LinkRamp int    `json:"link_ramp"` // link-type bucket for widget color
	Down     int    `json:"down"`      // KB/s
	Up       int    `json:"up"`        // KB/s
	DownRamp int    `json:"down_ramp"` // 1-12 bucket
	UpRamp   int    `json:"up_ramp"`   // 1-12 bucket
	DownFmt  string `json:"down_fmt"`  // Pango-formatted with <sub> units
	UpFmt    string `json:"up_fmt"`    // Pango-formatted with <sub> units
}

type Network struct {
	state  StateSetter
	config config.NetworkConfig
	done   chan struct{}
	active bool

	prevRx int64
	prevTx int64
}

func NewNetwork(state StateSetter, cfg config.NetworkConfig) Provider {
	return &Network{
		state:  state,
		config: cfg,
		done:   make(chan struct{}),
	}
}

func (n *Network) Name() string {
	return "network"
}

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

func (n *Network) Stop() error {
	if n.active {
		close(n.done)
		n.active = false
	}
	return nil
}

func (n *Network) read() *NetworkState {
	out, err := exec.Command("nmcli", "-t", "-f", "TYPE,NAME,DEVICE", "connection", "show", "--active").Output()
	if err != nil {
		return &NetworkState{
			Type:     "none",
			Icon:     "󰖪",
			Name:     "error",
			Iface:    "lo",
			LinkRamp: 1,
			DownFmt:  "000<sub>K</sub>",
			UpFmt:    "000<sub>K</sub>",
			DownRamp: 1,
			UpRamp:   1,
		}
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

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

	if iface == "" {
		return &NetworkState{
			Type:     "none",
			Icon:     "󰖪",
			Name:     "disconnected",
			Iface:    "lo",
			VPN:      hasVPN,
			LinkRamp: 1,
			DownFmt:  "000<sub>K</sub>",
			UpFmt:    "000<sub>K</sub>",
			DownRamp: 1,
			UpRamp:   1,
		}
	}

	icon := ""
	name := iface
	if connType == "802-11-wireless" {
		icon = "󰖩"
		name = connName
	}

	rx := readInt64File(fmt.Sprintf("/sys/class/net/%s/statistics/rx_bytes", iface))
	tx := readInt64File(fmt.Sprintf("/sys/class/net/%s/statistics/tx_bytes", iface))

	// First poll has no prior sample; counter resets clamp to 0.
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
		LinkRamp: getLinkRamp(connType),
		Down:     downKB,
		Up:       upKB,
		DownRamp: getRamp(downKB),
		UpRamp:   getRamp(upKB),
		DownFmt:  fmtSpeed(downKB),
		UpFmt:    fmtSpeed(upKB),
	}
}

func getLinkRamp(connType string) int {
	switch connType {
	case "802-11-wireless":
		return 9
	case "802-3-ethernet":
		return 4
	default:
		return 1
	}
}

func readInt64File(path string) int64 {
	data := readFile(path)
	v, _ := strconv.ParseInt(strings.TrimSpace(data), 10, 64)
	return v
}

// getRamp buckets KB/s into a 1-12 scale for the bar widget.
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

// fmtSpeed renders KB/s with a Pango <sub> unit suffix.
func fmtSpeed(kb int) string {
	if kb < 1000 {
		return fmt.Sprintf("%03d<sub>K</sub>", kb)
	} else if kb < 10240 {
		return fmt.Sprintf("%04.1f<sub>M</sub>", float64(kb)/1024)
	} else {
		return fmt.Sprintf("%03.0f<sub>M</sub>", float64(kb)/1024)
	}
}
