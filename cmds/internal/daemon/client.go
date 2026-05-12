package daemon

// client.go implements a Unix socket client for request/response and event streaming.
import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"syscall"
	"time"
)

// Client dials a daemon's Unix socket for one-shot or streaming commands.
type Client struct {
	SocketPath string
}

// NewClient returns a Client bound to socketPath.
func NewClient(socketPath string) *Client {
	return &Client{SocketPath: socketPath}
}

// Send writes command and returns the first response (15s deadline, 64 KiB cap).
func (c *Client) Send(command string) (string, error) {
	conn, err := net.Dial("unix", c.SocketPath)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(command)); err != nil {
		return "", err
	}

	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return "", err
	}
	buf := make([]byte, 64*1024)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}

	return string(buf[:n]), nil
}

// Stream sends command and copies newline-delimited response lines to stdout until EOF.
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
		if _, err := os.Stdout.WriteString(line); err != nil {
			if errors.Is(err, syscall.EPIPE) {
				return nil
			}
			return err
		}
	}
}

// IsRunning returns true when the daemon answers "ping" with "pong".
func (c *Client) IsRunning() bool {
	conn, err := net.Dial("unix", c.SocketPath)
	if err != nil {
		return false
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return false
	}
	if _, err := conn.Write([]byte("ping")); err != nil {
		return false
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return false
	}
	return string(buf[:n]) == "pong"
}
