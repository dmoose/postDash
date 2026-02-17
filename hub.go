package main

import (
	"encoding/json"
	"sync"
	"time"
)

// Event is the structured payload received and relayed.
type Event struct {
	Seq        uint64         `json:"seq"`
	Source     string         `json:"source"`
	Channel    string         `json:"channel,omitempty"`
	Action     string         `json:"action,omitempty"`
	Level      string         `json:"level,omitempty"` // info, warn, error, debug
	AgentID    string         `json:"agent_id,omitempty"`
	DurationMS *int64         `json:"duration_ms,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	TS         time.Time      `json:"ts"`
}

// HubStats holds aggregate counters for the dashboard.
type HubStats struct {
	TotalEvents    uint64            `json:"total_events"`
	ClientCount    int               `json:"client_count"`
	BySource       map[string]uint64 `json:"by_source"`
	ByLevel        map[string]uint64 `json:"by_level"`
	ByChannel      map[string]uint64 `json:"by_channel"`
	RecentRate     float64           `json:"events_per_second"` // over last 10s window
}

// Hub manages the event ring buffer and client fan-out.
type Hub struct {
	mu      sync.RWMutex
	seq     uint64
	ring    []Event
	maxSize int
	clients map[*Client]bool

	// Stats counters
	bySource  map[string]uint64
	byLevel   map[string]uint64
	byChannel map[string]uint64
}

// Client is a connected SSE subscriber.
type Client struct {
	ch     chan Event
	filter Filter
}

// Filter controls which events a client receives.
type Filter struct {
	Source  string
	Channel string
	Action  string
	Level   string
	AgentID string
}

func (f Filter) matches(e Event) bool {
	if f.Source != "" && e.Source != f.Source {
		return false
	}
	if f.Channel != "" && e.Channel != f.Channel {
		return false
	}
	if f.Action != "" && e.Action != f.Action {
		return false
	}
	if f.Level != "" && e.Level != f.Level {
		return false
	}
	if f.AgentID != "" && e.AgentID != f.AgentID {
		return false
	}
	return true
}

// NewHub creates a Hub with the given ring buffer size.
func NewHub(maxSize int) *Hub {
	return &Hub{
		ring:      make([]Event, 0, maxSize),
		maxSize:   maxSize,
		clients:   make(map[*Client]bool),
		bySource:  make(map[string]uint64),
		byLevel:   make(map[string]uint64),
		byChannel: make(map[string]uint64),
	}
}

// Publish adds an event to the ring buffer and fans it out to clients.
func (h *Hub) Publish(raw json.RawMessage) (*Event, error) {
	var evt Event
	if err := json.Unmarshal(raw, &evt); err != nil {
		return nil, err
	}

	h.mu.Lock()
	h.seq++
	evt.Seq = h.seq
	if evt.TS.IsZero() {
		evt.TS = time.Now()
	}
	if evt.Level == "" {
		evt.Level = "info"
	}

	if len(h.ring) >= h.maxSize {
		h.ring = h.ring[1:]
	}
	h.ring = append(h.ring, evt)

	// Update counters
	if evt.Source != "" {
		h.bySource[evt.Source]++
	}
	h.byLevel[evt.Level]++
	if evt.Channel != "" {
		h.byChannel[evt.Channel]++
	}

	clients := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	for _, c := range clients {
		if c.filter.matches(evt) {
			select {
			case c.ch <- evt:
			default:
			}
		}
	}

	return &evt, nil
}

// Stats returns current aggregate counters.
func (h *Hub) Stats() HubStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := HubStats{
		TotalEvents: h.seq,
		ClientCount: len(h.clients),
		BySource:    copyMap(h.bySource),
		ByLevel:     copyMap(h.byLevel),
		ByChannel:   copyMap(h.byChannel),
	}

	// Calculate rate from ring buffer (events in last 10s)
	cutoff := time.Now().Add(-10 * time.Second)
	var recent int
	for i := len(h.ring) - 1; i >= 0; i-- {
		if h.ring[i].TS.Before(cutoff) {
			break
		}
		recent++
	}
	stats.RecentRate = float64(recent) / 10.0

	return stats
}

func copyMap(m map[string]uint64) map[string]uint64 {
	c := make(map[string]uint64, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// Subscribe creates a new client with the given filter.
func (h *Hub) Subscribe(f Filter) *Client {
	c := &Client{
		ch:     make(chan Event, 64),
		filter: f,
	}
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
	return c
}

// Unsubscribe removes a client.
func (h *Hub) Unsubscribe(c *Client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	close(c.ch)
}

// Recent returns the last n events from the ring buffer, filtered.
func (h *Hub) Recent(n int, f Filter) []Event {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []Event
	for i := len(h.ring) - 1; i >= 0 && len(result) < n; i-- {
		if f.matches(h.ring[i]) {
			result = append(result, h.ring[i])
		}
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}
