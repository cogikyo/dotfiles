package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SnapcastState struct {
	Available bool      `json:"available"`
	Speakers  []Speaker `json:"speakers"`
}

type Speaker struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Volume int    `json:"volume"`
	Muted  bool   `json:"muted"`
}

type Snapcast struct {
	state    StateSetter
	address  string
	actions  chan snapcastAction
	stop     chan struct{}
	stopped  chan struct{}
	stopOnce sync.Once
	last     SnapcastState
	timeout  time.Duration
	targets  *volumeTargets
	mutes    map[string]snapcastMute
}

type snapcastMute struct {
	muted bool
	seq   uint64
}

type snapcastAction struct {
	client  string
	percent int
	muted   *bool
	reply   chan error
}

type snapcastCall struct {
	client string
	seq    uint64
	mute   bool
	status bool
	sent   time.Time
}

type snapcastMessage struct {
	ID     *int            `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewSnapcast(state StateSetter, address string) *Snapcast {
	return &Snapcast{
		state:   state,
		address: address,
		actions: make(chan snapcastAction),
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
		last:    SnapcastState{Speakers: []Speaker{}},
		timeout: 2 * time.Second,
		targets: newVolumeTargets(),
		mutes:   map[string]snapcastMute{},
	}
}

func (s *Snapcast) Name() string {
	return "snapcast"
}

func (s *Snapcast) Stop() error {
	s.stopOnce.Do(func() { close(s.stop) })
	return nil
}

func (s *Snapcast) Start(ctx context.Context, notify func(data any)) error {
	defer close(s.stopped)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-s.stop:
			cancel()
		case <-ctx.Done():
		}
	}()

	backoff := 250 * time.Millisecond
	publish := func(next SnapcastState) {
		if next.Available {
			backoff = 250 * time.Millisecond
		}
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
		s.refuse(ctx, nil, errors.New("audio.snapcast.rpc is unset"))
		return nil
	}

	for ctx.Err() == nil {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", s.address)
		if err == nil {
			err = s.follow(ctx, conn, publish)
		}
		if ctx.Err() != nil {
			return nil
		}
		if s.last.Available {
			fmt.Fprintf(os.Stderr, "ewwd: snapcast: %v\n", err)
		}
		publish(SnapcastState{Speakers: []Speaker{}})
		timer := time.NewTimer(backoff)
		s.refuse(ctx, timer.C, errors.New("snapserver is unavailable"))
		timer.Stop()
		backoff = min(backoff*2, 5*time.Second)
	}
	return nil
}

func (s *Snapcast) refuse(ctx context.Context, until <-chan time.Time, err error) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-until:
			return
		case action := <-s.actions:
			action.reply <- err
		}
	}
}

func (s *Snapcast) follow(ctx context.Context, conn net.Conn, publish func(SnapcastState)) error {
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	quit := make(chan struct{})
	defer close(quit)
	messages := make(chan snapcastMessage)
	failed := make(chan error, 1)
	go func() {
		decoder := json.NewDecoder(conn)
		for {
			var message snapcastMessage
			if err := decoder.Decode(&message); err != nil {
				failed <- err
				return
			}
			select {
			case messages <- message:
			case <-quit:
				return
			}
		}
	}()

	session := &snapcastSession{Snapcast: s, conn: conn, publish: publish, calls: map[int]snapcastCall{}}
	if err := session.resume(); err != nil {
		return err
	}
	ticker := time.NewTicker(s.timeout / 4)
	defer ticker.Stop()
	for {
		select {
		case err := <-failed:
			return err
		case now := <-ticker.C:
			if err := session.expired(now); err != nil {
				return err
			}
		case action := <-s.actions:
			err := session.act(action)
			action.reply <- err
			if err != nil {
				return err
			}
		case message := <-messages:
			if err := session.handle(message); err != nil {
				return err
			}
		}
	}
}

type snapcastSession struct {
	*Snapcast
	conn    net.Conn
	publish func(SnapcastState)
	server  *snapcastServer
	calls   map[int]snapcastCall
	nextID  int
	muteSeq uint64
}

func (s *snapcastSession) resume() error {
	if err := s.status(); err != nil {
		return err
	}
	for _, client := range s.targets.requeue() {
		if err := s.sendTarget(client); err != nil {
			return err
		}
	}
	for client, mute := range s.mutes {
		if err := s.sendMute(client, mute.muted); err != nil {
			return err
		}
	}
	return nil
}

func (s *snapcastSession) expired(now time.Time) error {
	for _, call := range s.calls {
		if now.Sub(call.sent) > s.timeout {
			return fmt.Errorf("snapserver did not answer within %s", s.timeout)
		}
	}
	return nil
}

func (s *snapcastSession) act(action snapcastAction) error {
	defer s.show()
	if action.muted != nil {
		return s.sendMute(action.client, *action.muted)
	}
	if s.targets.set(action.client, "", action.percent) {
		return s.sendTarget(action.client)
	}
	return nil
}

func (s *snapcastSession) handle(message snapcastMessage) error {
	if message.ID == nil {
		if strings.HasPrefix(message.Method, "Client.") || strings.HasPrefix(message.Method, "Group.") || message.Method == "Server.OnUpdate" {
			return s.status()
		}
		return nil
	}
	call, ok := s.calls[*message.ID]
	if !ok {
		return nil
	}
	delete(s.calls, *message.ID)
	if call.status {
		var result struct {
			Server snapcastServer `json:"server"`
		}
		if message.Error != nil || json.Unmarshal(message.Result, &result) != nil {
			return errors.New("Server.GetStatus failed")
		}
		s.server = &result.Server
		s.show()
		return nil
	}
	if message.Error != nil {
		fmt.Fprintf(os.Stderr, "ewwd: snapcast: Client.SetVolume: %s\n", message.Error.Message)
	}
	if call.mute {
		if s.mutes[call.client].seq == call.seq {
			delete(s.mutes, call.client)
		}
		return s.status()
	}
	if s.targets.done(call.client, call.seq) {
		return s.sendTarget(call.client)
	}
	return s.status()
}

func (s *snapcastSession) show() {
	if s.server == nil {
		return
	}
	speakers := s.server.speakers()
	for i, speaker := range speakers {
		if percent, ok := s.targets.percent(speaker.ID); ok {
			speakers[i].Volume = percent
		}
		if mute, ok := s.mutes[speaker.ID]; ok {
			speakers[i].Muted = mute.muted
		}
	}
	s.publish(SnapcastState{Available: true, Speakers: speakers})
}

func (s *snapcastSession) status() error {
	return s.request("Server.GetStatus", nil, snapcastCall{status: true})
}

func (s *snapcastSession) sendTarget(client string) error {
	percent, _, seq := s.targets.next(client)
	return s.request("Client.SetVolume", map[string]any{"id": client, "volume": map[string]any{"percent": percent}}, snapcastCall{client: client, seq: seq})
}

func (s *snapcastSession) sendMute(client string, muted bool) error {
	s.muteSeq++
	s.mutes[client] = snapcastMute{muted: muted, seq: s.muteSeq}
	return s.request("Client.SetVolume", map[string]any{"id": client, "volume": map[string]any{"muted": muted}}, snapcastCall{client: client, seq: s.muteSeq, mute: true})
}

func (s *snapcastSession) request(method string, params any, call snapcastCall) error {
	s.nextID++
	call.sent = time.Now()
	s.calls[s.nextID] = call
	request := map[string]any{"id": s.nextID, "jsonrpc": "2.0", "method": method}
	if params != nil {
		request["params"] = params
	}
	if err := s.conn.SetWriteDeadline(call.sent.Add(s.timeout)); err != nil {
		return err
	}
	return json.NewEncoder(s.conn).Encode(request)
}

func (s *Snapcast) HandleAction(args []string) (string, error) {
	if len(args) != 3 {
		return "", errors.New("snapcast action requires: volume <client> <percent>, or mute <client> <true|false>")
	}
	action := snapcastAction{client: args[1], reply: make(chan error, 1)}
	switch args[0] {
	case "volume":
		percent, err := strconv.Atoi(args[2])
		if err != nil || percent < 0 || percent > 100 {
			return "", fmt.Errorf("invalid volume %q", args[2])
		}
		action.percent = percent
	case "mute":
		muted, err := strconv.ParseBool(args[2])
		if err != nil {
			return "", fmt.Errorf("invalid mute %q", args[2])
		}
		action.muted = &muted
	default:
		return "", fmt.Errorf("unknown snapcast action: %s", args[0])
	}
	select {
	case s.actions <- action:
	case <-s.stopped:
		return "", errors.New("snapcast provider is stopped")
	}
	if err := <-action.reply; err != nil {
		return "", err
	}
	return "ok", nil
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
				Kind:   speakerKind(client.Host.Name),
				Volume: client.Config.Volume.Percent,
				Muted:  client.Config.Volume.Muted,
			})
		}
	}
	return speakers
}

func speakerKind(host string) string {
	if strings.HasPrefix(host, "node-") {
		return "node"
	}
	if local, err := os.Hostname(); err == nil && host == local {
		return "tv"
	}
	return "speaker"
}
