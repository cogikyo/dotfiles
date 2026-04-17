package daemon

import (
	"encoding/json"
	"net"
	"slices"
	"strings"
	"sync"
)

// Subscriber is a single client connection receiving events for a topic set.
//
// mu serializes writes on conn so Notify and SendEvent can safely race.
type Subscriber struct {
	conn   net.Conn
	topics map[string]bool
	mu     sync.Mutex
}

// SubscriptionManager fans out events to active Subscribers. Safe for concurrent use.
type SubscriptionManager struct {
	mu          sync.RWMutex
	subscribers []*Subscriber
}

// NewSubscriptionManager returns an empty SubscriptionManager.
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{}
}

// Subscribe registers conn for topics and invokes onSubscribe (if non-nil) under the manager lock.
//
// Holding the lock guarantees initial-state emission happens before any Notify can race it.
func (m *SubscriptionManager) Subscribe(conn net.Conn, topics []string, onSubscribe func(sub *Subscriber)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	topicMap := make(map[string]bool, len(topics))
	for _, t := range topics {
		topicMap[t] = true
	}

	sub := &Subscriber{
		conn:   conn,
		topics: topicMap,
	}
	m.subscribers = append(m.subscribers, sub)

	if onSubscribe != nil {
		onSubscribe(sub)
	}
}

// Unsubscribe removes the subscriber bound to conn. No-op if not found.
func (m *SubscriptionManager) Unsubscribe(conn net.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, sub := range m.subscribers {
		if sub.conn == conn {
			m.subscribers = slices.Delete(m.subscribers, i, i+1)
			return
		}
	}
}

// Notify broadcasts {event: topic, data: data} as a newline-terminated JSON frame to matching subscribers.
//
// Matches are subscribers whose topic set contains topic or "*".
// Marshal and write errors are swallowed; dead connections are evicted later by Unsubscribe, not here.
func (m *SubscriptionManager) Notify(topic string, data any) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	event := map[string]any{
		"event": topic,
		"data":  data,
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		return
	}
	jsonData = append(jsonData, '\n')

	for _, sub := range m.subscribers {
		if sub.topics[topic] || sub.topics["*"] {
			sub.mu.Lock()
			sub.conn.Write(jsonData)
			sub.mu.Unlock()
		}
	}
}

// SendEvent writes a single newline-terminated JSON event directly to sub.
//
// Intended for initial-state pushes from OnSubscribe callbacks.
func (sub *Subscriber) SendEvent(topic string, data any) {
	event := map[string]any{"event": topic, "data": data}
	if jsonData, err := json.Marshal(event); err == nil {
		sub.mu.Lock()
		sub.conn.Write(append(jsonData, '\n'))
		sub.mu.Unlock()
	}
}

// WantsTopic reports whether sub is subscribed to topic (or the "*" wildcard).
func (sub *Subscriber) WantsTopic(topic string) bool {
	return sub.topics[topic] || sub.topics["*"]
}

// ParseSubscribeCommand extracts topics from a "subscribe [topic...]" command.
//
// Returns ["*"] when no topics are given.
func ParseSubscribeCommand(cmd string) []string {
	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return []string{"*"}
	}
	return parts[1:]
}
