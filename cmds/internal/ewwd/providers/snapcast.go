package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const snapcastStatusRequest = `{"id":1,"jsonrpc":"2.0","method":"Server.GetStatus"}` + "\n"

type SnapcastState struct {
	Available bool      `json:"available"`
	Speakers  []Speaker `json:"speakers"`
}

type Speaker struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Volume int    `json:"volume"`
	Muted  bool   `json:"muted"`
}

type Snapcast struct {
	state    StateSetter
	address  string
	stop     chan struct{}
	stopOnce sync.Once
	last     SnapcastState
}

func NewSnapcast(state StateSetter, address string) *Snapcast {
	return &Snapcast{state: state, address: address, stop: make(chan struct{}), last: SnapcastState{Speakers: []Speaker{}}}
}

func (s *Snapcast) Name() string {
	return "snapcast"
}

func (s *Snapcast) Stop() error {
	s.stopOnce.Do(func() { close(s.stop) })
	return nil
}

func (s *Snapcast) Start(ctx context.Context, notify func(data any)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-s.stop:
			cancel()
		case <-ctx.Done():
		}
	}()

	publish := func(next SnapcastState) {
		if next.Available == s.last.Available && slices.Equal(next.Speakers, s.last.Speakers) {
			return
		}
		s.last = next
		s.state.Set("snapcast", &next)
		notify(&next)
	}
	initial := s.last
	s.state.Set("snapcast", &initial)
	notify(&initial)
	if s.address == "" {
		<-ctx.Done()
		return nil
	}

	backoff := 250 * time.Millisecond
	for ctx.Err() == nil {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.address)
		if err == nil {
			err = followSnapcast(ctx, conn, func(server snapcastServer) {
				backoff = 250 * time.Millisecond
				publish(SnapcastState{Available: true, Speakers: server.speakers()})
			})
		}
		if ctx.Err() != nil {
			return nil
		}
		if s.last.Available {
			fmt.Fprintf(os.Stderr, "ewwd: snapcast: %v\n", err)
		}
		publish(SnapcastState{Speakers: []Speaker{}})
		if !waitContext(ctx, backoff) {
			return nil
		}
		backoff = min(backoff*2, 5*time.Second)
	}
	return nil
}

func (s *Snapcast) HandleAction(args []string) (string, error) {
	if len(args) != 3 {
		return "", errors.New("snapcast action requires: volume <client> <percent>, or mute <client> <true|false>")
	}
	var volume map[string]any
	switch args[0] {
	case "volume":
		percent, err := strconv.Atoi(args[2])
		if err != nil || percent < 0 || percent > 100 {
			return "", fmt.Errorf("invalid volume %q", args[2])
		}
		volume = map[string]any{"percent": percent}
	case "mute":
		muted, err := strconv.ParseBool(args[2])
		if err != nil {
			return "", fmt.Errorf("invalid mute %q", args[2])
		}
		volume = map[string]any{"muted": muted}
	default:
		return "", fmt.Errorf("unknown snapcast action: %s", args[0])
	}
	if err := s.call("Client.SetVolume", map[string]any{"id": args[1], "volume": volume}); err != nil {
		return "", err
	}
	return "ok", nil
}

func (s *Snapcast) call(method string, params any) error {
	if s.address == "" {
		return errors.New("audio.snapcast.rpc is unset")
	}
	conn, err := net.DialTimeout("tcp", s.address, 2*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return err
	}
	request := map[string]any{"id": 2, "jsonrpc": "2.0", "method": method, "params": params}
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return err
	}
	decoder := json.NewDecoder(conn)
	for {
		var reply struct {
			ID    json.RawMessage `json:"id"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := decoder.Decode(&reply); err != nil {
			return err
		}
		if reply.ID == nil {
			continue
		}
		if reply.Error != nil {
			return fmt.Errorf("%s: %s", method, reply.Error.Message)
		}
		return nil
	}
}

type snapcastServer struct {
	Groups []struct {
		Clients []struct {
			ID        string `json:"id"`
			Connected bool   `json:"connected"`
			Host      struct {
				Name string `json:"name"`
			} `json:"host"`
			Config struct {
				Name   string `json:"name"`
				Volume struct {
					Muted   bool `json:"muted"`
					Percent int  `json:"percent"`
				} `json:"volume"`
			} `json:"config"`
		} `json:"clients"`
	} `json:"groups"`
}

func (s snapcastServer) speakers() []Speaker {
	speakers := []Speaker{}
	for _, group := range s.Groups {
		for _, client := range group.Clients {
			if !client.Connected {
				continue
			}
			name := client.Config.Name
			if name == "" {
				name = client.Host.Name
			}
			speakers = append(speakers, Speaker{
				ID:     client.ID,
				Name:   name,
				Volume: client.Config.Volume.Percent,
				Muted:  client.Config.Volume.Muted,
			})
		}
	}
	return speakers
}

func followSnapcast(ctx context.Context, conn net.Conn, send func(snapcastServer)) error {
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	if _, err := io.WriteString(conn, snapcastStatusRequest); err != nil {
		return err
	}
	decoder := json.NewDecoder(conn)
	for {
		var message struct {
			Method string `json:"method"`
			Result *struct {
				Server snapcastServer `json:"server"`
			} `json:"result"`
		}
		if err := decoder.Decode(&message); err != nil {
			return err
		}
		switch {
		case message.Result != nil:
			send(message.Result.Server)
		case strings.HasPrefix(message.Method, "Client."), strings.HasPrefix(message.Method, "Group."), message.Method == "Server.OnUpdate":
			if _, err := io.WriteString(conn, snapcastStatusRequest); err != nil {
				return err
			}
		}
	}
}
