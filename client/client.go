// Package client provides a Go SDK for sending events to an eventrelay server.
package client

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"
	"sync"
	"time"
)

// Event is the payload sent to the relay.
type Event struct {
	Source     string         `json:"source"`
	Channel    string         `json:"channel,omitempty"`
	Action     string         `json:"action,omitempty"`
	Level      string         `json:"level,omitempty"`
	AgentID    string         `json:"agent_id,omitempty"`
	DurationMS *int64         `json:"duration_ms,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	TS         time.Time      `json:"ts"`
}

// Client sends events to an eventrelay server.
// All methods are safe for concurrent use.
// If URL is empty, all operations are no-ops.
type Client struct {
	url    string
	source string
	http   *http.Client
	wg     sync.WaitGroup
}

// New creates a Client. If url is empty, returns a no-op client.
func New(url, source string) *Client {
	return &Client{
		url:    url,
		source: source,
		http:   &http.Client{Timeout: 2 * time.Second},
	}
}

// Emit sends an event. Fire-and-forget — non-blocking, errors silently dropped.
func (c *Client) Emit(action string, data map[string]any) {
	if c.url == "" {
		return
	}
	c.send(Event{
		Source: c.source,
		Action: action,
		Level:  "info",
		Data:   data,
		TS:     time.Now(),
	})
}

// EmitWith sends a fully customized event.
func (c *Client) EmitWith(evt Event) {
	if c.url == "" {
		return
	}
	if evt.Source == "" {
		evt.Source = c.source
	}
	if evt.TS.IsZero() {
		evt.TS = time.Now()
	}
	if evt.Level == "" {
		evt.Level = "info"
	}
	c.send(evt)
}

// Timed is a helper for timing operations. Returns a function that emits
// the event with duration_ms set when called.
//
//	done := client.Timed("db_query", nil)
//	// ... do work ...
//	done(map[string]any{"rows": 42})
func (c *Client) Timed(action string, data map[string]any) func(map[string]any) {
	start := time.Now()
	return func(extra map[string]any) {
		merged := make(map[string]any)
		maps.Copy(merged, data)
		maps.Copy(merged, extra)
		ms := time.Since(start).Milliseconds()
		c.EmitWith(Event{
			Action:     action,
			Level:      "info",
			DurationMS: &ms,
			Data:       merged,
		})
	}
}

// Flush waits for all pending events to be sent.
func (c *Client) Flush() {
	c.wg.Wait()
}

func (c *Client) send(evt Event) {
	c.wg.Go(func() {
		body, err := json.Marshal(evt)
		if err != nil {
			return
		}
		resp, err := c.http.Post(c.url, "application/json", bytes.NewReader(body))
		if err != nil {
			return
		}
		resp.Body.Close()
	})
}
