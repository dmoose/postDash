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
eventrelay send -s myapp -a deploy -d '{"env":"prod"}'
```

Or with curl:

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

## CLI Send

Send events from scripts, cron jobs, or the terminal without curl:

```bash
eventrelay send -s myapp -a deploy -l info -d '{"branch":"main"}'
eventrelay send --source ci --action build_done --channel builds
eventrelay send -s myapp -a crash -l error

# Pipe raw JSON
echo '{"source":"ci","action":"done"}' | eventrelay send --stdin

# With auth and custom server
eventrelay send -s myapp -a test -t mysecret -p 8080
eventrelay send -s myapp -a test --url http://remote:6060
```

## Event Schema

Events are JSON objects posted to `POST /events`:

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

### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `source` | string | **yes** | What system or tool sent this event. Use a consistent identifier like `myapp`, `ci`, `llmshadow`. This is the primary grouping key in the dashboard. |
| `channel` | string | no | A topic or category within the source. Use to separate concerns like `deploy`, `builds`, `monitoring`. Events can be filtered by channel via tabs in the dashboard. |
| `action` | string | no | What happened. Use a short verb or operation name like `started`, `completed`, `db_query`, `shadow_scan`. |
| `level` | string | no | Severity: `debug`, `info`, `warn`, or `error`. Defaults to `info`. The dashboard color-codes events by level and shows error/warn counts. |
| `agent_id` | string | no | Identifies which agent, worker, or instance emitted the event. Useful when multiple agents share the same source — e.g., `claude-code-session-1`, `worker-3`. Displayed in the dashboard and filterable. |
| `duration_ms` | integer | no | How long the operation took in milliseconds. Displayed inline in the dashboard. Use the SDK `Timed()` helpers to set this automatically. |
| `data` | object | no | Arbitrary JSON payload with additional context. Shown inline in the dashboard for small payloads, expandable for larger ones. |
| `ts` | string | no | ISO 8601 timestamp. Auto-set to the server's current time if omitted. |
| `seq` | integer | — | Assigned by the server. Monotonically increasing sequence number. Do not set this. |

### Guidelines for agents

- Always set `source` to identify yourself consistently across events
- Set `agent_id` to distinguish between concurrent instances of the same source
- Use `action` for the operation name, not a full sentence — keep it grep-friendly
- Use `level: error` for failures, `level: warn` for degraded states, `level: debug` for verbose tracing
- Put structured details in `data`, not in `action` — the action should be a stable key you can filter on
- Use `channel` to separate event streams within a source (e.g., a CI system might use channels `build`, `test`, `deploy`)

## API

| Endpoint | Method | Description |
|---|---|---|
| `/events` | POST | Submit an event |
| `/events/batch` | POST | Submit multiple events as a JSON array |
| `/events/stream` | GET | SSE stream (filterable via query params) |
| `/events/recent` | GET | Last N events as JSON (`?n=100`) |
| `/events/stats` | GET | Aggregate counters (by source, level, channel) |
| `/events/rate` | GET | Event rate history (`?minutes=5&buckets=60`) |
| `/events/channels` | GET | List all active channels |
| `/healthz` | GET | Health check (`{"ok":true,"version":"..."}`) |
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

## Pages (command portal)

eventrelay can execute local commands and display their output as dashboard pages. This turns it into a portal for any CLI tool on the system — anything that can produce text, JSON, YAML, or markdown becomes a browser-accessible dashboard tab.

### Configuration

Add a `pages` section to your config file:

```yaml
server:
  scripts_dir: /usr/local/share/eventrelay/scripts

pages:
  - name: System
    command: er-system
    format: markdown
    interval: 30s

  - name: Ports
    command: er-ports
    format: text
    interval: 10s

  - name: Homebrew
    command: er-brew
    format: markdown
    interval: 5m
```

### Output formats

| Format | Rendering |
|--------|-----------|
| `text` | Pre-formatted monospace, HTML-escaped |
| `json` | Syntax-highlighted with color-coded keys, strings, numbers |
| `yaml` | Pre-formatted monospace, HTML-escaped |
| `markdown` | Rendered with headings, bold, code blocks, lists, tables, blockquotes |

### Bundled scripts

The `scripts/` directory contains ready-to-use page scripts, installed to `$PREFIX/share/eventrelay/scripts/` by `make install`:

| Script | Format | Description |
|--------|--------|-------------|
| `er-system` | markdown | Machine overview — OS, chip, memory, disk, load, top processes |
| `er-ports` | text | Listening TCP ports with process names |
| `er-services` | text | Running launchd user agents (non-Apple) |
| `er-brew` | markdown | Homebrew status — outdated packages, installed counts |
| `er-example` | markdown | Demonstrates all markdown rendering features |

### Writing your own scripts

Page scripts are regular shell scripts. They can output any supported format. Set `scripts_dir` in the config so scripts are on PATH automatically (important for launchd services which have a minimal PATH).

```bash
#!/bin/sh
# my-script — description
# Format: markdown

echo "# My Dashboard"
echo ""
echo "| Key | Value |"
echo "|-----|-------|"
echo "| Time | $(date) |"
echo "| User | $(whoami) |"
```

### Security

Commands can only be registered in the config file — there is no API for adding commands at runtime. All output is HTML-escaped before rendering. See [SECURITY.md](SECURITY.md) for the full threat model.

## Notifications

Create `eventrelay.yaml` (see [eventrelay.example.yaml](eventrelay.example.yaml)):

```yaml
# Server settings (flags override these)
server:
  port: 6060
  bind: 127.0.0.1
  # token: mysecret
  buffer: 1000
  # log: /var/log/eventrelay/events.jsonl

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
--version        print version and exit
```

## macOS Service

```bash
make install            # build and install binary to /usr/local/bin
make install-service    # install + start on login via launchd
make status             # check if running
make upgrade            # build, replace binary, restart service
make restart-service    # restart without rebuilding
make uninstall-service  # stop and remove service
```

### Upgrading

After pulling new code, run `make upgrade`. This stops the running service, installs the new binary, and restarts via launchd. The service has `KeepAlive` enabled, so launchd handles the restart automatically if the process exits.

If you installed via `go install` without the launchd service, stop the running process (`kill $(cat ~/.config/eventrelay/eventrelay.pid)`), then `go install github.com/dmoose/eventrelay@latest` and start again.

## Network / Intranet Deployment

eventrelay has real value as an intranet dashboard — a single URL for your team to see events, system status, and tool output. **Do not expose it to the public internet.**

### Docker + Caddy

The `deploy/` directory contains a ready-to-use setup with Caddy for TLS and optional basic auth:

```bash
cd deploy
docker compose up -d
```

This gives you:
- eventrelay on port 6060 (internal)
- Caddy reverse proxy with automatic TLS on ports 80/443
- Basic auth (optional, see `deploy/Caddyfile`)

Configure the domain in `deploy/Caddyfile` and event token in `deploy/eventrelay.yaml`.

### Recommended network architecture

```
SDKs/agents → eventrelay:6060 (Bearer token auth)
Browsers    → Caddy (TLS + basic auth) → eventrelay:6060
```

- eventrelay handles SDK authentication via `--token`
- Caddy handles browser authentication via basic auth
- This separation means SDKs use token auth (no browser needed) while the dashboard is password-protected

See `deploy/Caddyfile` for examples including protecting only the dashboard while leaving the event API open.

## Security

eventrelay is designed for localhost and trusted networks. See [SECURITY.md](SECURITY.md) for the full threat model covering:

- On-device security (localhost default)
- Network deployment considerations
- Page command security model
- XSS prevention in the dashboard
- What NOT to do

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for design details on the ring buffer, SSE fan-out, notification pipeline, and pages system.

## License

MIT — see [LICENSE](LICENSE).
