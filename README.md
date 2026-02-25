# eventrelay

[![CI](https://github.com/dmoose/eventrelay/actions/workflows/ci.yml/badge.svg)](https://github.com/dmoose/eventrelay/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A lightweight real-time event streaming service. Any tool that can POST JSON gets a live dashboard — browser UI, TUI, or both.

## Why eventrelay?

Most observability tools are heavyweight — they need databases, collectors, dashboards, and configuration before you see anything. eventrelay is the opposite: a single binary that gives you a real-time event feed in seconds. It's designed for development workflows, CI pipelines, agent monitoring, and anywhere you want visibility without infrastructure.

- **Zero dependencies** — single Go binary, no database required
- **Language-agnostic** — POST JSON from any language or tool, SDKs for Go, Python, and TypeScript
- **Fire-and-forget** — SDKs never block your application's critical path
- **Two dashboards** — web UI in the browser, TUI in the terminal
- **Notification routing** — match rules forward events to Slack, Discord, webhooks, or a database

## Install

```bash
go install github.com/dmoose/eventrelay@latest
```

Or build from source:

```bash
git clone https://github.com/dmoose/eventrelay.git
cd eventrelay
make build
```

## Quick Start

```bash
eventrelay --port 6060
```

Open http://localhost:6060 in a browser, then send events:

```bash
curl -X POST http://localhost:6060/events \
  -d '{"source":"myapp","action":"deploy","level":"info","data":{"env":"prod"}}'
```

## TUI Dashboard

Connect a terminal dashboard to a running server:

```bash
eventrelay --tui
eventrelay --tui --url http://remote-server:6060
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
| `/events/rate` | GET | Event rate history (`?minutes=5&buckets=60`) |
| `/events/channels` | GET | List all active channels |
| `/` | GET | Web dashboard |

SSE and recent endpoints accept filter params: `?source=x&channel=y&level=error&action=z&agent_id=a`

## SDKs

### Go

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

#### slog Integration

```go
handler := client.NewSlogHandler(c, "logs")
logger := slog.New(handler)
logger.Info("request handled", "path", "/api/users", "status", 200)
```

See [client/README.md](client/README.md) for full Go SDK documentation.

### Python

```python
from eventrelay import Client

er = Client("http://localhost:6060/events", "myapp")
er.emit("deploy", {"env": "prod"})

with er.timed("db_query") as t:
    result = do_query()
    t.data["rows"] = len(result)

er.flush()
```

See [sdks/python/README.md](sdks/python/README.md) for full Python SDK documentation.

### TypeScript

```typescript
import { Client } from "eventrelay";

const er = new Client("http://localhost:6060/events", "myapp");
er.emit("deploy", { env: "prod" });

const done = er.timed("db_query");
const result = await doQuery();
done({ rows: result.length });

await er.flush();
```

See [sdks/typescript/README.md](sdks/typescript/README.md) for full TypeScript SDK documentation.

## Notifications

Create `eventrelay.yaml` (see [eventrelay.example.yaml](eventrelay.example.yaml)):

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
eventrelay --config eventrelay.yaml
```

## Network Mode

```bash
eventrelay --bind 0.0.0.0 --token mysecret
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
--status         check if eventrelay is running
```

## macOS Service

```bash
make install            # install binary to /usr/local/bin
make install-service    # start on login via launchd
make status             # check if running
make uninstall-service  # stop and remove service
```

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for design details on the ring buffer, SSE fan-out, and notification pipeline.

## License

MIT — see [LICENSE](LICENSE).
