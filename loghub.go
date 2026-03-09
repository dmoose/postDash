package main

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// LogEntry is a structured log record.
type LogEntry struct {
	Seq     uint64         `json:"seq"`
	Level   string         `json:"level"` // debug, info, warn, error, fatal
	Message string         `json:"message"`
	Logger  string         `json:"logger,omitempty"` // logger name / component
	Fields  map[string]any `json:"fields,omitempty"` // structured key-value pairs
	Caller  string         `json:"caller,omitempty"` // file:line
	TS      time.Time      `json:"ts"`
}

// LogHubStats holds aggregate counters for logs.
type LogHubStats struct {
	TotalLogs   uint64            `json:"total_logs"`
	Accepted    uint64            `json:"accepted"`
	Gated       uint64            `json:"gated"`
	ClientCount int               `json:"client_count"`
	MinLevel    string            `json:"min_level"`
	ByLevel     map[string]uint64 `json:"by_level"`
	ByLogger    map[string]uint64 `json:"by_logger"`
	RecentRate  float64           `json:"logs_per_second"`
}

var levelOrder = map[string]int{"debug": 0, "info": 1, "warn": 2, "error": 3, "fatal": 4}

func levelNum(level string) int {
	if n, ok := levelOrder[strings.ToLower(level)]; ok {
		return n
	}
	return 1 // default unknown to info
}

// LogHub manages the log ring buffer with level gating.
type LogHub struct {
	rb       *RingBroadcaster[LogEntry]
	minLevel int
	minName  string

	mu       sync.RWMutex
	seq      uint64
	accepted uint64
	gated    uint64
	byLevel  map[string]uint64
	byLogger map[string]uint64
}

// NewLogHub creates a LogHub with the given buffer size and minimum level.
func NewLogHub(maxSize int, minLevel string) *LogHub {
	return &LogHub{
		rb:       NewRingBroadcaster[LogEntry](maxSize),
		minLevel: levelNum(minLevel),
		minName:  strings.ToLower(minLevel),
		byLevel:  make(map[string]uint64),
		byLogger: make(map[string]uint64),
	}
}

// Publish parses a log entry, applies level gating, and appends to the buffer.
// Returns the entry, whether it was accepted, and any parse error.
func (lh *LogHub) Publish(raw json.RawMessage) (*LogEntry, bool, error) {
	var entry LogEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, false, err
	}

	if entry.Level == "" {
		entry.Level = "info"
	}
	entry.Level = strings.ToLower(entry.Level)

	// Level gating
	if levelNum(entry.Level) < lh.minLevel {
		lh.mu.Lock()
		lh.gated++
		lh.mu.Unlock()
		return &entry, false, nil
	}

	lh.mu.Lock()
	lh.seq++
	entry.Seq = lh.seq
	if entry.TS.IsZero() {
		entry.TS = time.Now()
	}
	lh.accepted++
	lh.byLevel[entry.Level]++
	if entry.Logger != "" {
		lh.byLogger[entry.Logger]++
	}
	lh.mu.Unlock()

	lh.rb.Append(entry)

	return &entry, true, nil
}

// Subscribe creates a subscriber, optionally filtered to a minimum level.
// Empty minLevel means no filtering (all accepted entries).
func (lh *LogHub) Subscribe(minLevel string) *Subscriber[LogEntry] {
	if minLevel == "" {
		return lh.rb.Subscribe(func(LogEntry) bool { return true })
	}
	min := levelNum(minLevel)
	return lh.rb.Subscribe(func(e LogEntry) bool {
		return levelNum(e.Level) >= min
	})
}

// Unsubscribe removes a log subscriber.
func (lh *LogHub) Unsubscribe(s *Subscriber[LogEntry]) {
	lh.rb.Unsubscribe(s)
}

// Recent returns the last n log entries at or above the given level.
// Empty minLevel means no filtering (all accepted entries).
func (lh *LogHub) Recent(n int, minLevel string) []LogEntry {
	if minLevel == "" {
		return lh.rb.Recent(n, func(LogEntry) bool { return true })
	}
	min := levelNum(minLevel)
	return lh.rb.Recent(n, func(e LogEntry) bool {
		return levelNum(e.Level) >= min
	})
}

// Stats returns current log counters.
func (lh *LogHub) Stats() LogHubStats {
	lh.mu.RLock()
	stats := LogHubStats{
		TotalLogs:   lh.accepted + lh.gated,
		Accepted:    lh.accepted,
		Gated:       lh.gated,
		ClientCount: lh.rb.ClientCount(),
		MinLevel:    lh.minName,
		ByLevel:     copyMap(lh.byLevel),
		ByLogger:    copyMap(lh.byLogger),
	}
	lh.mu.RUnlock()

	// Calculate rate from ring buffer (entries in last 10s)
	cutoff := time.Now().Add(-10 * time.Second)
	var recent int
	lh.rb.Walk(func(e LogEntry) bool {
		if e.TS.Before(cutoff) {
			return false
		}
		recent++
		return true
	})
	stats.RecentRate = float64(recent) / 10.0

	return stats
}

// RateHistory returns log counts per bucket over the last duration.
func (lh *LogHub) RateHistory(duration time.Duration, buckets int) []int {
	now := time.Now()
	bucketSize := duration / time.Duration(buckets)
	counts := make([]int, buckets)

	lh.rb.Walk(func(e LogEntry) bool {
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

// BufferUsage returns current ring occupancy and configured max size.
func (lh *LogHub) BufferUsage() (used int, capacity int) {
	return lh.rb.Len(), lh.rb.Cap()
}
