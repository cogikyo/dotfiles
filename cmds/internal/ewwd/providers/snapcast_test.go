package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"sync"
	"testing"
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
