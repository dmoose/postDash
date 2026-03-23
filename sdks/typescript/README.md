# postDash TypeScript SDK

TypeScript/JavaScript client for sending events to an [postDash](https://github.com/dmoose/postDash) server.

## Install

```bash
npm install ./sdks/typescript
```

## Usage

```typescript
import { Client } from "postDash";

const er = new Client("http://localhost:6060/events", "myapp");

er.emit("deploy", { env: "prod" });
er.error("crash", { msg: "something broke" });
er.warn("high_latency", { p99: 450 });
er.debug("cache_miss");

await er.flush(); // wait for pending events before exit
```

### Channels

```typescript
const deploy = er.withChannel("deploy");
deploy.emit("started", { branch: "main" });
```

### Timed Operations

```typescript
const done = er.timed("db_query");
const result = await doQuery();
done({ rows: result.length });
```

### No-Op Mode

When the URL is empty, all operations silently no-op:

```typescript
const er = new Client(process.env.POSTDASH_URL ?? "", "myapp");
er.emit("startup"); // safe even if POSTDASH_URL is unset
```

## Requirements

TypeScript 5.0+ / Node.js with fetch support (18+)
