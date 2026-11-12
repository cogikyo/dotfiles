package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

const snapcastStatusRequest = `{"id":1,"jsonrpc":"2.0","method":"Server.GetStatus"}` + "\n"

type snapcastServer struct {
	Groups []struct {
		Muted   bool `json:"muted"`
		Clients []struct {
			Connected bool `json:"connected"`
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

func (s snapcastServer) speakers() string {
	var names []string
	for _, group := range s.Groups {
		if group.Muted {
			continue
		}
		for _, client := range group.Clients {
			if !client.Connected || client.Config.Volume.Muted || client.Config.Volume.Percent == 0 {
				continue
			}
			name := client.Config.Name
			if name == "" {
				name = client.Host.Name
			}
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "No speakers"
	}
	return strings.Join(names, " + ")
}

func subscribeSnapcast(ctx context.Context, address string, labels chan<- string) {
	last := ""
	send := func(label string) {
		if label == last {
			return
		}
		last = label
		select {
		case labels <- label:
		case <-ctx.Done():
		}
	}
	backoff := 250 * time.Millisecond
	for ctx.Err() == nil {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
		if err == nil {
			err = followSnapcast(ctx, conn, func(label string) {
				backoff = 250 * time.Millisecond
				send(label)
			})
		}
		if ctx.Err() != nil {
			return
		}
		if last != "" {
			fmt.Fprintf(os.Stderr, "ewwd: audio: snapcast: %v\n", err)
		}
		send("")
		if !waitContext(ctx, backoff) {
			return
		}
		backoff = min(backoff*2, 5*time.Second)
	}
}

func followSnapcast(ctx context.Context, conn net.Conn, send func(string)) error {
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
			send(message.Result.Server.speakers())
		case strings.HasPrefix(message.Method, "Client."), strings.HasPrefix(message.Method, "Group."), message.Method == "Server.OnUpdate":
			if _, err := io.WriteString(conn, snapcastStatusRequest); err != nil {
				return err
			}
		}
	}
}
