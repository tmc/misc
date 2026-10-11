import importlib.util
import json
import http.server
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import unittest

SCRIPT = Path(__file__).with_name("template-compat.py")
spec = importlib.util.spec_from_file_location("template_compat", SCRIPT)
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)


class TemplateCompatibilityTest(unittest.TestCase):
    def render(self, template, variables=None, extra=None, turn_extra=None):
        return bridge.evaluate({"schema_version": 1, "operation": "render", "template": template,
                                "variables": variables or {}, "extra": extra or {}, "turn_extra": turn_extra or {}})

    def test_upstream_render_semantics(self):
        from jinja2 import Environment
        templates = [
            '{"text": {{ texts|tojson }}}',
            '{"values": [{% for item in texts %}{{ item|upper|tojson }}{% if not loop.last %},{% endif %}{% endfor %}]}',
            '{"value": {% if enabled %}{{ texts|join(" ")|tojson }}{% else %}null{% endif %}}',
            '{% macro field(x) %}{{ x|tojson }}{% endmacro %}{"value": {{ field(texts) }}}',
        ]
        variables = {"texts": ["a<", "b\""], "enabled": True}
        for template in templates:
            with self.subTest(template=template):
                expected = json.loads(Environment(autoescape=True).from_string(template).render(**variables))
                self.assertEqual(self.render(template, variables)["value"], expected)

    def test_precedence_and_receipt(self):
        result = self.render('{"x":1}', extra={"x": 2}, turn_extra={"x": 3})
        self.assertEqual(result["value"], {"x": 3})
        self.assertEqual(result["provenance"]["value_sha256"], bridge.digest({"x": 3}))
        self.assertEqual(result, self.render('{"x":1}', extra={"x": 2}, turn_extra={"x": 3}))

    def test_jmespath_projection_filter_and_functions(self):
        response = {"items": [{"score": 0, "text": "a"}, {"score": 2, "text": "b"}]}
        for expression, expected in [("items[?score > `0`].text", ["b"]), ("join(' ', items[].text)", "a b"), ("items[1].text", "b")]:
            result = bridge.evaluate({"schema_version": 1, "operation": "select", "expression": expression, "response": response})
            self.assertEqual(result["value"], expected)

    def test_unmatched_selection_rejects(self):
        for value in [None, [], {}, "", 0, False]:
            with self.assertRaises(ValueError):
                bridge.evaluate({"schema_version": 1, "operation": "select", "expression": "text", "response": {"text": value}})

    def test_unsafe_or_invalid_template_refuses(self):
        for template in ['{"x": {{ cycler.__init__.__globals__|tojson }}}', '{"x":1,"x":2}', '[]', '{"x":NaN}', '{"x":"{{ "a" * 1048577 }}"}']:
            with self.subTest(template=template), self.assertRaises(Exception):
                self.render(template)

    def test_cli_and_private_error(self):
        request = {"schema_version": 1, "operation": "render", "template": '{"text":{{ text|tojson }}}', "variables": {"text": "credential-secret"}, "extra": {}, "turn_extra": {}}
        run = subprocess.run([sys.executable, str(SCRIPT)], input=json.dumps(request).encode(), capture_output=True, timeout=10)
        self.assertEqual(run.returncode, 0, run.stderr)
        self.assertEqual(json.loads(run.stdout)["value"], {"text": "credential-secret"})
        request["template"] = "credential-secret invalid JSON"
        run = subprocess.run([sys.executable, str(SCRIPT)], input=json.dumps(request).encode(), capture_output=True, timeout=10)
        self.assertEqual(run.returncode, 1)
        self.assertNotIn(b"credential-secret", run.stderr)
        self.assertEqual(run.stdout, b"")

    @unittest.skipUnless(os.environ.get("K6_SMOKE_BINARY"), "set K6_SMOKE_BINARY for actual extension invocation")
    def test_actual_k6_authored_payload(self):
        received = []
        class Handler(http.server.BaseHTTPRequestHandler):
            def do_POST(self):
                received.append(json.loads(self.rfile.read(int(self.headers["Content-Length"]))))
                body = b'{"result":{"text":"selected fixture"}}'
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
            def log_message(self, *_args):
                pass
        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        work = Path.home() / "tmp/xk6-llm-perf/20261010-parity-contracts/tmp"
        work.mkdir(parents=True, exist_ok=True)
        try:
            with tempfile.TemporaryDirectory(dir=work) as temporary:
                artifact = Path(temporary) / "rendered.json"
                artifact.write_bytes(bridge.encode(self.render('{"input": {{ texts|map("upper")|list|tojson }}}', {"texts": ["hello", "world"]})))
                environment = dict(os.environ, BASE_URL="http://127.0.0.1:" + str(server.server_port), TEMPLATE_FILE=str(artifact))
                run = subprocess.run([os.environ["K6_SMOKE_BINARY"], "run", "--quiet", str(SCRIPT.parent.parent / "examples/template-compat.js")], env=environment, capture_output=True, timeout=30)
                self.assertEqual(run.returncode, 0, run.stderr)
                self.assertEqual(received, [{"input": ["HELLO", "WORLD"]}])
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=3)


if __name__ == "__main__":
    unittest.main()
