# eventrelay Go SDK

Go client for sending events to an [eventrelay](https://github.com/dmoose/eventrelay) server.

## Install

```bash
go get github.com/dmoose/eventrelay/client
```

## Usage

```go
import "github.com/dmoose/eventrelay/client"

c := client.New("http://localhost:6060/events", "myapp")

c.Emit("deploy", map[string]any{"env": "prod"})
c.EmitError("crash", map[string]any{"msg": "something broke"})
c.EmitWarn("high_latency", map[string]any{"p99": 450})
c.EmitDebug("cache_miss", nil)

c.Flush() // wait for pending events before exit
```

### Channels

```go
deploy := c.WithChannel("deploy")
deploy.Emit("started", map[string]any{"branch": "main"})
```

### Timed Operations

```go
done := c.Timed("db_query", nil)
rows := doQuery()
done(map[string]any{"rows": len(rows)})
```

### slog Integration

Route Go structured logging to eventrelay:

```go
handler := client.NewSlogHandler(c, "logs")
logger := slog.New(handler)
logger.Info("request handled", "path", "/api/users", "status", 200)
```

### No-Op Mode

When the URL is empty, all operations silently no-op. This lets you embed instrumentation without conditional checks:

```go
c := client.New(os.Getenv("EVENTRELAY_URL"), "myapp")
c.Emit("startup", nil) // safe even if EVENTRELAY_URL is unset
```
