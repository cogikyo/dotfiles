package daemon

// Subscription manager for streaming state changes to clients

import (
	"encoding/json"
	"net"
	"strings"
	"sync"
)

// Subscriber represents a client subscribed to state changes.
type Subscriber struct {
	conn   net.Conn
	topics map[string]bool
	mu     sync.Mutex // protects conn writes
}

// SubscriptionManager handles client subscriptions.
type SubscriptionManager struct {
	mu          sync.RWMutex
	subscribers []*Subscriber
}

// NewSubscriptionManager creates a subscription manager.
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{}
}

// Subscribe adds a new subscriber for the given topics.
// The onSubscribe callback is called to send initial state.
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

// Unsubscribe removes a subscriber.
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

// Notify sends an event to all subscribers interested in the topic.
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

// SendEvent sends a single event to a subscriber.
func (sub *Subscriber) SendEvent(topic string, data any) {
	event := map[string]any{"event": topic, "data": data}
	if jsonData, err := json.Marshal(event); err == nil {
		sub.mu.Lock()
		sub.conn.Write(append(jsonData, '\n'))
		sub.mu.Unlock()
	}
}

// WantsTopic returns true if subscriber is interested in the topic.
func (sub *Subscriber) WantsTopic(topic string) bool {
	return sub.topics[topic] || sub.topics["*"]
}

// ParseSubscribeCommand parses "subscribe topic1 topic2 ..." into topics.
func ParseSubscribeCommand(cmd string) []string {
	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return []string{"*"} // Subscribe to all
	}
	return parts[1:]
}
