"""Fake backend for Mimir, Loki, Pyroscope and OTLP/HTTP.

Accepts every POST, and counts requests by tenant (X-Scope-OrgID), consumer
(X-Alloy-Consumer) and path. GET /stats returns the counts as JSON.
"""

import json
import threading
from collections import defaultdict
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

lock = threading.Lock()
stats = defaultdict(lambda: defaultdict(lambda: defaultdict(int)))


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length", 0)))
        path = urlparse(self.path).path
        tenant = self.headers.get("X-Scope-OrgID", "<none>")
        consumer = self.headers.get("X-Alloy-Consumer", "<none>")
        with lock:
            stats[tenant][consumer][path] += 1
        print(json.dumps({"tenant": tenant, "consumer": consumer, "path": path}), flush=True)

        if path.startswith("/v1/"):
            self.send_response(200)
            self.send_header("Content-Type", "application/x-protobuf")
            self.send_header("Content-Length", "0")
            self.end_headers()
        elif path == "/ingest":
            self.send_response(200)
            self.send_header("Content-Length", "0")
            self.end_headers()
        else:
            self.send_response(204)
            self.end_headers()

    def do_GET(self):
        with lock:
            body = json.dumps(stats, indent=2).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_DELETE(self):
        with lock:
            stats.clear()
        self.send_response(204)
        self.end_headers()

    def log_message(self, *args):
        pass


ThreadingHTTPServer(("0.0.0.0", 9999), Handler).serve_forever()
