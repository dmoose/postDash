package client

import (
	"context"
	"log/slog"
	"time"
)

// SlogHandler implements slog.Handler, sending structured log records as events.
// Use with slog.New(eventrelay.NewSlogHandler(client)) to route all structured
// logging to the event relay.
type SlogHandler struct {
	client  *Client
	channel string
	attrs   []slog.Attr
	groups  []string
}

// NewSlogHandler creates a slog.Handler that sends log records to the relay.
func NewSlogHandler(c *Client, channel string) *SlogHandler {
	return &SlogHandler{client: c, channel: channel}
}

func (h *SlogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return h.client.url != ""
}

func (h *SlogHandler) Handle(_ context.Context, r slog.Record) error {
	data := make(map[string]any)

	// Add pre-set attrs
	for _, a := range h.attrs {
		data[a.Key] = a.Value.Any()
	}

	// Add record attrs
	r.Attrs(func(a slog.Attr) bool {
		key := a.Key
		for _, g := range h.groups {
			key = g + "." + key
		}
		data[key] = a.Value.Any()
		return true
	})

	var level string
	switch {
	case r.Level >= slog.LevelError:
		level = "error"
	case r.Level >= slog.LevelWarn:
		level = "warn"
	case r.Level >= slog.LevelInfo:
		level = "info"
	default:
		level = "debug"
	}

	h.client.EmitWith(Event{
		Channel: h.channel,
		Action:  r.Message,
		Level:   level,
		Data:    data,
		TS:      time.Now(),
	})

	return nil
}

func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SlogHandler{
		client:  h.client,
		channel: h.channel,
		attrs:   append(append([]slog.Attr{}, h.attrs...), attrs...),
		groups:  h.groups,
	}
}

func (h *SlogHandler) WithGroup(name string) slog.Handler {
	return &SlogHandler{
		client:  h.client,
		channel: h.channel,
		attrs:   h.attrs,
		groups:  append(append([]string{}, h.groups...), name),
	}
}
