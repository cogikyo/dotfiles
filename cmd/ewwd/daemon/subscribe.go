package daemon

// ================================================================================
// Subscription manager for streaming state changes to clients
// ================================================================================

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
	state       *State
}

// NewSubscriptionManager creates a subscription manager.
func NewSubscriptionManager(state *State) *SubscriptionManager {
	return &SubscriptionManager{
		state: state,
	}
}

// Subscribe adds a new subscriber for the given topics.
func (m *SubscriptionManager) Subscribe(conn net.Conn, topics []string) {
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

	// Send initial state for subscribed topics
	m.sendInitialState(sub)
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

// sendInitialState sends current state for subscribed topics.
func (m *SubscriptionManager) sendInitialState(sub *Subscriber) {
	allState := m.state.GetAll()

	for topic, data := range allState {
		if data != nil && (sub.topics[topic] || sub.topics["*"]) {
			event := map[string]any{"event": topic, "data": data}
			if jsonData, err := json.Marshal(event); err == nil {
				sub.mu.Lock()
				sub.conn.Write(append(jsonData, '\n'))
				sub.mu.Unlock()
			}
		}
	}
}

// Query returns the current state for requested topics.
func Query(state *State, topic string) (string, error) {
	if topic == "all" || topic == "" {
		jsonData, err := state.JSON()
		return string(jsonData), err
	}

	data := state.Get(topic)
	if data == nil {
		return "null", nil
	}
	jsonData, err := json.Marshal(data)
	return string(jsonData), err
}

// ParseSubscribeCommand parses "subscribe topic1 topic2 ..." into topics.
func ParseSubscribeCommand(cmd string) []string {
	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return []string{"*"} // Subscribe to all
	}
	return parts[1:]
}
