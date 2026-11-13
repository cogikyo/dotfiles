package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
)

type stateMap struct {
	mu   sync.Mutex
	data map[string]any
}

func (m *stateMap) Set(key string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

func (m *stateMap) get(key string) any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data[key]
}

func TestSnapcastInitialStateStaysPublished(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if _, err := bufio.NewReader(conn).ReadString('\n'); err != nil {
			return
		}
		io.WriteString(conn, `{"id":1,"jsonrpc":"2.0","result":{"server":{"groups":[{"clients":[{"id":"tv","connected":true,"config":{"name":"TV","volume":{"percent":50}}}]}]}}}`+"\n")
		io.Copy(io.Discard, conn)
	}()

	state := &stateMap{data: map[string]any{}}
	snapcast := NewSnapcast(state, listener.Addr().String())
	notified := make(chan struct{}, 2)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		snapcast.Start(ctx, func(any) { notified <- struct{}{} })
	}()
	defer func() {
		cancel()
		<-stopped
	}()

	<-notified
	initial := state.get("snapcast")
	want, err := json.Marshal(initial)
	if err != nil {
		t.Fatal(err)
	}
	for published := false; !published; {
		select {
		case <-notified:
			published = true
		default:
		}
		got, err := json.Marshal(initial)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("initial snapshot changed after publish: got %s, want %s", got, want)
		}
	}
}

type fakeSnapserver struct {
	notifySender bool
	setDelay     time.Duration
	stallConn    int

	mu      sync.Mutex
	percent int
	muted   bool
	volumes []int
	mutes   []bool
	conns   int
}

func (f *fakeSnapserver) start(t *testing.T) string {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			f.mu.Lock()
			f.conns++
			n := f.conns
			f.mu.Unlock()
			go f.serve(conn, n)
		}
	}()
	return listener.Addr().String()
}

func (f *fakeSnapserver) serve(conn net.Conn, n int) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		var request struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
			Params struct {
				Volume struct {
					Percent *int  `json:"percent"`
					Muted   *bool `json:"muted"`
				} `json:"volume"`
			} `json:"params"`
		}
		if err := json.Unmarshal(line, &request); err != nil {
			return
		}
		switch request.Method {
		case "Server.GetStatus":
			f.mu.Lock()
			percent, muted := f.percent, f.muted
			f.mu.Unlock()
			fmt.Fprintf(conn, `{"id":%d,"jsonrpc":"2.0","result":{"server":{"groups":[{"clients":[{"id":"tv","connected":true,"config":{"name":"TV","volume":{"percent":%d,"muted":%t}}}]}]}}}`+"\n", request.ID, percent, muted)
		case "Client.SetVolume":
			if n == f.stallConn {
				continue
			}
			if f.notifySender {
				io.WriteString(conn, `{"jsonrpc":"2.0","method":"Client.OnVolumeChanged","params":{}}`+"\n")
			}
			time.Sleep(f.setDelay)
			f.mu.Lock()
			if v := request.Params.Volume.Percent; v != nil {
				f.percent = *v
				f.volumes = append(f.volumes, *v)
			}
			if m := request.Params.Volume.Muted; m != nil {
				f.muted = *m
				f.mutes = append(f.mutes, *m)
			}
			f.mu.Unlock()
			fmt.Fprintf(conn, `{"id":%d,"jsonrpc":"2.0","result":{}}`+"\n", request.ID)
		}
	}
}

func runSnapcast(t *testing.T, address string, timeout time.Duration) (*Snapcast, <-chan Speaker) {
	published := make(chan Speaker, 256)
	snapcast := NewSnapcast(&stateMap{data: map[string]any{}}, address)
	if timeout > 0 {
		snapcast.timeout = timeout
	}
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		snapcast.Start(ctx, func(data any) {
			if state := data.(*SnapcastState); len(state.Speakers) == 1 {
				published <- state.Speakers[0]
			}
		})
	}()
	t.Cleanup(func() {
		cancel()
		<-stopped
	})
	return snapcast, published
}

func awaitSpeaker(t *testing.T, published <-chan Speaker, within time.Duration, done func(Speaker) bool) Speaker {
	t.Helper()
	deadline := time.After(within)
	for {
		select {
		case speaker := <-published:
			if done(speaker) {
				return speaker
			}
		case <-deadline:
			t.Fatalf("speaker state did not arrive within %s", within)
		}
	}
}

func TestSnapcastVolumeBurstNeverPublishesStaleReadback(t *testing.T) {
	server := &fakeSnapserver{notifySender: true, setDelay: 10 * time.Millisecond, percent: 80}
	snapcast, published := runSnapcast(t, server.start(t), 0)

	if got := (<-published).Volume; got != 80 {
		t.Fatalf("initial volume = %d, want 80", got)
	}
	for percent := 79; percent >= 60; percent-- {
		if _, err := snapcast.HandleAction([]string{"volume", "tv", strconv.Itoa(percent)}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	last := 80
	deadline := time.After(2 * time.Second)
	for last != 60 {
		select {
		case speaker := <-published:
			if speaker.Volume > last {
				t.Fatalf("published volume went back from %d to %d", last, speaker.Volume)
			}
			last = speaker.Volume
		case <-deadline:
			t.Fatalf("published volume settled at %d, want 60", last)
		}
	}
	select {
	case speaker := <-published:
		t.Fatalf("published %d after settling at 60", speaker.Volume)
	case <-time.After(100 * time.Millisecond):
	}

	server.mu.Lock()
	defer server.mu.Unlock()
	if server.percent != 60 {
		t.Fatalf("server volume = %d, want 60", server.percent)
	}
	if len(server.volumes) >= 20 {
		t.Fatalf("sent %d Client.SetVolume calls for 20 targets, want coalescing", len(server.volumes))
	}
}

func TestSnapcastMuteRefreshesWithoutSenderNotification(t *testing.T) {
	server := &fakeSnapserver{percent: 50}
	snapcast, published := runSnapcast(t, server.start(t), 0)

	speaker := awaitSpeaker(t, published, time.Second, func(Speaker) bool { return true })
	for range 2 {
		if _, err := snapcast.HandleAction([]string{"mute", "tv", strconv.FormatBool(!speaker.Muted)}); err != nil {
			t.Fatal(err)
		}
		want := !speaker.Muted
		speaker = awaitSpeaker(t, published, time.Second, func(s Speaker) bool { return s.Muted == want })
	}
	select {
	case got := <-published:
		if got.Muted != speaker.Muted {
			t.Fatalf("mute flipped back to %t after the readback", got.Muted)
		}
	case <-time.After(100 * time.Millisecond):
	}

	server.mu.Lock()
	defer server.mu.Unlock()
	if want := []bool{true, false}; !slices.Equal(server.mutes, want) {
		t.Fatalf("mute requests = %v, want %v", server.mutes, want)
	}
}

func TestSnapcastStalledVolumeRequestRecovers(t *testing.T) {
	server := &fakeSnapserver{stallConn: 1, percent: 50}
	snapcast, published := runSnapcast(t, server.start(t), 200*time.Millisecond)

	awaitSpeaker(t, published, time.Second, func(Speaker) bool { return true })
	if _, err := snapcast.HandleAction([]string{"volume", "tv", "40"}); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	for {
		server.mu.Lock()
		applied, conns := server.percent, server.conns
		server.mu.Unlock()
		if applied == 40 && conns >= 2 {
			break
		}
		if time.Since(start) > 2*time.Second {
			t.Fatalf("stalled request not recovered: server volume %d after %d connections", applied, conns)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := snapcast.HandleAction([]string{"volume", "tv", "41"}); err != nil {
		t.Fatal(err)
	}
	awaitSpeaker(t, published, 2*time.Second, func(s Speaker) bool { return s.Volume == 41 })
	for start := time.Now(); ; time.Sleep(10 * time.Millisecond) {
		server.mu.Lock()
		applied := server.percent
		server.mu.Unlock()
		if applied == 41 {
			break
		}
		if time.Since(start) > time.Second {
			t.Fatalf("server volume = %d after recovery, want 41", applied)
		}
	}
}
