"""Tests for the eventrelay Python SDK."""

import json
from http.server import BaseHTTPRequestHandler, HTTPServer
from threading import Thread

from eventrelay import Client


class Handler(BaseHTTPRequestHandler):
    received = []

    def do_POST(self):
        length = int(self.headers["Content-Length"])
        body = json.loads(self.rfile.read(length))
        Handler.received.append(body)
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'{"seq":1}')

    def log_message(self, *_):
        pass


def setup_server():
    server = HTTPServer(("127.0.0.1", 0), Handler)
    port = server.server_address[1]
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server, f"http://127.0.0.1:{port}/events"


def test_emit():
    Handler.received.clear()
    server, url = setup_server()

    c = Client(url, "testapp")
    c.emit("deploy", {"env": "prod"})
    c.flush()
    server.shutdown()

    assert len(Handler.received) == 1
    evt = Handler.received[0]
    assert evt["source"] == "testapp"
    assert evt["action"] == "deploy"
    assert evt["level"] == "info"
    assert evt["data"]["env"] == "prod"


def test_error_and_warn():
    Handler.received.clear()
    server, url = setup_server()

    c = Client(url, "app")
    c.error("crash", {"msg": "boom"})
    c.warn("slow", {"ms": 5000})
    c.flush()
    server.shutdown()

    assert len(Handler.received) == 2
    levels = {e["level"] for e in Handler.received}
    assert levels == {"error", "warn"}


def test_channel():
    Handler.received.clear()
    server, url = setup_server()

    c = Client(url, "app").with_channel("ops")
    c.emit("restart")
    c.flush()
    server.shutdown()

    assert Handler.received[0]["channel"] == "ops"


def test_timed():
    Handler.received.clear()
    server, url = setup_server()

    c = Client(url, "app")
    with c.timed("query") as t:
        t.data["rows"] = 42
    c.flush()
    server.shutdown()

    evt = Handler.received[0]
    assert evt["action"] == "query"
    assert "duration_ms" in evt
    assert evt["data"]["rows"] == 42


def test_noop():
    c = Client("", "noop")
    c.emit("anything")  # should not raise
    c.flush()
