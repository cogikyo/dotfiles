package daemon

import (
	"bufio"
	"io"
	"net"
	"os"
	"time"
)

// Client dials a daemon's Unix socket for one-shot or streaming commands.
type Client struct {
	SocketPath string
}

// NewClient returns a Client bound to socketPath. The socket is not dialed until use.
func NewClient(socketPath string) *Client {
	return &Client{SocketPath: socketPath}
}

// Send dials the daemon, writes command, and returns the first response read.
//
// Read deadline is 15s and the response is capped at 64 KiB. Use Stream for long-running or larger payloads.
func (c *Client) Send(command string) (string, error) {
	conn, err := net.Dial("unix", c.SocketPath)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(command)); err != nil {
		return "", err
	}

	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	buf := make([]byte, 64*1024)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}

	return string(buf[:n]), nil
}

// Stream sends command and copies newline-delimited response lines to stdout until the server closes.
//
// Intended for subscribe commands.
func (c *Client) Stream(command string) error {
	conn, err := net.Dial("unix", c.SocketPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(command)); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		os.Stdout.WriteString(line)
	}
}

// IsRunning returns true when the daemon answers "ping" with "pong".
//
// Dial or read failures are treated as not-running.
func (c *Client) IsRunning() bool {
	conn, err := net.Dial("unix", c.SocketPath)
	if err != nil {
		return false
	}
	defer conn.Close()

	conn.Write([]byte("ping"))
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return false
	}
	return string(buf[:n]) == "pong"
}
