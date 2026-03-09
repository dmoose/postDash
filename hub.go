package main

import (
	"encoding/json"
	"maps"
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
	TotalEvents uint64            `json:"total_events"`
	ClientCount int               `json:"client_count"`
	BySource    map[string]uint64 `json:"by_source"`
	ByLevel     map[string]uint64 `json:"by_level"`
	ByChannel   map[string]uint64 `json:"by_channel"`
	RecentRate  float64           `json:"events_per_second"` // over last 10s window
}

// Hub manages the event ring buffer and client fan-out.
type Hub struct {
	rb  *RingBroadcaster[Event]
	mu  sync.RWMutex
	seq uint64

	// Stats counters
	bySource  map[string]uint64
	byLevel   map[string]uint64
	byChannel map[string]uint64
}

// Client is a connected SSE subscriber.
type Client = Subscriber[Event]

// Filter controls which events a client receives.
type Filter struct {
	Source  string
	Channel string
	Action  string
	Level   string
	AgentID string
}

func (f Filter) matches(e Event) bool {
	return matchesEvent(f.Source, f.Channel, f.Action, f.Level, f.AgentID, e)
}

func (f Filter) matchFunc() func(Event) bool {
	return f.matches
}

// NewHub creates a Hub with the given ring buffer size.
func NewHub(maxSize int) *Hub {
	return &Hub{
		rb:        NewRingBroadcaster[Event](maxSize),
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

	// Update counters
	if evt.Source != "" {
		h.bySource[evt.Source]++
	}
	h.byLevel[evt.Level]++
	if evt.Channel != "" {
		h.byChannel[evt.Channel]++
	}
	h.mu.Unlock()

	h.rb.Append(evt)

	return &evt, nil
}

// Stats returns current aggregate counters.
func (h *Hub) Stats() HubStats {
	h.mu.RLock()
	stats := HubStats{
		TotalEvents: h.seq,
		ClientCount: h.rb.ClientCount(),
		BySource:    copyMap(h.bySource),
		ByLevel:     copyMap(h.byLevel),
		ByChannel:   copyMap(h.byChannel),
	}
	h.mu.RUnlock()

	// Calculate rate from ring buffer (events in last 10s)
	cutoff := time.Now().Add(-10 * time.Second)
	var recent int
	h.rb.Walk(func(e Event) bool {
		if e.TS.Before(cutoff) {
			return false
		}
		recent++
		return true
	})
	stats.RecentRate = float64(recent) / 10.0

	return stats
}

// RateHistory returns event counts per bucket over the last duration.
func (h *Hub) RateHistory(duration time.Duration, buckets int) []int {
	now := time.Now()
	bucketSize := duration / time.Duration(buckets)
	counts := make([]int, buckets)

	h.rb.Walk(func(e Event) bool {
		age := now.Sub(e.TS)
		if age > duration {
			return false
		}
		idx := max(buckets-1-int(age/bucketSize), 0)
		if idx >= buckets {
			idx = buckets - 1
		}
		counts[idx]++
		return true
	})
	return counts
}

// Channels returns a list of all channels that have received events.
func (h *Hub) Channels() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	channels := make([]string, 0, len(h.byChannel))
	for ch := range h.byChannel {
		channels = append(channels, ch)
	}
	return channels
}

// BufferUsage returns current ring occupancy and configured max size.
func (h *Hub) BufferUsage() (used int, capacity int) {
	return h.rb.Len(), h.rb.Cap()
}

func copyMap(m map[string]uint64) map[string]uint64 {
	c := make(map[string]uint64, len(m))
	maps.Copy(c, m)
	return c
}

// Subscribe creates a new client with the given filter.
func (h *Hub) Subscribe(f Filter) *Client {
	return h.rb.Subscribe(f.matchFunc())
}

// Unsubscribe removes a client.
func (h *Hub) Unsubscribe(c *Client) {
	h.rb.Unsubscribe(c)
}

// Recent returns the last n events from the ring buffer, filtered.
func (h *Hub) Recent(n int, f Filter) []Event {
	return h.rb.Recent(n, f.matchFunc())
}
