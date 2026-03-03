# Architecture

eventrelay is a lightweight event streaming service built around three core concepts: a ring buffer for in-memory event storage, Server-Sent Events (SSE) for real-time fan-out, and match rules for notification routing.

## Design Goals

1. **Zero-config useful** — `go build && ./eventrelay` gives you a working dashboard with no setup
2. **Language-agnostic ingestion** — anything that can POST JSON can send events
3. **Fire-and-forget SDKs** — clients never block the caller's critical path
4. **Single binary** — no external dependencies (database, message broker, etc.) required

## Component Overview

```
                                ┌─────────────┐
    POST /events ───────────────▶   Handlers   │
                                │  handlers.go │
                                └──────┬───────┘
                                       │
                  ┌────────────────────▼────────────────────┐
                  │              Hub (hub.go)                │
                  │                                         │
                  │  Ring Buffer     Stats     Subscribers  │
                  │  [e1|e2|e3|…]   counters  [client1, …] │
                  └──┬──────────────────────────────┬───────┘
                     │                              │
              ┌──────▼──────┐              ┌────────▼────────┐
              │  Notifier   │              │   SSE Stream    │
              │ notify.go   │              │  GET /events/   │
              │             │              │     stream      │
              │ Slack       │              └─────────────────┘
              │ Discord     │
              │ Webhook     │
              │ Database    │
              └─────────────┘
```

## Hub and Ring Buffer

The `Hub` (`hub.go`) is the central data structure. It maintains:

- A **ring buffer** of fixed capacity (default 1000 events). When full, the oldest event is evicted. This provides bounded memory usage regardless of throughput.
- **Stats counters** aggregated by source, level, and channel. These are monotonically increasing — they count all events ever received, not just those in the ring.
- A **subscriber map** of SSE clients with per-client filters.

All hub operations are protected by a `sync.RWMutex`. Publishing takes a write lock; stats and recent queries take a read lock. The lock is released before fanning out to SSE clients to avoid holding it during I/O.

## Event Flow

1. **Validate**: The handler checks that `source` is present, rejecting the request with 400 if missing
2. **Ingest**: `POST /events` (or `/events/batch`) passes raw bytes to `hub.Publish()`
3. **Enrich**: The hub assigns a monotonic sequence number, defaults level to "info", and sets timestamp if missing
4. **Store**: The event is appended to the ring buffer (evicting the oldest if full)
5. **Fan-out**: Each subscribed SSE client whose filter matches receives the event via a buffered channel (non-blocking send — slow clients drop events rather than causing backpressure)
6. **Notify**: If a notifier is configured, the event is checked against match rules asynchronously (in a goroutine) and dispatched to Slack/Discord/webhook/database targets
7. **Log**: If `--log` is set, the event is appended to a JSONL file

## Match Rules

Both SSE filters and notification rules use AND logic: all non-empty fields must match. This is intentionally simple — complex routing belongs in the notification targets (e.g., Slack channel routing via separate rules).

## SDKs

All three SDKs (Go, Python, TypeScript) follow the same pattern:

- **Fire-and-forget**: Events are sent asynchronously. The caller never blocks.
- **No-op when unconfigured**: Empty URL produces a client that silently discards events, so instrumentation code doesn't need conditional checks.
- **`Timed()` helper**: Captures start time and emits an event with `duration_ms` on completion.
- **`Flush()`**: Waits for all pending sends — used before process exit.

The Go SDK additionally provides a `slog.Handler` implementation for routing structured log output to eventrelay.

## Web Dashboard

The web UI (`static/index.html` + `static/app.js`) is embedded in the binary via `//go:embed`. It connects to the SSE stream for live events and polls `/events/stats` and `/events/rate` for dashboard metrics. All rendering is client-side vanilla JS — no build step or framework.

## TUI Dashboard

The TUI (`tui.go`) uses the Charmbracelet BubbleTea framework. It connects to a running eventrelay server as an SSE client, providing terminal-based monitoring with filtering, pause, and color-coded output. The TUI is a client, not a server — it connects to the same HTTP endpoints as the web dashboard.

## CLI Send Command

The `send` subcommand (`send.go`) is a thin HTTP client that builds an event from flags and POSTs it to the server. It supports both flag-based construction (`-s myapp -a deploy`) and raw JSON from stdin (`--stdin`). This avoids the need for `curl` in shell scripts and cron jobs.

## HTTP Middleware

All requests pass through a CORS middleware that sets `Access-Control-Allow-Origin: *` and handles `OPTIONS` preflight requests. This allows browser-based clients on other origins to use the API directly.

A `GET /healthz` endpoint returns `{"ok":true,"version":"..."}` for load balancer and monitoring health checks.

## Graceful Shutdown

The server listens for SIGINT and SIGTERM via `signal.NotifyContext`. On signal, `http.Server.Shutdown()` is called which stops accepting new connections, waits for in-flight requests (including SSE streams) to complete, then returns. Deferred cleanup (PID file removal, notifier DB close, log file close) runs after shutdown completes.
