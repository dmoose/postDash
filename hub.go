package main

import (
	"encoding/json"
	"sync"
	"time"
)

// Event is the structured payload received and relayed.
type Event struct {
	Seq     uint64         `json:"seq"`
	Source  string         `json:"source"`
	Action  string         `json:"action,omitempty"`
	AgentID string         `json:"agent_id,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
	TS      time.Time      `json:"ts"`
}

// Hub manages the event ring buffer and client fan-out.
type Hub struct {
	mu      sync.RWMutex
	seq     uint64
	ring    []Event
	maxSize int
	clients map[*Client]bool
}

// Client is a connected SSE subscriber.
type Client struct {
	ch     chan Event
	filter Filter
}

// Filter controls which events a client receives.
type Filter struct {
	Source  string
	Action  string
	AgentID string
}

func (f Filter) matches(e Event) bool {
	if f.Source != "" && e.Source != f.Source {
		return false
	}
	if f.Action != "" && e.Action != f.Action {
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
		ring:    make([]Event, 0, maxSize),
		maxSize: maxSize,
		clients: make(map[*Client]bool),
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

	// Ring buffer
	if len(h.ring) >= h.maxSize {
		h.ring = h.ring[1:]
	}
	h.ring = append(h.ring, evt)

	// Snapshot clients under lock
	clients := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	// Fan out without holding the lock
	for _, c := range clients {
		if c.filter.matches(evt) {
			select {
			case c.ch <- evt:
			default:
				// Client is slow, drop event
			}
		}
	}

	return &evt, nil
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

	// Reverse to chronological order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}
