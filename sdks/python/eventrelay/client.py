"""Fire-and-forget event client for eventrelay."""

from __future__ import annotations

import json
import threading
import time
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from typing import Any
from urllib.request import Request, urlopen


@dataclass
class Event:
    """Structured event payload."""

    source: str = ""
    channel: str = ""
    action: str = ""
    level: str = "info"
    agent_id: str = ""
    duration_ms: int | None = None
    data: dict[str, Any] = field(default_factory=dict)
    ts: str = ""

    def to_dict(self) -> dict[str, Any]:
        d = {k: v for k, v in asdict(self).items() if v is not None and v != "" and v != {}}
        if "ts" not in d:
            d["ts"] = datetime.now(timezone.utc).isoformat()
        return d


class Client:
    """Sends events to an eventrelay server.

    All methods are thread-safe. If url is empty, all operations are no-ops.

    Usage::

        from eventrelay import Client

        er = Client("http://localhost:6060/events", "myapp")
        er.emit("deploy", {"env": "prod"})
        er.error("crash", {"msg": "something broke"})

        # Timed operations
        with er.timed("db_query") as t:
            result = do_query()
            t.data["rows"] = len(result)

        er.flush()
    """

    def __init__(self, url: str = "", source: str = "", channel: str = "", timeout: float = 2.0):
        self.url = url
        self.source = source
        self.channel = channel
        self.timeout = timeout
        self._pending: list[threading.Thread] = []
        self._lock = threading.Lock()

    def with_channel(self, channel: str) -> Client:
        """Return a new Client that tags all events with the given channel."""
        return Client(url=self.url, source=self.source, channel=channel, timeout=self.timeout)

    def emit(self, action: str, data: dict[str, Any] | None = None) -> None:
        """Send an info-level event."""
        self._send("info", action, data)

    def error(self, action: str, data: dict[str, Any] | None = None) -> None:
        """Send an error-level event."""
        self._send("error", action, data)

    def warn(self, action: str, data: dict[str, Any] | None = None) -> None:
        """Send a warn-level event."""
        self._send("warn", action, data)

    def debug(self, action: str, data: dict[str, Any] | None = None) -> None:
        """Send a debug-level event."""
        self._send("debug", action, data)

    def emit_event(self, event: Event) -> None:
        """Send a fully customized event."""
        if not self.url:
            return
        if not event.source:
            event.source = self.source
        if not event.channel:
            event.channel = self.channel
        if not event.level:
            event.level = "info"
        self._post(event.to_dict())

    def timed(self, action: str, data: dict[str, Any] | None = None) -> TimedContext:
        """Context manager for timing operations.

        Usage::

            with er.timed("db_query") as t:
                result = do_query()
                t.data["rows"] = len(result)
        """
        return TimedContext(self, action, data or {})

    def flush(self) -> None:
        """Wait for all pending events to be sent."""
        with self._lock:
            threads = list(self._pending)
        for t in threads:
            t.join(timeout=5.0)
        with self._lock:
            self._pending = [t for t in self._pending if t.is_alive()]

    def _send(self, level: str, action: str, data: dict[str, Any] | None) -> None:
        if not self.url:
            return
        evt = Event(
            source=self.source,
            channel=self.channel,
            action=action,
            level=level,
            data=data or {},
        )
        self._post(evt.to_dict())

    def _post(self, payload: dict[str, Any]) -> None:
        def do_post():
            try:
                body = json.dumps(payload).encode()
                req = Request(self.url, data=body, headers={"Content-Type": "application/json"})
                with urlopen(req, timeout=self.timeout):
                    pass
            except Exception:
                pass  # fire and forget

        t = threading.Thread(target=do_post, daemon=True)
        t.start()
        with self._lock:
            self._pending.append(t)
            # Prune completed threads
            self._pending = [th for th in self._pending if th.is_alive()]


class TimedContext:
    """Context manager that emits a timed event on exit."""

    def __init__(self, client: Client, action: str, data: dict[str, Any]):
        self.client = client
        self.action = action
        self.data = data
        self._start = 0.0

    def __enter__(self) -> TimedContext:
        self._start = time.monotonic()
        return self

    def __exit__(self, *_: Any) -> None:
        ms = int((time.monotonic() - self._start) * 1000)
        evt = Event(
            action=self.action,
            level="info",
            duration_ms=ms,
            data=self.data,
        )
        self.client.emit_event(evt)
