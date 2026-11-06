#!/usr/bin/env python3
"""Minimal path-style S3/R2 stub for disposable Finance attachment tests."""

from __future__ import annotations

import os
import pathlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT = pathlib.Path(os.environ.get("FINANCE_E2E_OBJECT_ROOT", "/tmp/finance-e2e-objects"))
HOST = os.environ.get("FINANCE_E2E_S3_HOST", "127.0.0.1")
PORT = int(os.environ.get("FINANCE_E2E_S3_PORT", "19000"))
PID_FILE = pathlib.Path(os.environ.get("FINANCE_E2E_S3_PID", "/tmp/finance-e2e-s3.pid"))


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt: str, *args) -> None:  # noqa: A003
        return

    def do_PUT(self) -> None:  # noqa: N802
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length)
        path = ROOT / self.path.lstrip("/")
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(body)
        self.send_response(200)
        self.end_headers()

    def do_GET(self) -> None:  # noqa: N802
        path = ROOT / self.path.lstrip("/")
        if not path.is_file():
            self.send_response(404)
            self.end_headers()
            return
        data = path.read_bytes()
        self.send_response(200)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_HEAD(self) -> None:  # noqa: N802
        path = ROOT / self.path.lstrip("/")
        if not path.is_file():
            self.send_response(404)
            self.end_headers()
            return
        self.send_response(200)
        self.send_header("Content-Length", str(path.stat().st_size))
        self.end_headers()


def main() -> None:
    ROOT.mkdir(parents=True, exist_ok=True)
    server = ThreadingHTTPServer((HOST, PORT), Handler)
    PID_FILE.write_text(str(os.getpid()), encoding="utf-8")
    print(f"s3_stub listening on http://{HOST}:{PORT}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
