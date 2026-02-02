package daemon

import (
	"encoding/json"
	"net"
	"strings"
	"sync"
)

// Subscriber represents a client connection receiving events for subscribed topics.
type Subscriber struct {
	conn   net.Conn         // Client connection
	topics map[string]bool  // Topics the client is subscribed to
	mu     sync.Mutex       // Protects concurrent writes to conn
}

// SubscriptionManager coordinates event delivery to active subscribers.
type SubscriptionManager struct {
	mu          sync.RWMutex   // Protects subscribers slice
	subscribers []*Subscriber  // Active client subscriptions
}

// NewSubscriptionManager creates a SubscriptionManager.
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{}
}

// Subscribe registers conn to receive events for topics, then calls onSubscribe to send initial state.
func (m *SubscriptionManager) Subscribe(conn net.Conn, topics []string, onSubscribe func(sub *Subscriber)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	topicMap := make(map[string]bool)
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

// Unsubscribe removes the subscriber associated with the given connection.
func (m *SubscriptionManager) Unsubscribe(conn net.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, sub := range m.subscribers {
		if sub.conn == conn {
			m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
			return
		}
	}
}

// Notify broadcasts a JSON event to all subscribers interested in topic.
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

// SendEvent writes a JSON event to the subscriber's connection.
func (sub *Subscriber) SendEvent(topic string, data any) {
	event := map[string]any{"event": topic, "data": data}
	if jsonData, err := json.Marshal(event); err == nil {
		sub.mu.Lock()
		sub.conn.Write(append(jsonData, '\n'))
		sub.mu.Unlock()
	}
}

// WantsTopic reports whether the subscriber is interested in the given topic.
func (sub *Subscriber) WantsTopic(topic string) bool {
	return sub.topics[topic] || sub.topics["*"]
}

// ParseSubscribeCommand extracts topics from cmd, defaulting to ["*"] if none specified.
func ParseSubscribeCommand(cmd string) []string {
	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return []string{"*"} // Subscribe to all
	}
	return parts[1:]
}
