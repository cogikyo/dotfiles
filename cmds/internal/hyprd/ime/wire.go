package ime

import (
	"bufio"
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
)

const display uint32 = 1

type conn struct {
	sock *net.UnixConn
	r    *bufio.Reader
	next uint32
}

type event struct {
	object uint32
	opcode uint16
	body   []byte
}

func dial() (*conn, error) {
	path := cmp.Or(os.Getenv("WAYLAND_DISPLAY"), "wayland-0")
	if !filepath.IsAbs(path) {
		dir := os.Getenv("XDG_RUNTIME_DIR")
		if dir == "" {
			return nil, errors.New("XDG_RUNTIME_DIR not set")
		}
		path = filepath.Join(dir, path)
	}
	sock, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	return &conn{sock: sock, r: bufio.NewReader(sock), next: display + 1}, nil
}

func (c *conn) Close() error {
	return c.sock.Close()
}

func (c *conn) id() uint32 {
	id := c.next
	c.next++
	return id
}

func (c *conn) send(object uint32, opcode uint16, args ...any) error {
	var body []byte
	for _, arg := range args {
		switch v := arg.(type) {
		case uint32:
			body = binary.NativeEndian.AppendUint32(body, v)
		case string:
			body = binary.NativeEndian.AppendUint32(body, uint32(len(v)+1))
			body = append(body, v...)
			body = append(body, make([]byte, 4-len(v)%4)...)
		default:
			return fmt.Errorf("wayland: unsupported argument %T", arg)
		}
	}
	msg := binary.NativeEndian.AppendUint32(nil, object)
	msg = binary.NativeEndian.AppendUint32(msg, uint32(8+len(body))<<16|uint32(opcode))
	_, err := c.sock.Write(append(msg, body...))
	return err
}

func (c *conn) read() (event, error) {
	var head [8]byte
	if _, err := io.ReadFull(c.r, head[:]); err != nil {
		return event{}, err
	}
	word := binary.NativeEndian.Uint32(head[4:])
	size := word >> 16
	if size < 8 {
		return event{}, fmt.Errorf("wayland: message size %d", size)
	}
	e := event{object: binary.NativeEndian.Uint32(head[:4]), opcode: uint16(word), body: make([]byte, size-8)}
	_, err := io.ReadFull(c.r, e.body)
	return e, err
}

func (e event) uint(at int) (uint32, int, error) {
	if at+4 > len(e.body) {
		return 0, at, errors.New("wayland: short message")
	}
	return binary.NativeEndian.Uint32(e.body[at:]), at + 4, nil
}

func (e event) string(at int) (string, int, error) {
	n, at, err := e.uint(at)
	if err != nil {
		return "", at, err
	}
	if n == 0 {
		return "", at, nil
	}
	end := at + int(n)
	if end > len(e.body) {
		return "", at, errors.New("wayland: short string")
	}
	return string(e.body[at : end-1]), at + int((n+3)&^3), nil
}
