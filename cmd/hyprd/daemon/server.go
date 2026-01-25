// Package daemon server.go handles client socket communication.
package daemon

import (
	"bufio"
	"io"
	"net"
	"os"
)

// SendCommand sends a command to the running daemon and returns the response.
func SendCommand(command string) (string, error) {
	conn, err := net.Dial("unix", SocketPath)
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

// StreamCommand sends a command and streams the response to stdout.
// Used for subscribe commands that keep the connection open.
func StreamCommand(command string) (string, error) {
	conn, err := net.Dial("unix", SocketPath)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(command)); err != nil {
		return "", err
	}

	// Stream response lines to stdout
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return "", nil
			}
			return "", err
		}
		os.Stdout.WriteString(line)
	}
}
