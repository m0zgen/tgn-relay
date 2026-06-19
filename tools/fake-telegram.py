#!/usr/bin/env python3

# chmod +x fake-telegram.py
# ./fake-telegram.py

# tgn-relay configuration for testing with this fake Telegram server:

# telegram:
#   api_url: "http://127.0.0.1:18080"
#   timeout: 5s
#   queue_size: 100
#   send_interval: 1s

from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import time

counter = 0

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        global counter
        counter += 1

        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length).decode("utf-8", errors="replace")

        print(f"[{time.strftime('%H:%M:%S')}] #{counter} {self.path} body={body}", flush=True)

        self.send_header("Content-Type", "application/json")

        # Каждое 3-е сообщение симулирует Telegram 429.
        if counter % 3 == 0:
            self.send_response(429)
            self.end_headers()
            self.wfile.write(json.dumps({
                "ok": False,
                "error_code": 429,
                "description": "Too Many Requests: retry after 5",
                "parameters": {
                    "retry_after": 5
                }
            }).encode())
            return

        self.send_response(200)
        self.end_headers()
        self.wfile.write(json.dumps({
            "ok": True,
            "result": {
                "message_id": counter
            }
        }).encode())

    def log_message(self, format, *args):
        return

HTTPServer(("127.0.0.1", 18080), Handler).serve_forever()