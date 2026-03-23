# postDash Go SDK

Go client for sending events to an [postDash](https://github.com/dmoose/postDash) server.

## Install

```bash
go get github.com/dmoose/postDash/client
```

## Usage

```go
import "github.com/dmoose/postDash/client"

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

Route Go structured logging to postDash:

```go
handler := client.NewSlogHandler(c, "logs")
logger := slog.New(handler)
logger.Info("request handled", "path", "/api/users", "status", 200)
```

### No-Op Mode

When the URL is empty, all operations silently no-op. This lets you embed instrumentation without conditional checks:

```go
c := client.New(os.Getenv("POSTDASH_URL"), "myapp")
c.Emit("startup", nil) // safe even if POSTDASH_URL is unset
```
