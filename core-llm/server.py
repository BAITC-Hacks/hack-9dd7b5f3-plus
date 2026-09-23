#!/usr/bin/env python3
"""Private HTTP adapter for router.py. Run behind the Next.js /api/core-route proxy."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os

from router import Router, RouterError, load_env

MAX_BODY = 32_768


def validate(payload):
    if not isinstance(payload, dict):
        raise ValueError("Expected an object")
    text = payload.get("text")
    if not isinstance(text, str) or not text.strip() or len(text) > 4000:
        raise ValueError("text must contain 1–4000 characters")
    history = payload.get("history", [])
    if not isinstance(history, list) or len(history) > 10:
        raise ValueError("history must contain at most 10 turns")
    for turn in history:
        if not isinstance(turn, dict) or not isinstance(turn.get("text"), str):
            raise ValueError("Invalid history turn")
        if len(turn["text"]) > 4000 or not isinstance(turn.get("scenario", ""), str):
            raise ValueError("Invalid history turn")
    for key in ("active", "last_bot"):
        if payload.get(key) is not None and (not isinstance(payload[key], str) or len(payload[key]) > 4000):
            raise ValueError(f"Invalid {key}")
    return {"text": text.strip(), "history": history,
            "active": payload.get("active"), "last_bot": payload.get("last_bot")}


def make_handler(router):
    class Handler(BaseHTTPRequestHandler):
        def send_json(self, status, data):
            body = json.dumps(data, ensure_ascii=False).encode()
            self.send_response(status)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self):
            if self.path == "/healthz":
                self.send_json(200, {"ok": True, "model": router.model})
            else:
                self.send_json(404, {"error": "Not found"})

        def do_POST(self):
            if self.path != "/api/route":
                self.send_json(404, {"error": "Not found"})
                return
            try:
                size = int(self.headers.get("Content-Length", "0"))
                if not 0 < size <= MAX_BODY:
                    raise ValueError("Invalid request size")
                args = validate(json.loads(self.rfile.read(size)))
            except (ValueError, UnicodeDecodeError):
                self.send_json(400, {"error": "Invalid route request"})
                return
            try:
                result = router.route(**args)
                # Log measurements only: no utterances, credentials or upstream error bodies.
                print(json.dumps({k: result.get(k) for k in
                      ("model", "provider", "route_ms", "prompt_tokens", "completion_tokens")}), flush=True)
                self.send_json(200, result)
            except RouterError:
                self.send_json(502, {"error": "LLM недоступна. Проверьте настройки core-llm и повторите запрос."})
    return Handler


if __name__ == "__main__":
    load_env()
    router = Router()
    server = ThreadingHTTPServer((os.environ.get("CORE_LLM_HOST", "127.0.0.1"),
                                 int(os.environ.get("CORE_LLM_PORT", "8090"))), make_handler(router))
    print(f"core-llm listening on {server.server_address}, model={router.model}", flush=True)
    server.serve_forever()
