# postDash Python SDK

Python client for sending events to an [postDash](https://github.com/dmoose/postDash) server.

## Install

```bash
pip install ./sdks/python
```

## Usage

```python
from postDash import Client

er = Client("http://localhost:6060/events", "myapp")

er.emit("deploy", {"env": "prod"})
er.error("crash", {"msg": "something broke"})
er.warn("high_latency", {"p99": 450})
er.debug("cache_miss")

er.flush()  # wait for pending events before exit
```

### Channels

```python
deploy = er.with_channel("deploy")
deploy.emit("started", {"branch": "main"})
```

### Timed Operations

```python
with er.timed("db_query") as t:
    result = do_query()
    t.data["rows"] = len(result)
```

### No-Op Mode

When the URL is empty, all operations silently no-op:

```python
import os
er = Client(os.environ.get("POSTDASH_URL", ""), "myapp")
er.emit("startup")  # safe even if POSTDASH_URL is unset
```

## Requirements

Python 3.10+
