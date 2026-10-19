#!/usr/bin/env python3
"""Bounded loopback observer for an existing, coordinator-leased endpoint.

Only forwards authored JSON POST bodies. Never inserts or strips usage options.
Run tools separately against this observer; it does not launch model workloads.
"""
import argparse
import base64
import hashlib
import http.client
import http.server
import ipaddress
import json
import pathlib
import socket
import ssl
import threading
import time
import urllib.parse

LIMIT = 4 * 1024 * 1024
HOP = {'connection', 'keep-alive', 'proxy-authenticate', 'proxy-authorization',
       'te', 'trailer', 'transfer-encoding', 'upgrade', 'host', 'content-length'}
SECRET = {'authorization', 'cookie', 'set-cookie', 'x-api-key', 'api-key'}


def headers(items):
    return [[k, '[REDACTED]' if k.lower() in SECRET else v] for k, v in items]


def close_read_write(sock):
    # shutdown interrupts a blocked read/write even when a file wrapper owns it.
    try:
        sock.shutdown(socket.SHUT_RDWR)
    except OSError:
        pass


def deadline_timer(sock, deadline):
    timer = threading.Timer(max(0, deadline-time.monotonic()), close_read_write, (sock,))
    timer.daemon = True
    timer.start()
    return timer


class _DeadlineServer(http.server.HTTPServer):
    def __init__(self, *args, deadline):
        self.deadline = deadline
        self.request_timer = None
        super().__init__(*args)

    def get_request(self):
        request, address = super().get_request()
        # This runs before BaseHTTPRequestHandler reads request line or headers.
        self.request_deadline = min(self.deadline, time.monotonic()+30)
        request.settimeout(max(.001, self.request_deadline-time.monotonic()))
        self.request_timer = deadline_timer(request, self.request_deadline)
        return request, address

    def shutdown_request(self, request):
        if self.request_timer:
            self.request_timer.cancel()
            self.request_timer = None
        super().shutdown_request(request)


class _LiteralConnection(http.client.HTTPConnection):
    def __init__(self, host, port, *, deadline, secure=False):
        self.deadline = deadline
        self.secure = secure
        super().__init__(host, port, timeout=max(.001, deadline-time.monotonic()))

    def connect(self):
        # One numeric address, one connect attempt; no getaddrinfo or fallback.
        address = ipaddress.ip_address(self.host)
        family = socket.AF_INET6 if address.version == 6 else socket.AF_INET
        self.sock = socket.socket(family, socket.SOCK_STREAM)
        self.sock.settimeout(max(.001, self.deadline-time.monotonic()))
        if time.monotonic() >= self.deadline:
            raise TimeoutError('upstream connect deadline')
        self.sock.connect((str(address), self.port))
        if self.secure:
            self.sock.settimeout(max(.001, self.deadline-time.monotonic()))
            if time.monotonic() >= self.deadline:
                raise TimeoutError('upstream TLS deadline')
            self.sock = ssl.create_default_context().wrap_socket(self.sock, server_hostname=self.host)


def serve(upstream, output, arm, port, max_requests=6, duration=300):
    target = urllib.parse.urlsplit(upstream)
    if target.scheme not in ('http', 'https') or not target.hostname or target.username or target.password or target.query or target.fragment:
        raise ValueError('upstream must be an HTTP origin without credentials/query/fragment')
    if target.path not in ('', '/'):
        raise ValueError('upstream must be an origin; request path is forwarded exactly')
    try:
        ipaddress.ip_address(target.hostname)
    except ValueError:
        raise ValueError('upstream host must be a literal IP address') from None
    deadline = time.monotonic() + duration

    class Handler(http.server.BaseHTTPRequestHandler):
        protocol_version = 'HTTP/1.0'

        def log_message(self, *args):
            pass

        def do_POST(self):
            nonlocal remaining
            if remaining <= 0 or time.monotonic() >= deadline:
                self.send_error(429, 'observer budget exhausted')
                return
            remaining -= 1
            row = dict(arm=arm, path=self.path, start_ns=time.monotonic_ns(),
                       start_wall_ns=time.time_ns(), request_headers=headers(self.headers.items()), chunks=[])
            connection = None
            upstream_timer = None
            request_deadline = self.server.request_deadline
            try:
                if self.headers.get('Transfer-Encoding') or not self.headers.get('Content-Length', '').isdigit():
                    raise ValueError('requires bounded Content-Length request')
                length = int(self.headers['Content-Length'])
                if not 0 < length <= LIMIT:
                    raise ValueError('request body size')
                self.connection.settimeout(min(30, max(.001, request_deadline-time.monotonic())))
                body = self.rfile.read(length)
                if len(body) != length:
                    raise ValueError('truncated request body')
                json.loads(body)
                row['request_body_base64'] = base64.b64encode(body).decode()
                # Absolute-form URLs could redirect a supposedly fixed observer.
                if not self.path.startswith('/') or self.path.startswith('//') or '\r' in self.path or '\n' in self.path:
                    raise ValueError('invalid origin-form path')
                connection = _LiteralConnection(target.hostname, target.port or (443 if target.scheme == 'https' else 80), deadline=request_deadline, secure=target.scheme == 'https')
                connection.connect()
                upstream_socket = connection.sock
                upstream_timer = deadline_timer(upstream_socket, request_deadline)
                connection.request('POST', self.path, body, {k: v for k, v in self.headers.items() if k.lower() not in HOP})
                response = connection.getresponse()
                row.update(status=response.status, response_headers=headers(response.getheaders()))
                self.send_response(response.status)
                for key, value in response.getheaders():
                    if key.lower() not in HOP:
                        self.send_header(key, value)
                self.end_headers()
                received = 0
                while True:
                    if time.monotonic() >= request_deadline:
                        raise TimeoutError('observer request deadline')
                    upstream_socket.settimeout(max(.001, request_deadline-time.monotonic()))
                    chunk = response.read1(65536)
                    if not chunk:
                        break
                    received += len(chunk)
                    if received > LIMIT:
                        raise ValueError('response body exceeds 4 MiB')
                    row['chunks'].append(dict(ns=time.monotonic_ns(), base64=base64.b64encode(chunk).decode()))
                    if time.monotonic() >= request_deadline:
                        raise TimeoutError('observer request deadline')
                    self.connection.settimeout(max(.001, request_deadline-time.monotonic()))
                    self.wfile.write(chunk)
                    self.wfile.flush()
            except (ValueError, OSError, http.client.HTTPException) as error:
                row['error'] = type(error).__name__ + ': ' + str(error)
                if 'status' not in row:
                    row['status'] = 502
                    try:
                        self.send_error(502, 'observer forwarding failed')
                    except OSError:
                        pass
            finally:
                row['end_ns'] = time.monotonic_ns()
                if upstream_timer:
                    upstream_timer.cancel()
                if connection:
                    connection.close()
                output.write(json.dumps(row, separators=(',', ':')) + '\n')
                output.flush()

    remaining = max_requests
    server = _DeadlineServer(('127.0.0.1', port), Handler, deadline=deadline)
    try:
        while remaining and time.monotonic() < deadline:
            server.timeout = min(.25, max(0, deadline-time.monotonic()))
            server.handle_request()
    finally:
        server.server_close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--upstream', required=True)
    parser.add_argument('--output', type=pathlib.Path, required=True)
    parser.add_argument('--arm', choices=('xk6', 'aiperf'), required=True)
    parser.add_argument('--port', type=int, required=True)
    parser.add_argument('--lease-receipt', type=pathlib.Path, required=True)
    parser.add_argument('--lease-sha256', required=True)
    args = parser.parse_args()
    raw = args.lease_receipt.read_bytes()
    if len(raw) > 16384 or hashlib.sha256(raw).hexdigest() != args.lease_sha256:
        parser.error('lease receipt size or SHA256 mismatch')
    lease = json.loads(raw)
    # A receipt is identity evidence, not proof that the coordinator granted it.
    for key in ('source_revision', 'binary_sha256', 'aiperf_revision', 'runtime_identity', 'model_digest', 'plan_sha256', 'coordinator_lease'):
        if not isinstance(lease.get(key), str) or not lease[key]:
            parser.error('missing frozen lease identity: '+key)
    if not 1024 <= args.port <= 65535:
        parser.error('port must be 1024..65535')
    with args.output.open('x') as output:
        serve(args.upstream, output, args.arm, args.port)


if __name__ == '__main__':
    main()
