#!/usr/bin/env python3
"""Render offline authored Jinja requests or query captured JSON with JMESPath."""
import hashlib
import importlib.metadata
import json
import resource
import signal
import sys

MAX_BYTES = 1 << 20
PINS = {"Jinja2": "3.1.6", "jmespath": "1.0.1", "MarkupSafe": "3.0.3"}
REFERENCE = "f10eb50a1776775d80d7f13396b0f668f14e2c03"


def encode(value):
    data = json.dumps(value, allow_nan=False, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()
    if len(data) > MAX_BYTES:
        raise ValueError("JSON exceeds 1 MiB")
    return data


def decode(data):
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError("duplicate JSON key")
            result[key] = value
        return result
    def constant(value):
        raise ValueError("nonfinite JSON number")
    return json.loads(data, object_pairs_hook=pairs, parse_constant=constant)


def digest(value):
    return hashlib.sha256(encode(value)).hexdigest()


def evaluate(request):
    if not isinstance(request, dict) or request.get("schema_version") != 1 or type(request.get("schema_version")) is not int:
        raise ValueError("invalid schema version")
    versions = {name: importlib.metadata.version(name) for name in PINS}
    if versions != PINS:
        raise ValueError("dependency versions differ from pins")
    operation = request.get("operation")
    if operation == "render":
        if set(request) != {"schema_version", "operation", "template", "variables", "extra", "turn_extra"}:
            raise ValueError("invalid render fields")
        if not isinstance(request["template"], str) or not request["template"]:
            raise ValueError("missing template")
        for name in ("variables", "extra", "turn_extra"):
            if not isinstance(request[name], dict):
                raise ValueError("invalid " + name)
        from jinja2.sandbox import SandboxedEnvironment
        class BoundedEnvironment(SandboxedEnvironment):
            intercepted_binops = frozenset({"*", "**"})

            def call_binop(self, context, operator, left, right):
                if operator == "**":
                    raise ValueError("template exponentiation is unsupported")
                for sequence, count in ((left, right), (right, left)):
                    if isinstance(sequence, (str, list, tuple)) and isinstance(count, int):
                        if count < 0 or len(sequence) * count > MAX_BYTES:
                            raise ValueError("template multiplication exceeds size limit")
                return super().call_binop(context, operator, left, right)
        # JSON data only; object reflection and callable access are intentionally
        # narrower than upstream's trusted Python-object template context.
        environment = BoundedEnvironment(autoescape=True)
        template = environment.from_string(request["template"])
        chunks, size = [], 0
        for chunk in template.generate(**request["variables"]):
            size += len(chunk.encode())
            if size > MAX_BYTES:
                raise ValueError("rendered JSON exceeds 1 MiB")
            chunks.append(chunk)
        value = decode("".join(chunks))
        if not isinstance(value, dict):
            raise ValueError("rendered payload must be an object")
        # Same precedence as TemplateEndpoint.format_payload.
        value.update(request["extra"])
        value.update(request["turn_extra"])
    elif operation == "select":
        if set(request) != {"schema_version", "operation", "expression", "response"}:
            raise ValueError("invalid selection fields")
        expression = request["expression"]
        if not isinstance(expression, str) or not expression or len(expression.encode()) > 8192:
            raise ValueError("invalid response expression")
        import jmespath
        value = jmespath.compile(expression).search(request["response"])
        # Upstream treats false/empty explicit selections as parse failures.
        if not value:
            raise ValueError("response expression did not select a nonempty value")
    else:
        raise ValueError("unsupported operation")
    encode(value)
    return {"schema_version": 1, "operation": operation, "value": value,
            "provenance": {"reference_revision": REFERENCE, "dependency_versions": versions,
                           "scope": "offline-json-jinja-sandbox-jmespath-v1",
                           "request_sha256": digest(request), "value_sha256": digest(value)}}


def main():
    def timeout(_signum, _frame):
        raise ValueError("template evaluation exceeded 5 seconds")
    signal.signal(signal.SIGALRM, timeout)
    signal.alarm(5)
    try:
        resource.setrlimit(resource.RLIMIT_CPU, (5, 5))
        if sys.platform == "linux":
            resource.setrlimit(resource.RLIMIT_AS, (512 << 20, 512 << 20))
        resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
        data = sys.stdin.buffer.read(MAX_BYTES + 1)
        if len(data) > MAX_BYTES:
            raise ValueError("request exceeds 1 MiB")
        result = evaluate(decode(data))
        sys.stdout.buffer.write(encode(result) + b"\n")
    except Exception as error:
        # Templates and variables can contain credentials. Never echo either.
        sys.stderr.write("template compatibility evaluation failed (" + type(error).__name__ + ")\n")
        return 1
    finally:
        signal.alarm(0)
    return 0


if __name__ == "__main__":
    sys.exit(main())
