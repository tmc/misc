#!/usr/bin/env python3
"""Verify a retained C1 chat SSE cell with genuinely absent provider counts.

Does not qualify token estimates, costs, replay, slices, or performance parity.
"""
import argparse
import base64
import hashlib
import importlib.util
import json
import math
import pathlib

spec = importlib.util.spec_from_file_location('full_usage', pathlib.Path(__file__).with_name('aiperf-live-verify.py'))
full = importlib.util.module_from_spec(spec)
spec.loader.exec_module(full)
loads = full.loads


def read(path):
    raw = path.read_bytes()
    if len(raw) > 64 * 1024 * 1024:
        raise ValueError('artifact exceeds 64 MiB')
    return raw


def wire(row):
    if row['status'] != 200 or row.get('error'):
        raise ValueError('wire request failed')
    start, end = row['start_ns'], row['end_ns']
    if type(start) is not int or type(end) is not int or not start < end:
        raise ValueError('invalid request bounds')
    body = base64.b64decode(row['request_body_base64'], validate=True)
    request = loads(body)
    if request.get('stream') is not True:
        raise ValueError('body.stream must be true')
    previous, terminal, content = start, False, False
    usage = None
    chunks = []
    for event in row['chunks']:
        at = event['ns']
        if type(at) is not int or not previous <= at <= end:
            raise ValueError('invalid chunk timestamp')
        previous = at
        chunks.append(base64.b64decode(event['base64'], validate=True))
    raw = b''.join(chunks)
    # Parse SSE events, including multiline data and CRLF. Bytes remain retained.
    for event in raw.replace(b'\r\n', b'\n').split(b'\n\n'):
        data = b'\n'.join(line[5:].lstrip(b' ') for line in event.split(b'\n') if line.startswith(b'data:'))
        if not data:
            continue
        if terminal:
            raise ValueError('data after terminal')
        if data == b'[DONE]':
            terminal = True
            continue
        value = loads(data)
        if not isinstance(value, dict) or 'error' in value:
            raise ValueError('invalid provider event')
        if any(key.lower() == 'usage' and key != 'usage' for key in value):
            raise ValueError('usage case alias')
        if value.get('usage') is not None:
            if not isinstance(value['usage'], dict):
                raise ValueError('invalid usage object')
            current_usage = value['usage']
            for key, count in current_usage.items():
                if key.lower() in ('prompt_tokens', 'completion_tokens', 'total_tokens'):
                    if key not in ('prompt_tokens', 'completion_tokens', 'total_tokens') or (count is not None and (type(count) is not int or count < 0)):
                        raise ValueError('invalid usage count or alias')
            if usage is None:
                usage = {}
            usage.update({key: count for key, count in current_usage.items() if count is not None})
            if all(key in usage for key in ('prompt_tokens', 'completion_tokens', 'total_tokens')) and usage['prompt_tokens'] + usage['completion_tokens'] != usage['total_tokens']:
                raise ValueError('inconsistent usage counts')
        choices = value.get('choices', [])
        if not isinstance(choices, list) or len(choices) > 1:
            raise ValueError('unsupported choices')
        for choice in choices:
            if not isinstance(choice, dict) or type(choice.get('index', 0)) is not int or choice.get('index', 0) != 0:
                raise ValueError('unsupported choice identity')
            delta = choice.get('delta', {})
            if not isinstance(delta, dict):
                raise ValueError('invalid delta')
            for key in ('content', 'reasoning', 'reasoning_content'):
                text = delta.get(key)
                if text is not None and not isinstance(text, str):
                    raise ValueError('invalid delta text')
                content = content or bool(text)
    if not terminal or not content:
        raise ValueError('missing terminal or content')
    return request, usage


def verify(path, payload_sha256=None):
    rows = [loads(line) for line in read(path / 'wire.jsonl').splitlines()]
    if len(rows) != 12 or {r['arm'] for r in rows} != {'xk6', 'aiperf'}:
        raise ValueError('requires exactly two arms with two warmups and four measured requests each')
    payloads, observed_usage = set(), False
    groups = {}
    for arm in ('xk6', 'aiperf'):
        captured = sorted((r for r in rows if r['arm'] == arm), key=lambda r: r['start_ns'])
        if len(captured) != 6:
            raise ValueError('incorrect arm population')
        for prior, current in zip(captured, captured[1:]):
            if prior['end_ns'] > current['start_ns']:
                raise ValueError('C1 or warmup exclusion violated')
        for row in captured:
            request, present = wire(row)
            payloads.add(json.dumps(request, sort_keys=True, separators=(',', ':')))
            observed_usage |= present is not None
            row['_usage'] = present
        groups[arm] = captured
    if len(payloads) != 1:
        raise ValueError('forwarded payloads differ; inspect automatic stream_options.include_usage')
    if payload_sha256 is not None and hashlib.sha256(next(iter(payloads)).encode()).hexdigest() != payload_sha256:
        raise ValueError('forwarded payload differs from pinned policy')
    run = loads(read(path / 'xk6' / 'run.json'))
    records = run['records']
    if len(records) != 4 or len({r['request_id'] for r in records}) != 4:
        raise ValueError('xk6 record population')
    for key in ('attempts', 'successes'):
        if type(run[key]) is not int or run[key] != 4:
            raise ValueError('xk6 accounting: ' + key)
    for key in ('failures', 'dropped_records', 'excluded_boundary_requests', 'inflight_at_end', 'estimated_input_counts', 'estimated_output_counts'):
        if type(run[key]) is not int or run[key] != 0:
            raise ValueError('xk6 accounting: ' + key)
    measured = groups['xk6'][2:]
    ordered = sorted(records, key=lambda r: r['dispatched_at'])
    for direction, wire_key in (('input', 'prompt_tokens'), ('output', 'completion_tokens')):
        values = [(row['_usage'] or {}).get(wire_key) for row in measured]
        missing = sum(value is None for value in values)
        if run['missing_' + direction + '_counts'] != missing:
            raise ValueError('xk6 direction missing accounting')
        rate = run[direction + '_throughput']
        if missing and rate is not None:
            raise ValueError('missing count became available throughput')
        if not missing and (type(rate) not in (int, float) or not math.isfinite(rate) or rate < 0):
            raise ValueError('reported count throughput unavailable or invalid')
        for record, value in zip(ordered, values):
            expected = dict(value=value, source='unavailable' if value is None else 'reported')
            if record[direction + '_tokens'] != expected:
                raise ValueError('count lost source/presence or disagrees with wire')
    for record, observed in zip(ordered, measured):
        if record['phase'] != 'measurement' or record['outcome'] != 'success' or record['stream'] is not True or record['protocol'] != 'openai-chat':
            raise ValueError('xk6 phase/outcome/protocol')
        if abs(record['dispatched_at'] - observed['start_wall_ns'] / 1e6) > 15:
            raise ValueError('xk6 record-to-wire start residual')
    exports = [loads(line) for line in read(path / 'aiperf' / 'profile_export.jsonl').splitlines()]
    if len(exports) != 6 or len({r['metadata']['x_request_id'] for r in exports}) != 6:
        raise ValueError('AIPerf record population')
    phases = [r['metadata']['benchmark_phase'] for r in sorted(exports, key=lambda r: r['metadata']['request_start_ns'])]
    if phases != ['warmup'] * 2 + ['profiling'] * 4:
        raise ValueError('AIPerf phase population')
    for record, observed in zip(sorted(exports, key=lambda r: r['metadata']['request_start_ns']), groups['aiperf']):
        if record.get('error') or record['metadata'].get('was_cancelled'):
            raise ValueError('AIPerf error/cancelled request')
        if abs(record['metadata']['request_start_ns'] - observed['start_wall_ns']) > 15_000_000:
            raise ValueError('AIPerf record-to-wire start residual')
        for key, metric in record['metrics'].items():
            if key.startswith('usage_'):
                expected = (observed['_usage'] or {}).get(key[6:])
                if expected is None or metric.get('value') != expected:
                    raise ValueError('AIPerf reported usage contradicts wire')
    native = loads(read(path / 'aiperf' / 'profile_export_aiperf.json'))
    if native['request_count']['avg'] != 4 or native['error_summary']:
        raise ValueError('AIPerf aggregate accounting')
    return dict(status='SOFTWARE_PASS', missingusage='UNAVAILABLE' if observed_usage else 'OBSERVED', scope='retained own-wire missing usage and native request accounting only', statistical_status='NOT statistically gated', remaining=['live provenance admission', 'independent-direction partial usage cells', 'run replay/slices/exports', 'cost availability', 'estimated AIPerf tokenizer counts are not provider counts'])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('archive', type=pathlib.Path)
    parser.add_argument('--plan', type=pathlib.Path, required=True)
    parser.add_argument('--plan-sha256', required=True)
    args = parser.parse_args()
    try:
        raw = read(args.plan)
        if len(raw) > 16384 or hashlib.sha256(raw).hexdigest() != args.plan_sha256:
            raise ValueError('verification policy size or SHA256 mismatch')
        plan = loads(raw)
        for key in ('source_revision', 'binary_sha256', 'aiperf_revision', 'model_digest', 'runtime_identity', 'payload_sha256', 'preregistration_sha256'):
            if not isinstance(plan.get(key), str) or not plan[key]:
                raise ValueError('missing policy identity: '+key)
        if hashlib.sha256(read(args.archive / 'PREREGISTRATION.md')).hexdigest() != plan['preregistration_sha256']:
            raise ValueError('preregistration identity mismatch')
        result = verify(args.archive, plan['payload_sha256'])
    except (ValueError, KeyError, TypeError, OSError) as error:
        result = dict(status='GAP', error=str(error))
    print(json.dumps(result, indent=2))
    return 0 if result['status'] == 'SOFTWARE_PASS' else 1


if __name__ == '__main__':
    raise SystemExit(main())
