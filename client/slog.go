package client

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"
)

// SlogHandler implements slog.Handler, sending structured log records as events.
// Use with slog.New(eventrelay.NewSlogHandler(client, "logs")) to route all
// structured logging to the event relay as events on a channel.
type SlogHandler struct {
	client  *Client
	channel string
	attrs   []slog.Attr
	groups  []string
}

// NewSlogHandler creates a slog.Handler that sends log records as events.
func NewSlogHandler(c *Client, channel string) *SlogHandler {
	return &SlogHandler{client: c, channel: channel}
}

func (h *SlogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return h.client.url != ""
}

func (h *SlogHandler) Handle(_ context.Context, r slog.Record) error {
	data := make(map[string]any)
	for _, a := range h.attrs {
		data[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		key := a.Key
		for _, g := range h.groups {
			key = g + "." + key
		}
		data[key] = a.Value.Any()
		return true
	})

	h.client.EmitWith(Event{
		Channel: h.channel,
		Action:  r.Message,
		Level:   slogLevel(r.Level),
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

// SlogLogHandler implements slog.Handler, sending records to the /log endpoint
// as structured log entries (message, level, logger, fields, caller).
// Use with slog.New(eventrelay.NewSlogLogHandler(client)) for first-class
// log integration with the eventrelay Logs tab.
type SlogLogHandler struct {
	client *Client
	logger string // logger name (defaults to client source)
	attrs  []slog.Attr
	groups []string
	addSrc bool
}

// NewSlogLogHandler creates a slog.Handler that sends to the /log endpoint.
// Logger name defaults to the client's source; override with the returned
// handler's WithLogger method if needed.
func NewSlogLogHandler(c *Client, opts *SlogLogOptions) *SlogLogHandler {
	h := &SlogLogHandler{client: c, logger: c.source}
	if opts != nil {
		if opts.Logger != "" {
			h.logger = opts.Logger
		}
		h.addSrc = opts.AddSource
	}
	return h
}

// SlogLogOptions configures the log handler.
type SlogLogOptions struct {
	Logger    string // override logger name (default: client source)
	AddSource bool   // include file:line caller info
}

func (h *SlogLogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return h.client.url != ""
}

func (h *SlogLogHandler) Handle(_ context.Context, r slog.Record) error {
	fields := make(map[string]any)
	for _, a := range h.attrs {
		fields[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		key := a.Key
		for _, g := range h.groups {
			key = g + "." + key
		}
		fields[key] = a.Value.Any()
		return true
	})

	entry := LogEntry{
		Level:   slogLevel(r.Level),
		Message: r.Message,
		Logger:  h.logger,
		TS:      time.Now(),
	}
	if len(fields) > 0 {
		entry.Fields = fields
	}
	if h.addSrc && r.PC != 0 {
		f := runtime.FuncForPC(r.PC)
		if f != nil {
			file, line := f.FileLine(r.PC)
			entry.Caller = fmt.Sprintf("%s:%d", file, line)
		}
	}

	h.client.log(entry)
	return nil
}

func (h *SlogLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SlogLogHandler{
		client: h.client,
		logger: h.logger,
		attrs:  append(append([]slog.Attr{}, h.attrs...), attrs...),
		groups: h.groups,
		addSrc: h.addSrc,
	}
}

func (h *SlogLogHandler) WithGroup(name string) slog.Handler {
	return &SlogLogHandler{
		client: h.client,
		logger: h.logger,
		attrs:  h.attrs,
		groups: append(append([]string{}, h.groups...), name),
		addSrc: h.addSrc,
	}
}

func slogLevel(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "error"
	case l >= slog.LevelWarn:
		return "warn"
	case l >= slog.LevelInfo:
		return "info"
	default:
		return "debug"
	}
}
