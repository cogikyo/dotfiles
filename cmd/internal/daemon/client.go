package daemon

import (
	"bufio"
	"io"
	"net"
	"os"
)

// Client connects to a running daemon instance via Unix socket.
type Client struct {
	SocketPath string
}

// NewClient returns a new Client configured to connect to the given socket path.
func NewClient(socketPath string) *Client {
	return &Client{SocketPath: socketPath}
}

// Send transmits a command to the daemon and returns the response.
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

// Stream sends a command and continuously writes response lines to stdout.
// It keeps the connection open until the server closes it, making it suitable
// for subscribe commands that receive ongoing events.
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

// IsRunning reports whether a daemon is listening on the socket.
// It attempts a ping/pong exchange to verify the daemon is responsive.
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
