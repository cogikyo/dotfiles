package session

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"dotfiles/daemons/hyprd/hypr"
	"dotfiles/daemons/hyprd/state"
)

type NotifyFunc func(app, urgency, title, body string)

type Init struct {
	hypr   *hypr.Client
	state  *state.State
	notify NotifyFunc
}

func NewInit(h *hypr.Client, s *state.State) *Init {
	return &Init{hypr: h, state: s}
}

func (i *Init) SetNotify(fn NotifyFunc) {
	i.notify = fn
}

func (i *Init) Execute() (string, error) {
	cfg := i.state.GetConfig()
	init := cfg.Init

	EnsureBG(&cfg.Background)
	fmt.Println("hyprd init: background ready")

	if init.NetworkTimeout > 0 {
		if ok := i.waitNetwork(init.NetworkTimeout); ok {
			fmt.Println("hyprd init: network ready")
		}
	}

	layout := NewLayout(i.hypr, i.state)
	for _, name := range init.Sessions {
		result, err := layout.Execute(name)
		if err != nil {
			fmt.Printf("hyprd init: session %s: %v\n", name, err)
		} else {
			fmt.Printf("hyprd init: %s\n", result)
		}
	}

	for _, cmd := range init.Execs {
		i.hypr.Dispatch(fmt.Sprintf("exec %s", cmd))
		time.Sleep(200 * time.Millisecond)
	}

	if init.Workspace > 0 {
		i.hypr.Dispatch(fmt.Sprintf("workspace %d", init.Workspace))
	}

	if init.Greeting != "" && i.notify != nil {
		hostname, _ := os.Hostname()
		user := os.Getenv("USER")
		i.notify("attention", "low", fmt.Sprintf("%s@%s", user, hostname), init.Greeting)
	}

	if init.Lock {
		fmt.Println("hyprd init: locking screen")
		exec.Command("hyprlock").Run()
	}

	fmt.Println("hyprd init: complete")
	return "init: complete", nil
}

func (i *Init) waitNetwork(timeout int) bool {
	for range timeout {
		conn, err := net.DialTimeout("tcp", "1.1.1.1:53", time.Second)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(time.Second)
	}
	if i.notify != nil {
		i.notify("attention", "critical", "Network", fmt.Sprintf("Failed to connect after %ds", timeout))
	}
	return false
}
