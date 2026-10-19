#!/usr/bin/env python3
"""Evaluate trusted authored templates with the pinned AIPerf typed runtime."""
from dataclasses import asdict
import hashlib
import importlib.metadata
import json
import math
from pathlib import Path
import resource
import signal
import sys

REFERENCE = 'f10eb50a1776775d80d7f13396b0f668f14e2c03'
SOURCE_HASH = '144bd0b340d8405ddf0b91a01368a009dd48da508b9560387cc130aacedac716'
PLUGIN_HASH = 'd10bf1dded99edfb98968e470b407d875bf24c7651c001d90c8c6403f05191de'
MAX_BYTES = 1 << 20


def encode(value):
    data = json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(',', ':'), allow_nan=False).encode()
    if len(data) > MAX_BYTES:
        raise ValueError('output exceeds 1 MiB')
    return data


def sha(data):
    return hashlib.sha256(data).hexdigest()


def decode(data):
    if len(data) > MAX_BYTES:
        raise ValueError('input exceeds 1 MiB')
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError('duplicate JSON member')
            result[key] = value
        return result
    def constant(value):
        raise ValueError('nonfinite JSON number')
    value = json.loads(data, object_pairs_hook=pairs, parse_constant=constant)
    def check(item, depth=0):
        if depth > 64:
            raise ValueError('JSON depth exceeds 64')
        if isinstance(item, float) and not math.isfinite(item):
            raise ValueError('nonfinite JSON number')
        if isinstance(item, dict):
            for child in item.values():
                check(child, depth + 1)
        elif isinstance(item, list):
            for child in item:
                check(child, depth + 1)
    check(value)
    return value


def runtime_identity():
    # Locate without executing aiperf.__init__ or endpoint modules first.
    distribution = importlib.metadata.distribution('aiperf')
    root = Path(distribution.locate_file('aiperf'))
    hashes = {str(p.relative_to(root)): sha(p.read_bytes()) for p in sorted(root.rglob('*.py'))}
    tree = json.dumps(hashes, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()
    if len(hashes) != 1025 or sha(tree) != SOURCE_HASH:
        raise ValueError('AIPerf source differs from pinned reference')
    if sha((root / 'plugin/plugins.yaml').read_bytes()) != PLUGIN_HASH or distribution.version != '0.13.0':
        raise ValueError('AIPerf package or plugin defaults differ from pin')
    return {'reference': REFERENCE, 'source_sha256': SOURCE_HASH, 'plugins_sha256': PLUGIN_HASH,
            'python': sys.version, 'packages': {d.metadata['Name']: d.version for d in importlib.metadata.distributions()},
            'files': hashes}


def evaluate(value):
    if not isinstance(value, dict) or set(value) != {'schema_version', 'request_info', 'responses'} or type(value['schema_version']) is not int or value['schema_version'] != 1:
        raise ValueError('invalid input fields or schema version')
    if not isinstance(value['responses'], list) or len(value['responses']) > 256:
        raise ValueError('responses must be a list of at most 256 captured text chunks')
    identity = runtime_identity()
    from aiperf.common.models import RequestInfo
    from aiperf.common.models.record_models import TextResponse
    from aiperf.endpoints.template_endpoint import TemplateEndpoint
    from pydantic import TypeAdapter
    request = RequestInfo.model_validate(value['request_info'], extra='forbid')
    if request.model_endpoint.endpoint.type != 'template':
        raise ValueError('endpoint type must be template')
    endpoint = TemplateEndpoint(model_endpoint=request.model_endpoint)
    payload = endpoint.format_payload(request)
    if not isinstance(payload, dict):
        raise ValueError('rendered payload must be an object')
    parsed = []
    for chunk in value['responses']:
        response = TypeAdapter(TextResponse).validate_python(chunk)
        result = endpoint.parse_response(response)
        parsed.append(None if result is None else asdict(result))
    return {'schema_version': 1, 'payload': payload, 'parsed_responses': parsed,
            'inputs': {'data': [{'session_id': request.conversation_id, 'payloads': [payload]}]},
            'provenance': identity, 'qualification': 'offline typed preprocessing and captured TextResponse parsing only'}


def main():
    # These limits bound a trusted-template worker; they are not a security sandbox.
    resource.setrlimit(resource.RLIMIT_CPU, (20, 20))
    resource.setrlimit(resource.RLIMIT_AS, (2 << 30, 2 << 30))
    signal.alarm(30)
    raw = sys.stdin.buffer.read(MAX_BYTES + 1)
    result = evaluate(decode(raw))
    result['input_sha256'] = sha(raw)
    result['adapter_sha256'] = sha(Path(__file__).read_bytes())
    output = encode(result)
    decode(output)  # Apply the same nesting and finite-value gate to generated output.
    sys.stdout.buffer.write(output + b'\n')


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # Omit exception messages; upstream logging may still contain template data.
        print('aiperf-template: evaluation failed (' + type(error).__name__ + ')', file=sys.stderr)
        sys.exit(1)
