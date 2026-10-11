#!/usr/bin/env python3
"""Check matched live chat measurements against their own retained wire capture.

This qualifies a bounded streaming measurement cell, not cross-tool performance.
Run each tool in ABBA order at concurrency 1 and 2, with two excluded sequential
warmups and six measured requests. Keep all failed trials; do not retry a gate.
"""
import argparse
import collections
import hashlib
import json
import math
import pathlib
import statistics

METRICS = dict(ttft='time_to_first_token', ttfo='time_to_first_output_token',
               last_content_latency='request_latency', itl='inter_token_latency')


def loads(value):
    def pairs(items):
        result = {}
        for key, item in items:
            if key in result:
                raise ValueError('duplicate JSON key: '+key)
            result[key] = item
        return result
    def constant(value):
        raise ValueError('nonfinite JSON value: '+value)
    result = json.loads(value, object_pairs_hook=pairs, parse_constant=constant)
    def finite(item):
        if isinstance(item, float) and not math.isfinite(item):
            raise ValueError('nonfinite JSON number')
        if isinstance(item, dict):
            for child in item.values():
                finite(child)
        if isinstance(item, list):
            for child in item:
                finite(child)
    finite(result)
    return result


def parse_wire(row):
    first = output = last = None
    usage = None
    reasoning = False
    terminal = False
    if type(row.get('start_ns')) is not int or type(row.get('end_ns')) is not int or row['start_ns'] > row['end_ns']:
        raise ValueError('wire request bounds invalid')
    previous_ns = row['start_ns']
    if not isinstance(row.get('events'), list):
        raise ValueError('wire events invalid')
    for event in row['events']:
        if not isinstance(event, dict) or not isinstance(event.get('line'), str) or type(event.get('ns')) is not int:
            raise ValueError('wire event invalid')
        if not previous_ns <= event['ns'] <= row['end_ns']:
            raise ValueError('wire event timestamp outside bounds or reversing')
        previous_ns = event['ns']
        line = event['line'].strip()
        if not line.startswith('data: '):
            continue
        if terminal:
            raise ValueError('wire data after terminal')
        if line == 'data: [DONE]':
            terminal = True
            continue
        data = loads(line[6:])
        if not isinstance(data, dict) or 'error' in data:
            raise ValueError('wire terminal error or invalid object')
        if not isinstance(data.get('choices', []), list):
            raise ValueError('wire choices invalid')
        if data.get('usage') is not None:
            usage = data['usage']
        if len(data.get('choices', [])) > 1:
            raise ValueError('multichoice wire unavailable for this declared cell')
        for choice in data.get('choices', []):
            if not isinstance(choice, dict):
                raise ValueError('wire choice invalid')
            if type(choice.get('index', 0)) is not int or choice.get('index', 0) != 0:
                raise ValueError('nonprimary choice unavailable for this declared cell')
            delta = choice.get('delta', {})
            if not isinstance(delta, dict):
                raise ValueError('wire delta invalid')
            for key in ('content', 'reasoning_content', 'reasoning'):
                if delta.get(key) is not None and not isinstance(delta[key], str):
                    raise ValueError('wire delta text invalid: '+key)
            tools = delta.get('tool_calls')
            if tools is not None:
                if not isinstance(tools, list):
                    raise ValueError('wire tool calls invalid container')
                for tool in tools:
                    if not isinstance(tool, dict) or type(tool.get('index')) is not int or tool['index'] < 0:
                        raise ValueError('wire tool call invalid index')
                    for key in ('id', 'type'):
                        if tool.get(key) is not None and not isinstance(tool[key], str):
                            raise ValueError('wire tool call text invalid: '+key)
                    function = tool.get('function')
                    if function is not None:
                        if not isinstance(function, dict):
                            raise ValueError('wire tool function invalid object')
                        for key in ('name', 'arguments'):
                            if function.get(key) is not None and not isinstance(function[key], str):
                                raise ValueError('wire tool function text invalid: '+key)
            visible = bool(delta.get('content') or tools)
            thought = bool(delta.get('reasoning_content') or delta.get('reasoning'))
            if visible or thought:
                first = event['ns'] if first is None else first
                last = event['ns']
            if visible and output is None:
                output = event['ns']
            reasoning = reasoning or thought
    if not terminal:
        raise ValueError('wire terminal missing')
    if usage is None:
        raise ValueError('wire usage missing')
    if not isinstance(usage, dict):
        raise ValueError('wire usage invalid object')
    for key in ('prompt_tokens', 'completion_tokens', 'total_tokens'):
        if type(usage.get(key)) is not int or usage[key] < 0:
            raise ValueError('wire usage invalid')
    if usage['prompt_tokens'] + usage['completion_tokens'] != usage['total_tokens']:
        raise ValueError('wire usage inconsistent')
    if first is None or output is None or usage['completion_tokens'] <= 1:
        raise ValueError('wire content or ITL unavailable for this declared cell')
    if not row['start_ns'] <= first <= output <= last <= row['end_ns']:
        raise ValueError('wire timing order invalid')
    return dict(ttft=(first-row['start_ns'])/1e6,
                ttfo=(output-row['start_ns'])/1e6,
                last_content_latency=(last-row['start_ns'])/1e6,
                itl=(last-first)/1e6/(usage['completion_tokens']-1),
                usage=usage, reasoning=reasoning)



def read_plan(path, digest):
    """Read an explicitly pinned, bounded campaign policy; never infer from rows."""
    raw = path.read_bytes()
    if len(raw) > 16384 or hashlib.sha256(raw).hexdigest() != digest:
        raise ValueError('plan size or SHA256 mismatch')
    plan = loads(raw)
    fields = {'schema_version', 'concurrencies', 'measured_requests',
              'warmup_requests', 'payload_sha256', 'latency_limit_ms', 'itl_limit_ms',
              'preregistration_sha256'}
    if not isinstance(plan, dict) or set(plan) != fields or type(plan['schema_version']) is not int or plan['schema_version'] != 1:
        raise ValueError('invalid campaign plan schema')
    conc = plan['concurrencies']
    n = plan['measured_requests']
    if not isinstance(conc, list) or not 1 <= len(conc) <= 4 or any(type(c) is not int or not 1 <= c <= 8 for c in conc) or conc != sorted(set(conc)):
        raise ValueError('invalid campaign concurrencies')
    if type(n) is not int or n < max(conc) or len(conc)*4*n > 64:
        raise ValueError('invalid campaign measured population')
    if type(plan['warmup_requests']) is not int or plan['warmup_requests'] != 2:
        raise ValueError('campaign requires two excluded warmups')
    if type(plan['latency_limit_ms']) not in (int, float) or plan['latency_limit_ms'] != 15 or type(plan['itl_limit_ms']) not in (int, float) or plan['itl_limit_ms'] != .25:
        raise ValueError('campaign residual tolerances cannot change')
    for field in ('payload_sha256', 'preregistration_sha256'):
        payload = plan[field]
        if not isinstance(payload, str) or len(payload) != 64 or any(c not in '0123456789abcdef' for c in payload):
            raise ValueError('invalid campaign SHA256: '+field)
    return plan


def verify(path, plan=None):
    concurrencies = (1, 2) if plan is None else plan['concurrencies']
    n = 6 if plan is None else plan['measured_requests']
    arms = tuple(f'c{c}-{a}' for c in concurrencies for a in ('a1', 'b1', 'b2', 'a2'))
    checks = []
    def check(name, passed, **details):
        checks.append(dict(name=name, passed=bool(passed), **details))
    if (path/'wire.jsonl').stat().st_size > 64*1024*1024:
        raise ValueError('wire archive exceeds 64 MiB limit')
    rows = [loads(line) for line in (path/'wire.jsonl').read_text().splitlines()]
    grouped = collections.defaultdict(list)
    payloads = set()
    for row in rows:
        grouped[row['arm']].append(row)
        payloads.add(hashlib.sha256(json.dumps(row['request'], sort_keys=True,
            separators=(',', ':')).encode()).hexdigest())
    check('exact arm identities', set(grouped) == set(arms))
    check('one canonical payload', len(payloads) == 1)
    if plan is not None:
        check('declared payload identity', payloads == {plan['payload_sha256']})
        check('retained preregistration identity', hashlib.sha256((path/'PREREGISTRATION.md').read_bytes()).hexdigest() == plan['preregistration_sha256'])
    check('exact wire population', len(rows) == len(arms)*(n+2))
    observed_order = tuple(dict.fromkeys(row['arm'] for row in sorted(rows, key=lambda row: row['start_ns'])))
    check('observed ABBA order', observed_order == arms)
    for previous, current in zip(arms, arms[1:]):
        if grouped[previous] and grouped[current]:
            check('independent sequential arms', max(row['end_ns'] for row in grouped[previous]) <= min(row['start_ns'] for row in grouped[current]), arm=current)
    for arm in arms:
        captured = sorted(grouped[arm], key=lambda row: row['start_ns'])
        check('warmup and measurement population', len(captured) == n+2, arm=arm)
        if len(captured) != n+2:
            continue
        measured = captured[2:]
        check('excluded sequential warmups', captured[0]['end_ns'] <= captured[1]['start_ns'] and captured[1]['end_ns'] <= measured[0]['start_ns'], arm=arm)
        events = sorted([(row['start_ns'], 1) for row in measured] + [(row['end_ns'], -1) for row in measured])
        active = peak = 0
        for _, change in events:
            active += change
            peak = max(peak, active)
        check('observed concurrency', peak == int(arm.split('-')[0][1:]), arm=arm, observed=peak)
        check('all statuses successful', all(row['status'] == 200 for row in captured), arm=arm)
        parsed = [parse_wire(row) for row in captured][2:]
        check('real reasoning trigger', all(row['reasoning'] for row in parsed), arm=arm)
        inputs = sum(row['usage']['prompt_tokens'] for row in parsed)
        outputs = sum(row['usage']['completion_tokens'] for row in parsed)
        if '-b' in arm:
            run = loads((path/arm/'run.json').read_text())
            records = run['records']
            check('xk6 run and measurement identities', run['run_id'] == arm and len({row['request_id'] for row in records}) == n and all(row['phase'] == 'measurement' and row['protocol'] == 'openai-chat' and row['stream'] is True for row in records), arm=arm)
            native_starts = sorted(row['dispatched_at'] for row in records)
            check('xk6 record-to-wire starts', len(native_starts) == n and all(abs(start-row['start_wall_ns']/1e6) <= 15 for start, row in zip(native_starts, measured)), arm=arm)
            check('loss and count accounting', len(records) == n and
                  run['attempts'] == run['successes'] == n and
                  run['failures'] == run['dropped_records'] ==
                  run['excluded_boundary_requests'] == 0 and
                  run['input_tokens'] == inputs and run['output_tokens'] == outputs, arm=arm)
            native = {key: statistics.mean(row[key] for row in records) for key in METRICS}
        else:
            run = loads((path/arm/'profile_export_aiperf.json').read_text())
            exports = [loads(line) for line in (path/arm/'profile_export.jsonl').read_text().splitlines()]
            warm = [row for row in exports if row['metadata']['benchmark_phase'] == 'warmup']
            profile = [row for row in exports if row['metadata']['benchmark_phase'] == 'profiling']
            check('AIPerf phase and request identities', len(exports) == n+2 and len(warm) == 2 and len(profile) == n and len({row['metadata']['x_request_id'] for row in exports}) == n+2, arm=arm)
            starts = sorted(row['metadata']['request_start_ns']/1e6 for row in profile)
            check('AIPerf record-to-wire starts', len(starts) == n and all(abs(start-row['start_wall_ns']/1e6) <= 15 for start, row in zip(starts, measured)), arm=arm)
            check('AIPerf outcome and count accounting',
                  run['request_count']['avg'] == n and
                  run['input_sequence_length']['sum'] == inputs and
                  run['output_token_count']['sum'] == outputs and
                  not run['error_summary'], arm=arm)
            native = {key: run[value]['avg'] for key, value in METRICS.items()}
        for key in METRICS:
            residual = abs(native[key] - statistics.mean(row[key] for row in parsed))
            limit = .25 if key == 'itl' else 15
            check('own-wire mean residual', residual <= limit,
                  arm=arm, metric=key, residual_ms=residual, limit_ms=limit)
    return dict(status='PASS' if checks and all(row['passed'] for row in checks) else 'GAP',
                scope='streaming chat measurement semantics in this retained live cell',
                statistical_status='NOT statistically gated; no performance parity claim',
                payload_hashes=sorted(payloads), checks=checks)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('archive', type=pathlib.Path)
    parser.add_argument('--plan', type=pathlib.Path, help='preregistered bounded JSON policy')
    parser.add_argument('--plan-sha256', help='exact SHA256 of preregistered policy bytes')
    args = parser.parse_args()
    try:
        if bool(args.plan) != bool(args.plan_sha256):
            raise ValueError('plan and plan SHA256 must be supplied together')
        plan = read_plan(args.plan, args.plan_sha256) if args.plan else None
        result = verify(args.archive, plan)
        result['plan_sha256'] = args.plan_sha256
        result['policy'] = plan if plan is not None else dict(concurrencies=[1, 2], measured_requests=6, warmup_requests=2, latency_limit_ms=15, itl_limit_ms=.25)
    except (KeyError, ValueError, OSError, TypeError, AttributeError, RecursionError, statistics.StatisticsError) as error:
        result = dict(status='GAP', error=str(error),
                      scope='malformed or unavailable retained evidence')
    print(json.dumps(result, indent=2, allow_nan=False))
    return 0 if result['status'] == 'PASS' else 1


if __name__ == '__main__':
    raise SystemExit(main())
