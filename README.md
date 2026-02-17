# eventrelay

A lightweight real-time event streaming service. Any tool that can POST JSON gets a live dashboard — browser UI, TUI, or both.

## Quick Start

```bash
go build -o eventrelay .
./eventrelay --port 6060
```

Open http://localhost:6060 in a browser, then send events:

```bash
curl -X POST http://localhost:6060/events \
  -d '{"source":"myapp","action":"deploy","level":"info","data":{"env":"prod"}}'
```

## TUI Dashboard

Connect a terminal dashboard to a running server:

```bash
./eventrelay --tui
./eventrelay --tui --url http://remote-server:6060
```

Keys: `/` filter, `x` clear filter, `p` pause, `c` clear, `q` quit, `ctrl+c` force quit.

## Event Schema

```json
{
  "source": "myapp",
  "channel": "deploy",
  "action": "build_complete",
  "level": "info",
  "agent_id": "claude-code",
  "duration_ms": 4200,
  "data": {"branch": "main", "commit": "abc123"},
  "ts": "2026-03-17T12:00:00Z"
}
```

Only `source` is required. `level` defaults to `info`. `ts` is auto-set if missing. `seq` is assigned by the server.

## API

| Endpoint | Method | Description |
|---|---|---|
| `/events` | POST | Submit an event |
| `/events/stream` | GET | SSE stream (filterable via query params) |
| `/events/recent` | GET | Last N events as JSON (`?n=100`) |
| `/events/stats` | GET | Aggregate counters (by source, level, channel) |
| `/` | GET | Web dashboard |

SSE and recent endpoints accept filter params: `?source=x&channel=y&level=error&action=z&agent_id=a`

## Go SDK

```go
import "github.com/dmoose/eventrelay/client"

c := client.New("http://localhost:6060/events", "myapp")
c.Emit("deploy", map[string]any{"env": "prod"})

// Timed operations
done := c.Timed("db_query", nil)
// ... do work ...
done(map[string]any{"rows": 42})

c.Flush() // wait for pending events before exit
```

### slog Integration

```go
handler := client.NewSlogHandler(c, "logs")
logger := slog.New(handler)
logger.Info("request handled", "path", "/api/users", "status", 200)
```

## Notifications

Create `eventrelay.yaml`:

```yaml
notify:
  - name: errors to slack
    match:
      level: error
    slack:
      webhook_url: https://hooks.slack.com/services/T00/B00/xxx

  - name: deploys to discord
    match:
      source: ci
      action: deploy
    discord:
      webhook_url: https://discord.com/api/webhooks/xxx/yyy

  - name: forward to webhook
    match:
      source: myapp
    webhook:
      url: https://example.com/hooks
      headers:
        Authorization: Bearer mytoken
```

```bash
./eventrelay --config eventrelay.yaml
```

## Network Mode

```bash
./eventrelay --bind 0.0.0.0 --token mysecret
```

With `--token`, POST requests require `Authorization: Bearer mysecret`.

## Flags

```
--port int       listen port (default 6060)
--bind string    bind address (default 127.0.0.1)
--token string   require Bearer token for POST
--log string     append events to JSONL file
--buffer int     ring buffer size (default 1000)
--config string  notification config file
--tui            connect as TUI dashboard client
--url string     server URL for TUI mode
```
