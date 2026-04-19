// Package daemon provides Unix-socket request/response and pub/sub primitives for local daemons.
//
// Responsibilities:
// - Serve command handlers over Unix domain sockets.
// - Manage topic subscriptions and event fan-out.
// - Expose client helpers for one-shot and streaming calls.
package daemon

// server.go defines the daemon socket server lifecycle and command dispatch loop.
import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

// CommandHandler processes a command string and returns a response.
type CommandHandler func(command string) string

// SubscribeHandler is called on new subscriptions to push initial state.
type SubscribeHandler func(sub *Subscriber, topics []string)

// Server owns a Unix socket, routing commands to a Handler and dispatching events via Subs.
type Server struct {
	SocketPath   string
	Subs         *SubscriptionManager
	Handler      CommandHandler
	OnSubscribe  SubscribeHandler
	done         chan struct{}
	listener     net.Listener
	shutdownOnce sync.Once
}

// NewServer returns a Server bound to socketPath.
func NewServer(socketPath string, handler CommandHandler) *Server {
	return &Server{
		SocketPath: socketPath,
		Subs:       NewSubscriptionManager(),
		Handler:    handler,
		done:       make(chan struct{}),
	}
}

// Start removes any stale socket, listens on SocketPath (chmod 0600), and spawns the accept loop.
func (s *Server) Start() error {
	if err := os.Remove(s.SocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

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

// Shutdown closes the listener, removes the socket file, and signals Done. Safe to call multiple times.
func (s *Server) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.done)
		if s.listener != nil {
			_ = s.listener.Close()
		}
		if err := os.Remove(s.SocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "remove socket %s: %v\n", s.SocketPath, err)
		}
	})
}

// Done returns a channel that closes when Shutdown is called.
func (s *Server) Done() <-chan struct{} {
	return s.done
}

// WaitForSignal blocks until SIGINT or SIGTERM and returns the signal received.
func (s *Server) WaitForSignal() os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	return <-sigCh
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
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
		conn.Read(buf) // block until client disconnects (EOF)
		s.Subs.Unsubscribe(conn)
		conn.Close()
		return
	}

	response := s.Handler(command)
	conn.Write([]byte(response))
	conn.Close()
}
