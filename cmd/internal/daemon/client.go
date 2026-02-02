package daemon

// Client-side socket communication for sending commands to daemon

import (
	"bufio"
	"io"
	"net"
	"os"
)

// Client sends commands to a daemon socket.
type Client struct {
	SocketPath string
}

// NewClient creates a client for the given socket path.
func NewClient(socketPath string) *Client {
	return &Client{SocketPath: socketPath}
}

// Send sends a command and returns the response.
func (c *Client) Send(command string) (string, error) {
	conn, err := net.Dial("unix", c.SocketPath)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(command)); err != nil {
		return "", err
	}

	buf := make([]byte, 64*1024)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}

	return string(buf[:n]), nil
}

// Stream sends a command and streams the response to stdout.
// Used for subscribe commands that keep the connection open.
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

// IsRunning checks if a daemon is running on the socket.
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
