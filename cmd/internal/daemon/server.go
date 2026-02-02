package daemon

// Core server lifecycle, socket handling, and accept loop

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// CommandHandler processes a command and returns a response.
type CommandHandler func(command string) string

// SubscribeHandler is called when a client subscribes.
// It should send initial state to the subscriber.
type SubscribeHandler func(sub *Subscriber, topics []string)

// Server manages the Unix socket and client connections.
type Server struct {
	SocketPath       string
	Subs             *SubscriptionManager
	Handler          CommandHandler
	OnSubscribe      SubscribeHandler
	done             chan struct{}
	listener         net.Listener
}

// NewServer creates a new daemon server.
func NewServer(socketPath string, handler CommandHandler) *Server {
	return &Server{
		SocketPath: socketPath,
		Subs:       NewSubscriptionManager(),
		Handler:    handler,
		done:       make(chan struct{}),
	}
}

// Start begins listening on the socket and accepting connections.
func (s *Server) Start() error {
	// Remove stale socket
	os.Remove(s.SocketPath)

	listener, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.SocketPath, err)
	}
	s.listener = listener

	// Make socket world-accessible (for eww, scripts, etc.)
	if err := os.Chmod(s.SocketPath, 0o666); err != nil {
		listener.Close()
		return fmt.Errorf("chmod socket: %w", err)
	}

	go s.acceptLoop()
	return nil
}

// Shutdown cleanly stops the server.
func (s *Server) Shutdown() {
	close(s.done)
	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.SocketPath)
}

// Done returns the done channel for coordinating shutdown.
func (s *Server) Done() <-chan struct{} {
	return s.done
}

// WaitForSignal blocks until SIGINT or SIGTERM is received.
func (s *Server) WaitForSignal() os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	return <-sigCh
}

// acceptLoop handles incoming client connections.
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

// handleClient processes a single client connection.
func (s *Server) handleClient(conn net.Conn) {
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		conn.Close()
		return
	}

	command := strings.TrimSpace(string(buf[:n]))

	// Handle subscribe specially - keep connection open
	if strings.HasPrefix(command, "subscribe") {
		topics := ParseSubscribeCommand(command)
		s.Subs.Subscribe(conn, topics, func(sub *Subscriber) {
			if s.OnSubscribe != nil {
				s.OnSubscribe(sub, topics)
			}
		})
		// Block until client disconnects
		buf := make([]byte, 1)
		conn.Read(buf)
		s.Subs.Unsubscribe(conn)
		conn.Close()
		return
	}

	// Normal request-response
	response := s.Handler(command)
	conn.Write([]byte(response))
	conn.Close()
}
