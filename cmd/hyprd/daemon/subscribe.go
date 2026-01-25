package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
)

// Subscriber represents a client subscribed to state changes.
type Subscriber struct {
	conn   net.Conn
	topics map[string]bool
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
			sub.conn.Write(jsonData)
		}
	}
}

// sendInitialState sends current state for subscribed topics.
func (m *SubscriptionManager) sendInitialState(sub *Subscriber) {
	if sub.topics["workspace"] || sub.topics["*"] {
		data := map[string]any{
			"current":  m.state.GetWorkspace(),
			"occupied": m.state.GetOccupied(),
		}
		event := map[string]any{"event": "workspace", "data": data}
		if jsonData, err := json.Marshal(event); err == nil {
			sub.conn.Write(append(jsonData, '\n'))
		}
	}

	if sub.topics["monocle"] || sub.topics["*"] {
		monocle := m.state.GetMonocle()
		var data any
		if monocle != nil {
			data = map[string]any{
				"address":   monocle.Address,
				"origin_ws": monocle.OriginWS,
			}
		}
		event := map[string]any{"event": "monocle", "data": data}
		if jsonData, err := json.Marshal(event); err == nil {
			sub.conn.Write(append(jsonData, '\n'))
		}
	}

	if sub.topics["split"] || sub.topics["*"] {
		event := map[string]any{"event": "split", "data": m.state.GetSplitRatio()}
		if jsonData, err := json.Marshal(event); err == nil {
			sub.conn.Write(append(jsonData, '\n'))
		}
	}
}

// Query returns the current state for requested topics.
func Query(state *State, topic string) (string, error) {
	switch topic {
	case "workspace":
		data := map[string]any{
			"current":  state.GetWorkspace(),
			"occupied": state.GetOccupied(),
		}
		jsonData, err := json.Marshal(data)
		return string(jsonData), err

	case "monocle":
		monocle := state.GetMonocle()
		if monocle == nil {
			return "null", nil
		}
		jsonData, err := json.Marshal(monocle)
		return string(jsonData), err

	case "pseudo":
		pseudo := state.GetPseudo()
		if pseudo == nil {
			return "null", nil
		}
		jsonData, err := json.Marshal(pseudo)
		return string(jsonData), err

	case "split":
		return fmt.Sprintf(`"%s"`, state.GetSplitRatio()), nil

	case "all", "":
		jsonData, err := state.JSON()
		return string(jsonData), err

	default:
		return "", fmt.Errorf("unknown topic: %s", topic)
	}
}

// ParseSubscribeCommand parses "subscribe topic1 topic2 ..." into topics.
func ParseSubscribeCommand(cmd string) []string {
	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return []string{"*"} // Subscribe to all
	}
	return parts[1:]
}
