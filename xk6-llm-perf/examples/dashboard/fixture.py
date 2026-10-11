#!/usr/bin/env python3
"""Local streaming fixture; timing exercises charts, not model performance."""
import argparse
import json
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Timer


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers.get('Content-Length', 0))))
        if self.path != '/v1/chat/completions' or not body.get('stream'):
            self.send_error(400, 'streaming chat completions required')
            return
        if self.server.fail:
            self.send_error(503, 'intentional fixture failure')
            return
        self.send_response(200)
        self.send_header('Content-Type', 'text/event-stream')
        self.end_headers()
        try:
            for delta in [{'reasoning_content': 'Counting.'}, {'content': 'One'}, {'content': ' two'}, {'content': ' three'}, {'content': ' four.'}]:
                time.sleep(0.04)
                self.event({'choices': [{'index': 0, 'delta': delta}]})
            time.sleep(0.08)
            self.event({'choices': [{'index': 0, 'delta': {}, 'finish_reason': 'stop'}], 'usage': {'prompt_tokens': 8, 'completion_tokens': 5, 'total_tokens': 13}})
            self.wfile.write(b'data: [DONE]\n\n')
            self.wfile.flush()
        except (BrokenPipeError, ConnectionResetError):
            pass

    def event(self, data):
        self.wfile.write(('data: ' + json.dumps(data) + '\n\n').encode())
        self.wfile.flush()

    def log_message(self, *args):
        pass


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--port', type=int, default=8088)
    parser.add_argument('--fail', action='store_true')
    parser.add_argument('--duration', type=float, default=0,
                        help='stop after this many seconds (zero runs until interrupted)')
    args = parser.parse_args()
    server = ThreadingHTTPServer(('127.0.0.1', args.port), Handler)
    server.fail = args.fail
    print('Fixture listening on http://127.0.0.1:%d/v1' % args.port, flush=True)
    if args.duration > 0:
        Timer(args.duration, server.shutdown).start()
    server.serve_forever()
