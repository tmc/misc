"""Positive and refusal controls for retained missing-usage evidence."""
import base64
import importlib.util
import io
import http.client
import http.server
import threading
import socket
import time
from unittest import mock
import json
import hashlib
import subprocess
import sys
import pathlib
import tempfile
import unittest

ROOT = pathlib.Path(__file__).parent
spec = importlib.util.spec_from_file_location('missing_usage', ROOT / 'aiperf-missing-usage-verify.py')
verify = importlib.util.module_from_spec(spec)
spec.loader.exec_module(verify)
spec = importlib.util.spec_from_file_location('observer', ROOT / 'aiperf-live-endpoints.py')
observer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(observer)


def encoded(value):
    return base64.b64encode(value).decode()


def fixture():
    rows = []
    for arm in ('xk6', 'aiperf'):
        for index in range(6):
            start = (index + (10 if arm == 'aiperf' else 0)) * 100_000_000
            body = b'data: {"choices":[{"index":0,"delta":{"content":"blue"}}]}\n\ndata: [DONE]\n\n'
            rows.append(dict(arm=arm, start_ns=start, start_wall_ns=start,
                end_ns=start+10_000_000, status=200,
                request_body_base64=encoded(b'{"stream":true,"max_tokens":32}'),
                chunks=[dict(ns=start+1, base64=encoded(body))]))
    records = [dict(request_id=str(i), phase='measurement', outcome='success',
        protocol='openai-chat', stream=True, dispatched_at=rows[i+2]['start_wall_ns']/1e6,
        input_tokens=dict(value=None, source='unavailable'),
        output_tokens=dict(value=None, source='unavailable')) for i in range(4)]
    run = dict(records=records, input_throughput=None, output_throughput=None)
    for key in ('attempts', 'successes', 'missing_input_counts', 'missing_output_counts'):
        run[key] = 4
    for key in ('failures', 'dropped_records', 'excluded_boundary_requests', 'inflight_at_end', 'estimated_input_counts', 'estimated_output_counts'):
        run[key] = 0
    exports = [dict(metadata=dict(x_request_id=str(i), request_start_ns=rows[i+6]['start_wall_ns'],
        benchmark_phase='warmup' if i < 2 else 'profiling', was_cancelled=False), metrics={}) for i in range(6)]
    return rows, run, exports


class MissingUsageTests(unittest.TestCase):
    def archive(self, rows, run, exports):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        path = pathlib.Path(directory.name)
        (path/'xk6').mkdir()
        (path/'aiperf').mkdir()
        (path/'wire.jsonl').write_text(''.join(json.dumps(row)+'\n' for row in rows))
        (path/'xk6'/'run.json').write_text(json.dumps(run))
        (path/'aiperf'/'profile_export.jsonl').write_text(''.join(json.dumps(row)+'\n' for row in exports))
        (path/'aiperf'/'profile_export_aiperf.json').write_text(json.dumps(dict(request_count=dict(avg=4), error_summary={})))
        return path

    def test_positive(self):
        result = verify.verify(self.archive(*fixture()))
        self.assertEqual(result['status'], 'SOFTWARE_PASS')
        self.assertEqual(result['missingusage'], 'OBSERVED')

    def test_zero_and_partial_usage_are_not_missing(self):
        for usage in ({'prompt_tokens': 0, 'completion_tokens': 0}, {'prompt_tokens': 3}, {}):
            rows, run, exports = fixture()
            body = b'data: '+json.dumps(dict(choices=[dict(delta=dict(content='blue'))], usage=usage)).encode()+b'\n\ndata: [DONE]\n\n'
            rows[0]['chunks'][0]['base64'] = encoded(body)
            result = verify.verify(self.archive(rows, run, exports))
            self.assertEqual((result['status'], result['missingusage']), ('SOFTWARE_PASS', 'UNAVAILABLE'))

    def test_cli_policy_and_payload_refusal(self):
        path = self.archive(*fixture())
        (path/'PREREGISTRATION.md').write_text('authored fixture plan')
        plan = {key: 'fixture' for key in ('source_revision', 'binary_sha256', 'aiperf_revision', 'model_digest', 'runtime_identity')}
        plan['payload_sha256'] = hashlib.sha256(b'{"max_tokens":32,"stream":true}').hexdigest()
        plan['preregistration_sha256'] = hashlib.sha256((path/'PREREGISTRATION.md').read_bytes()).hexdigest()
        policy = path/'policy.json'
        policy.write_text(json.dumps(plan))
        digest = hashlib.sha256(policy.read_bytes()).hexdigest()
        command = [sys.executable, str(ROOT/'aiperf-missing-usage-verify.py'), str(path), '--plan', str(policy), '--plan-sha256']
        result = subprocess.run(command+[digest], capture_output=True, text=True, timeout=5)
        self.assertEqual(result.returncode, 0, result.stdout+result.stderr)
        self.assertEqual(json.loads(result.stdout)['missingusage'], 'OBSERVED')
        result = subprocess.run(command+['0'*64], capture_output=True, text=True, timeout=5)
        self.assertEqual(result.returncode, 1)
        self.assertIn('SHA256 mismatch', result.stdout)
        with self.assertRaisesRegex(ValueError, 'pinned policy'):
            verify.verify(path, '0'*64)

    def test_reported_zero_measured_accounting_independent(self):
        rows, run, exports = fixture()
        usage = dict(prompt_tokens=0, completion_tokens=0, total_tokens=0)
        body = b'data: '+json.dumps(dict(choices=[dict(delta=dict(content='blue'))], usage=usage)).encode()+b'\n\ndata: [DONE]\n\n'
        for row in rows:
            row['chunks'][0]['base64'] = encoded(body)
        for direction in ('input', 'output'):
            run['missing_'+direction+'_counts'] = 0
            run[direction+'_throughput'] = 0
            for record in run['records']:
                record[direction+'_tokens'] = dict(value=0, source='reported')
        for record in exports:
            record['metrics'] = {'usage_'+key: dict(value=value) for key, value in usage.items()}
        result = verify.verify(self.archive(rows, run, exports))
        self.assertEqual((result['status'], result['missingusage']), ('SOFTWARE_PASS', 'UNAVAILABLE'))
        run['records'][0]['output_tokens']['value'] = 1
        with self.assertRaises(ValueError):
            verify.verify(self.archive(rows, run, exports))

    def test_malformed_usage_refused(self):
        for usage in ({'prompt_tokens': -1}, {'prompt_tokens': True}, {'Prompt_tokens': 2},
                      {'prompt_tokens': 1, 'completion_tokens': 2, 'total_tokens': 4}, []):
            rows, _, _ = fixture()
            body = b'data: '+json.dumps(dict(choices=[dict(delta=dict(content='blue'))], usage=usage)).encode()+b'\n\ndata: [DONE]\n\n'
            rows[0]['chunks'][0]['base64'] = encoded(body)
            with self.subTest(usage=usage), self.assertRaises(ValueError):
                verify.wire(rows[0])

    def test_accounting_refusals(self):
        mutations = [lambda r: r.update(attempts=3), lambda r: r.update(inflight_at_end=1),
            lambda r: r.update(dropped_records=1), lambda r: r.update(output_throughput=0),
            lambda r: r['records'][0].update(input_tokens=dict(value=0, source='reported')),
            lambda r: r['records'][0].update(phase='warmup'),
            lambda r: r['records'][0].update(dispatched_at=-1000)]
        for mutate in mutations:
            with self.subTest(mutate=mutate):
                rows, run, exports = fixture()
                mutate(run)
                with self.assertRaises(ValueError):
                    verify.verify(self.archive(rows, run, exports))

    def test_wire_refusals(self):
        bodies = [b'data: {"usage":null,"usage":{}}\n\ndata: [DONE]\n\n',
            b'data: {"Usage":{}}\n\ndata: [DONE]\n\n',
            b'data: {"choices":[{"delta":{"content":3}}]}\n\ndata: [DONE]\n\n',
            b'data: {"error":"failure"}\n\ndata: [DONE]\n\n',
            b'data: {"choices":[{"delta":{"content":"blue"}}]}\n\n',
            b'data: [DONE]\n\ndata: [DONE]\n\n']
        for body in bodies:
            rows, _, _ = fixture()
            rows[0]['chunks'][0]['base64'] = encoded(body)
            with self.subTest(body=body), self.assertRaises(ValueError):
                verify.wire(rows[0])

    def test_auto_include_usage_payload_mismatch(self):
        rows, run, exports = fixture()
        rows[6]['request_body_base64'] = encoded(b'{"stream":true,"max_tokens":32,"stream_options":{"include_usage":true}}')
        with self.assertRaisesRegex(ValueError, 'forwarded payloads differ'):
            verify.verify(self.archive(rows, run, exports))

    def test_aiperf_controls(self):
        for mutate in (lambda e: e[0]['metadata'].update(was_cancelled=True),
                       lambda e: e[0]['metadata'].update(benchmark_phase='profiling'),
                       lambda e: e[0]['metrics'].update(usage_prompt_tokens=dict(value=0)),
                       lambda e: e[0]['metadata'].update(request_start_ns=-100_000_000)):
            rows, run, exports = fixture()
            mutate(exports)
            with self.assertRaises(ValueError):
                verify.verify(self.archive(rows, run, exports))

    def test_chunk_bytes_and_crlf(self):
        rows, _, _ = fixture()
        raw = base64.b64decode(rows[0]['chunks'][0]['base64']).replace(b'\n', b'\r\n')
        rows[0]['chunks'] = [dict(ns=1, base64=encoded(raw[:13])), dict(ns=2, base64=encoded(raw[13:]))]
        self.assertIsNone(verify.wire(rows[0])[1])

    def test_observer_refuses_unsafe_origin(self):
        for url in ('file:///x', 'http://u:p@localhost', 'http://localhost/path', 'http://localhost?q=1', 'http://localhost', 'http://example.com'):
            with self.subTest(url=url), self.assertRaises(ValueError):
                observer.serve(url, io.StringIO(), 'xk6', 12000, duration=0)

    def test_actual_http_bytes_and_status(self):
        received = []
        response_body = b'data: {"choices":[]}\r\n\r\ndata: [DONE]\r\n\r\n'

        class Upstream(http.server.BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                received.append((self.path, self.rfile.read(int(self.headers['Content-Length']))))
                self.send_response(200)
                self.send_header('Content-Type', 'text/event-stream')
                self.end_headers()
                self.wfile.write(response_body)

        server = http.server.HTTPServer(('127.0.0.1', 0), Upstream)
        upstream_thread = threading.Thread(target=server.handle_request)
        upstream_thread.start()
        output, ready, errors, address = io.StringIO(), threading.Event(), [], []
        original = observer._DeadlineServer

        def create_server(*args, **kwargs):
            observed = original(*args, **kwargs)
            address.append(observed.server_port)
            ready.set()
            return observed

        def run():
            try:
                observer.serve('http://127.0.0.1:'+str(server.server_port), output,
                    'xk6', 0, max_requests=1, duration=3)
            except Exception as error:
                errors.append(error)

        try:
            with mock.patch.object(observer, '_DeadlineServer', create_server):
                thread = threading.Thread(target=run)
                thread.start()
                self.assertTrue(ready.wait(2))
                client = http.client.HTTPConnection('127.0.0.1', address[0], timeout=2)
                body = b'{"stream":true,"max_tokens":32}'
                client.request('POST', '/v1/chat/completions', body,
                    {'Content-Type': 'application/json', 'Authorization': 'test-secret'})
                response = client.getresponse()
                self.assertEqual((response.status, response.read()), (200, response_body))
                client.close()
                thread.join(4)
                self.assertFalse(thread.is_alive())
            upstream_thread.join(4)
            self.assertFalse(upstream_thread.is_alive())
            self.assertEqual(errors, [])
            self.assertEqual(received, [('/v1/chat/completions', body)])
            row = json.loads(output.getvalue())
            self.assertEqual(base64.b64decode(row['request_body_base64']), body)
            self.assertEqual(b''.join(base64.b64decode(c['base64']) for c in row['chunks']), response_body)
            self.assertNotIn('test-secret', output.getvalue())
            self.assertLess(row['start_ns'], row['end_ns'])
        finally:
            server.server_close()

    def test_idle_and_partial_headers_bounded(self):
        for partial in (b'', b'POST /v1/chat/completions HTTP/1.1\r\nHost: localhost\r\nX-Slow: '):
            with self.subTest(partial=partial):
                output, ready, address, errors = io.StringIO(), threading.Event(), [], []
                original = observer._DeadlineServer

                def create_server(*args, **kwargs):
                    server = original(*args, **kwargs)
                    address.append(server.server_port)
                    ready.set()
                    return server

                def run():
                    try:
                        observer.serve('http://127.0.0.1:1', output, 'xk6', 0, duration=.3)
                    except Exception as error:
                        errors.append(error)

                started = time.monotonic()
                with mock.patch.object(observer, '_DeadlineServer', create_server):
                    thread = threading.Thread(target=run)
                    thread.start()
                    self.assertTrue(ready.wait(1))
                    client = socket.create_connection(('127.0.0.1', address[0]), timeout=1)
                    try:
                        if partial:
                            client.sendall(partial)
                        # Keep the client connected: only the observer deadline may unblock it.
                        thread.join(1.5)
                        self.assertFalse(thread.is_alive(), 'accepted socket bypassed cell deadline')
                        self.assertEqual(client.recv(1), b'')
                    finally:
                        client.close()
                        thread.join(1)
                self.assertLess(time.monotonic()-started, 1.5)
                self.assertEqual(errors, [])
                if output.getvalue():
                    self.assertIn('error', json.loads(output.getvalue()))

    def test_slow_upstream_headers_bounded(self):
        release, upstream_started = threading.Event(), threading.Event()

        class Slow(http.server.BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                self.rfile.read(int(self.headers['Content-Length']))
                self.connection.sendall(b'HTTP/1.1 200 OK\r\nX-Slow: ')
                upstream_started.set()
                release.wait(2)

        upstream = http.server.HTTPServer(('127.0.0.1', 0), Slow)
        upstream_thread = threading.Thread(target=upstream.handle_request)
        upstream_thread.start()
        output, ready, address, errors = io.StringIO(), threading.Event(), [], []
        original = observer._DeadlineServer

        def create_server(*args, **kwargs):
            server = original(*args, **kwargs)
            address.append(server.server_port)
            ready.set()
            return server

        def run():
            try:
                observer.serve('http://127.0.0.1:'+str(upstream.server_port),
                    output, 'xk6', 0, max_requests=1, duration=.3)
            except Exception as error:
                errors.append(error)

        try:
            with mock.patch.object(observer, '_DeadlineServer', create_server):
                thread = threading.Thread(target=run)
                thread.start()
                self.assertTrue(ready.wait(1))
                client = http.client.HTTPConnection('127.0.0.1', address[0], timeout=1)
                client.request('POST', '/', b'{"stream":true}')
                self.assertTrue(upstream_started.wait(1))
                thread.join(1.5)
                self.assertFalse(thread.is_alive(), 'upstream headers bypassed absolute deadline')
                client.close()
            self.assertEqual(errors, [])
            self.assertIn('error', json.loads(output.getvalue()))
        finally:
            release.set()
            upstream_thread.join(2)
            upstream.server_close()

    def test_single_literal_connect_remaining_deadline(self):
        fake = mock.Mock()
        deadline = time.monotonic()+.2
        with mock.patch.object(observer.socket, 'socket', return_value=fake), mock.patch.object(observer.socket, 'getaddrinfo', side_effect=AssertionError('DNS forbidden')):
            connection = observer._LiteralConnection('127.0.0.1', 12345, deadline=deadline)
            connection.connect()
        fake.connect.assert_called_once_with(('127.0.0.1', 12345))
        self.assertGreater(fake.settimeout.call_args.args[0], 0)
        self.assertLessEqual(fake.settimeout.call_args.args[0], .2)
        with mock.patch.object(observer.socket, 'socket', return_value=fake):
            connection = observer._LiteralConnection('127.0.0.1', 12345, deadline=time.monotonic()-1)
            with self.assertRaises(TimeoutError):
                connection.connect()
        fake.connect.assert_called_once()

    def test_actual_literal_connect_success_and_refusal(self):
        listener = socket.socket()
        listener.bind(('127.0.0.1', 0))
        listener.listen(1)
        port = listener.getsockname()[1]
        try:
            with mock.patch.object(observer.socket, 'getaddrinfo', side_effect=AssertionError('DNS forbidden')):
                connection = observer._LiteralConnection('127.0.0.1', port, deadline=time.monotonic()+.2)
                connection.connect()
                accepted, _ = listener.accept()
                accepted.close()
                connection.close()
        finally:
            listener.close()
        started = time.monotonic()
        connection = observer._LiteralConnection('127.0.0.1', port, deadline=started+.2)
        try:
            with self.assertRaises(OSError):
                connection.connect()
        finally:
            connection.close()
        self.assertLess(time.monotonic()-started, .5)

    def test_header_redaction(self):
        self.assertEqual(observer.headers([('Authorization', 'secret'), ('Content-Type', 'application/json')]),
            [['Authorization', '[REDACTED]'], ['Content-Type', 'application/json']])


if __name__ == '__main__':
    unittest.main()
