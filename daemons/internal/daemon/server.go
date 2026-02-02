// Package daemon provides shared infrastructure for Unix socket daemons.
//
// The package implements a client/server architecture for inter-process
// communication over Unix domain sockets. Daemons expose a socket that clients
// can connect to for sending commands and receiving responses.
//
// # Server
//
// Server manages the Unix socket lifecycle, accepting connections and routing
// commands to a user-provided handler. It handles graceful shutdown via signals
// and cleans up the socket file on exit.
//
//	server := daemon.NewServer("/tmp/my.sock", handleCommand)
//	server.Start()
//	server.WaitForSignal()
//	server.Shutdown()
//
// # Client
//
// Client provides methods for connecting to a running daemon. It supports
// one-shot request/response communication as well as streaming subscriptions.
//
//	client := daemon.NewClient("/tmp/my.sock")
//	response, err := client.Send("status")
//
// # Subscriptions
//
// The subscription mechanism allows clients to receive real-time updates from
// the daemon. Clients subscribe to topics and receive JSON events when the
// daemon calls Notify on the SubscriptionManager.
//
//	// Server side
//	server.Subs.Notify("window", windowData)
//
//	// Client side
//	client.Stream("subscribe window workspace")
package daemon

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// CommandHandler processes a command string and returns a response.
type CommandHandler func(command string) string

// SubscribeHandler is called when a client subscribes to topics.
// Implementations should send initial state to the subscriber.
type SubscribeHandler func(sub *Subscriber, topics []string)

// Server handles incoming daemon connections on a Unix socket.
type Server struct {
	SocketPath  string
	Subs        *SubscriptionManager
	Handler     CommandHandler
	OnSubscribe SubscribeHandler
	done        chan struct{}
	listener    net.Listener
}

// NewServer returns a new Server configured to listen on the given socket path.
// The handler is invoked for each non-subscribe command received.
func NewServer(socketPath string, handler CommandHandler) *Server {
	return &Server{
		SocketPath: socketPath,
		Subs:       NewSubscriptionManager(),
		Handler:    handler,
		done:       make(chan struct{}),
	}
}

// Start begins listening on the socket and accepting connections.
// It removes any stale socket file and restricts access to the current user.
func (s *Server) Start() error {
	os.Remove(s.SocketPath)

	listener, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.SocketPath, err)
	}
	s.listener = listener

	if err := os.Chmod(s.SocketPath, 0o600); err != nil {
		listener.Close()
		return fmt.Errorf("chmod socket: %w", err)
	}

	go s.acceptLoop()
	return nil
}

// Shutdown stops the server, closes the listener, and removes the socket file.
func (s *Server) Shutdown() {
	close(s.done)
	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.SocketPath)
}

// Done returns a channel that is closed when the server shuts down.
func (s *Server) Done() <-chan struct{} {
	return s.done
}

// WaitForSignal blocks until SIGINT or SIGTERM is received and returns the signal.
func (s *Server) WaitForSignal() os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	return <-sigCh
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				fmt.Fprintf(os.Stderr, "accept error: %v\n", err)
				continue
			}
		}
		go s.handleClient(conn)
	}
}

func (s *Server) handleClient(conn net.Conn) {
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		conn.Close()
		return
	}

	command := strings.TrimSpace(string(buf[:n]))

	if strings.HasPrefix(command, "subscribe") {
		topics := ParseSubscribeCommand(command)
		s.Subs.Subscribe(conn, topics, func(sub *Subscriber) {
			if s.OnSubscribe != nil {
				s.OnSubscribe(sub, topics)
			}
		})
		buf := make([]byte, 1)
		conn.Read(buf)
		s.Subs.Unsubscribe(conn)
		conn.Close()
		return
	}

	response := s.Handler(command)
	conn.Write([]byte(response))
	conn.Close()
}
