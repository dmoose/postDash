"""unittest-based tests for the eventrelay Python SDK."""

import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer

from eventrelay import Client


class Handler(BaseHTTPRequestHandler):
    received = []
    lock = threading.Lock()

    def do_POST(self):  # noqa: N802
        length = int(self.headers["Content-Length"])
        body = json.loads(self.rfile.read(length))
        with Handler.lock:
            Handler.received.append(body)
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'{"seq":1}')

    def log_message(self, *_):
        pass


def setup_server():
    server = HTTPServer(("127.0.0.1", 0), Handler)
    port = server.server_address[1]
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server, f"http://127.0.0.1:{port}/events"


class TestClient(unittest.TestCase):
    def setUp(self):
        with Handler.lock:
            Handler.received.clear()

    def _stop_server(self, server):
        server.shutdown()
        server.server_close()

    def test_emit(self):
        server, url = setup_server()
        c = Client(url, "testapp")
        c.emit("deploy", {"env": "prod"})
        c.flush()
        self._stop_server(server)
        with Handler.lock:
            self.assertEqual(1, len(Handler.received))
            evt = Handler.received[0]
        self.assertEqual("testapp", evt["source"])
        self.assertEqual("deploy", evt["action"])
        self.assertEqual("info", evt["level"])
        self.assertEqual("prod", evt["data"]["env"])

    def test_error_and_warn(self):
        server, url = setup_server()
        c = Client(url, "app")
        c.error("crash", {"msg": "boom"})
        c.warn("slow", {"ms": 5000})
        c.flush()
        self._stop_server(server)
        with Handler.lock:
            self.assertEqual(2, len(Handler.received))
            levels = {e["level"] for e in Handler.received}
        self.assertEqual({"error", "warn"}, levels)

    def test_channel(self):
        server, url = setup_server()
        c = Client(url, "app").with_channel("ops")
        c.emit("restart")
        c.flush()
        self._stop_server(server)
        with Handler.lock:
            self.assertEqual("ops", Handler.received[0]["channel"])

    def test_timed(self):
        server, url = setup_server()
        c = Client(url, "app")
        with c.timed("query") as t:
            t.data["rows"] = 42
        c.flush()
        self._stop_server(server)
        with Handler.lock:
            evt = Handler.received[0]
        self.assertEqual("query", evt["action"])
        self.assertIn("duration_ms", evt)
        self.assertEqual(42, evt["data"]["rows"])

    def test_noop(self):
        c = Client("", "noop")
        c.emit("anything")
        c.flush()


if __name__ == "__main__":
    unittest.main()

