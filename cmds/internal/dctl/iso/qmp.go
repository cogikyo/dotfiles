package iso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

type qmp struct {
	conn net.Conn
	dec  *json.Decoder
	dead error
}

type reply struct {
	Return json.RawMessage `json:"return"`
	Event  string          `json:"event"`
	Error  *struct {
		Class string `json:"class"`
		Desc  string `json:"desc"`
	} `json:"error"`
}

func dial(ctx context.Context, conn net.Conn) (*qmp, error) {
	q := &qmp{conn: conn, dec: json.NewDecoder(conn)}
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { conn.SetDeadline(time.Now()) })
	var greeting struct {
		QMP json.RawMessage `json:"QMP"`
	}
	err := q.dec.Decode(&greeting)
	stop()
	if err == nil && greeting.QMP == nil {
		err = errors.New("no greeting")
	}
	if err == nil {
		_, err = q.execute(ctx, "qmp_capabilities", nil)
	}
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("qmp: %w", err)
	}
	return q, nil
}

func (q *qmp) execute(ctx context.Context, command string, args any) (json.RawMessage, error) {
	if q.dead != nil {
		return nil, q.dead
	}
	deadline, _ := ctx.Deadline()
	q.conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { q.conn.SetDeadline(time.Now()) })
	msg, err := q.roundtrip(command, args)
	if !stop() && err == nil {
		err = ctx.Err()
	}
	if err != nil {
		q.dead = fmt.Errorf("qmp %s: %w", command, err)
		return nil, q.dead
	}
	q.conn.SetDeadline(time.Time{})
	if msg.Error != nil {
		return nil, fmt.Errorf("qmp %s: %s: %s", command, msg.Error.Class, msg.Error.Desc)
	}
	return msg.Return, nil
}

func (q *qmp) roundtrip(command string, args any) (reply, error) {
	req := map[string]any{"execute": command}
	if args != nil {
		req["arguments"] = args
	}
	if err := json.NewEncoder(q.conn).Encode(req); err != nil {
		return reply{}, err
	}
	for {
		var msg reply
		if err := q.dec.Decode(&msg); err != nil || msg.Event == "" {
			return msg, err
		}
	}
}
