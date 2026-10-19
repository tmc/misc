#!/usr/bin/env python3
"""Realize bounded count/cache trace snapshots with pinned offline AIPerf code."""
import argparse
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path
import re
import resource
import shutil
import signal
import stat
import subprocess
import sys
import time

REFERENCE = 'f10eb50a1776775d80d7f13396b0f668f14e2c03'
SOURCE_TREE = 'dcf22c44e8b544fa7f66411f4d9b38c0998253155b9d2453af908cffba620521'
CORPUS = 'e0e13a826912a4a81bb3a582aa73c4af0675bdeee6ddf6d505efb63d562d496f'
PINS = {'aiperf': '0.13.0', 'transformers': '5.19.0', 'tokenizers': '0.23.3',
        'huggingface-hub': '1.33.0', 'numpy': '2.4.6', 'pydantic': '2.14.0'}
MAX_BYTES = 8 << 20


def encoded(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=False, allow_nan=False).encode()


def digest(data):
    return hashlib.sha256(data).hexdigest()


def read(path, limit=MAX_BYTES):
    p = Path(path)
    if p.is_symlink() or not p.is_file():
        raise ValueError('input must be a regular nonsymlink file')
    with p.open('rb') as f:
        data = f.read(limit + 1)
    if len(data) > limit:
        raise ValueError('input exceeds byte budget')
    return data


def decode(data):
    def pairs(items):
        result = {}
        for k, v in items:
            if k in result:
                raise ValueError('duplicate JSON member')
            result[k] = v
        return result
    def constant(_):
        raise ValueError('nonfinite JSON number')
    return json.loads(data.decode('utf-8'), object_pairs_hook=pairs, parse_constant=constant)


def write(path, value):
    data = encoded(value) + b'\n'
    if len(data) > MAX_BYTES:
        raise ValueError('output exceeds byte budget')
    with Path(path).open('xb') as f:
        os.chmod(path, 0o600)
        f.write(data)
    return digest(data)


def pin(value):
    return isinstance(value, str) and re.fullmatch('[0-9a-f]{64}', value) is not None


def integer(value, low, high):
    return type(value) is int and low <= value <= high


def token_ids(value):
    return isinstance(value, list) and all(integer(x, 0, (1 << 31) - 1) for x in value)


def text_digest(text):
    try:
        return digest(text.encode('utf-8'))
    except UnicodeEncodeError:
        raise ValueError('reference text is not valid UTF-8') from None


# The worker may report only reference-stage statuses. VERIFIED is assigned by
# the parent alone, after its own Go tokenizer run for that row.
WORKER_STATUSES = ('HF_VERIFIED', 'UNAVAILABLE')
WORKER_EVIDENCE = {'text', 'text_sha256', 'intended_token_ids', 'hf_token_ids', 'hf_count'}
WORKER_DIAGNOSTICS = {'stable_terminator_ids', 'block_separator_id', 'reason',
                      'reference_error_type', 'reference_error_message'}


def check_worker(result, row):
    """Check one worker result against its admitted row; raise on any protocol violation."""
    if set(result) - set(row) - {'status'} - WORKER_EVIDENCE - WORKER_DIAGNOSTICS:
        raise ValueError('reference returned fields outside the worker protocol')
    status = result.get('status')
    if type(status) is not str or status not in WORKER_STATUSES:
        raise ValueError('reference returned a status outside the worker protocol')
    evidence = WORKER_EVIDENCE & set(result)
    if evidence and evidence != WORKER_EVIDENCE:
        raise ValueError('reference returned partial evidence')
    if evidence:
        if type(result['text']) is not str or result['text_sha256'] != text_digest(result['text']):
            raise ValueError('reference text digest differs')
        if not token_ids(result['intended_token_ids']) or not token_ids(result['hf_token_ids']):
            raise ValueError('reference token IDs are not exact integers')
        if type(result['hf_count']) is not int or result['hf_count'] != len(result['hf_token_ids']):
            raise ValueError('reference count differs from its token IDs')
    if 'stable_terminator_ids' in result and not token_ids(result['stable_terminator_ids']):
        raise ValueError('reference terminator IDs are not exact integers')
    if 'block_separator_id' in result and result['block_separator_id'] is not None and not token_ids([result['block_separator_id']]):
        raise ValueError('reference block separator is not an exact integer')
    if any(k in result and type(result[k]) is not str for k in ('reason', 'reference_error_type', 'reference_error_message')):
        raise ValueError('reference diagnostics are not strings')
    if status == 'UNAVAILABLE':
        if 'reason' not in result:
            raise ValueError('unavailable reference result has no reason')
        return
    if not evidence or any(k in result for k in ('reason', 'reference_error_type', 'reference_error_message')):
        raise ValueError('verified reference result is incomplete or carries an error')
    if len(result['hf_token_ids']) != row['target']:
        raise ValueError('verified reference count differs from target')
    if row['hash_ids'] and result['intended_token_ids'] != result['hf_token_ids']:
        raise ValueError('verified reference cached IDs differ from intended IDs')


def validate(config, trace):
    fields = {'schema_version', 'trace', 'trace_sha256', 'source_sha256', 'assets',
              'asset_manifest', 'asset_manifest_sha256', 'model', 'model_revision',
              'go_tokenizer', 'go_tokenizer_sha256', 'python_sha256', 'seed'}
    if not isinstance(config, dict) or set(config) != fields or type(config['schema_version']) is not int or config['schema_version'] != 1:
        raise ValueError('invalid configuration fields')
    for key in ('trace_sha256', 'source_sha256', 'asset_manifest_sha256', 'go_tokenizer_sha256', 'python_sha256'):
        if not pin(config[key]):
            raise ValueError('invalid SHA256 pin')
    if not integer(config['seed'], 0, (1 << 63) - 1):
        raise ValueError('invalid seed')
    for key in ('model', 'model_revision'):
        if not isinstance(config[key], str) or not 1 <= len(config[key]) <= 256:
            raise ValueError('missing model identity label')
    if not isinstance(trace, dict) or trace.get('sourceHash') != config['source_sha256'] or trace.get('format') not in ('mooncake-jsonl-v1', 'bailian-jsonl-v1', 'tracelab-jsonl-v1'):
        raise ValueError('wrong normalized source identity or format')
    if set(trace) - {'format', 'sourceHash', 'timestampPolicy', 'countPolicy', 'hashPolicy', 'sessions'}:
        raise ValueError('unknown normalized trace field')
    sessions = trace.get('sessions')
    if not isinstance(sessions, list) or not 1 <= len(sessions) <= 16:
        raise ValueError('invalid session population')
    rows, identifiers, total = [], set(), 0
    scope_blocks = {}
    for session in sessions:
        if not isinstance(session, dict) or set(session) - {'session_id', 'contextMode', 'blockSize', 'hashScope', 'originUnixMS', 'turns'}:
            raise ValueError('unknown session field')
        sid = session.get('session_id')
        if not isinstance(sid, str) or not 1 <= len(sid) <= 256 or sid in identifiers:
            raise ValueError('invalid or duplicate session identity')
        identifiers.add(sid)
        if session.get('hashScope') not in ('file', 'session-virtual') or not integer(session.get('blockSize'), 1, 512) or session.get('contextMode') not in ('recorded', 'live'):
            raise ValueError('unsupported context or prefix scope')
        scope = trace['sourceHash'] if session['hashScope'] == 'file' else digest(encoded([trace['sourceHash'], sid]))
        if scope in scope_blocks and scope_blocks[scope] != session['blockSize']:
            raise ValueError('one scope cannot use different block sizes')
        scope_blocks[scope] = session['blockSize']
        turns = session.get('turns')
        if not isinstance(turns, list) or not turns:
            raise ValueError('empty turn population')
        turn_ids = set()
        for turn in turns:
            if not isinstance(turn, dict) or set(turn) - {'stop','inputType','id','sourceRow','timestamp','delay','inputTokens','outputTokens','inputTarget','outputBudget','prefixTokens','newlyAppendTokens','reasoningTokens','hashIDs','roundIndex','model','requestType','parentID','apiTime','thinkTime','inputMode','textInput','messages','payload','tools','extra','raw'}:
                raise ValueError('unknown turn field')
            if turn.get('inputType') not in (None, '', 'text'):
                raise ValueError('modality input realization is unsupported')
            tid = turn.get('id')
            if not isinstance(tid, str) or not 1 <= len(tid) <= 256 or tid in turn_ids:
                raise ValueError('invalid or duplicate turn identity')
            turn_ids.add(tid)
            if turn.get('inputMode') != 'counts' or any(turn.get(k) is not None for k in ('messages', 'payload', 'textInput', 'tools', 'extra')):
                raise ValueError('only independent complete count snapshots are supported')
            target = turn.get('inputTarget')
            if target is None:
                target = turn.get('inputTokens')
            if not integer(target, 0, 4096) or not integer(turn.get('outputBudget'), 1, 32):
                raise ValueError('input target or output budget outside bounds')
            if turn.get('inputTokens') is not None and not integer(turn['inputTokens'], 0, 9007199254740991):
                raise ValueError('invalid original input token declaration')
            if turn.get('sourceRow') is not None and not integer(turn['sourceRow'], 0, 1000000):
                raise ValueError('invalid source row')
            hashes = turn.get('hashIDs')
            if not isinstance(hashes, list) or len(hashes) > 64:
                raise ValueError('invalid cache block population')
            if any(not isinstance(h, str) or not re.fullmatch('-?(0|[1-9][0-9]*)', h) or not -(1 << 63) <= int(h) < 1 << 63 for h in hashes):
                raise ValueError('hash IDs must be exact signed integer strings')
            total += target
            rows.append({'key': str(len(rows)), 'session_id': sid, 'turn_id': tid, 'scope': scope,
                         'scope_kind': session['hashScope'], 'block_size': session['blockSize'],
                         'target': target, 'hash_ids': hashes, 'output_budget': turn['outputBudget'],
                         'source': {'id': tid, 'hashIDs': hashes, 'inputTokens': turn.get('inputTokens'),
                                    'inputTarget': turn.get('inputTarget'), 'sourceRow': turn.get('sourceRow'),
                                    'contextMode': session['contextMode']}})
            if len(rows) > 16 or total > 16384:
                raise ValueError('aggregate realization budget exceeded')
    return rows


def runtime_identity(config):
    if digest(read(Path(sys.executable).resolve(), 64 << 20)) != config['python_sha256']:
        raise ValueError('Python executable hash differs')
    versions = {k: importlib.metadata.version(k) for k in PINS}
    if versions != PINS:
        raise ValueError('runtime package versions differ from pins')
    root = Path(importlib.metadata.distribution('aiperf').locate_file('aiperf'))
    entries = [(str(p.relative_to(root)), digest(read(p))) for p in sorted(root.rglob('*.py'))]
    source_tree = digest(json.dumps(entries, separators=(',', ':')).encode())
    if len(entries) != 1025 or source_tree != SOURCE_TREE:
        raise ValueError('AIPerf Python source tree differs from reference')
    corpus = read(root / 'dataset/generator/assets/shakespeare.txt')
    if digest(corpus) != CORPUS:
        raise ValueError('prompt corpus differs from reference')
    return {'reference_revision': REFERENCE, 'source_tree_sha256': source_tree,
            'versions': versions, 'python_sha256': config['python_sha256'], 'corpus_sha256': CORPUS}


def copy_assets(config, output):
    raw = read(config['asset_manifest'])
    if digest(raw) != config['asset_manifest_sha256']:
        raise ValueError('asset manifest hash differs')
    manifest = decode(raw)
    with (output / 'asset-manifest.json').open('xb') as stream:
        os.chmod(output / 'asset-manifest.json', 0o600)
        stream.write(raw)
    names = {'tokenizer.json', 'tokenizer_config.json', 'merges.txt', 'vocab.json'}
    root = Path(config['assets'])
    if root.is_symlink() or set(p.name for p in root.iterdir()) != names or not isinstance(manifest, dict) or set(manifest) != names:
        raise ValueError('asset directory or manifest fields differ')
    repo = 'local/aiperf-' + config['asset_manifest_sha256'][:16]
    revision = config['asset_manifest_sha256'][:40]
    snapshot = output / 'hf-cache' / ('models--' + repo.replace('/', '--')) / 'snapshots' / revision
    snapshot.mkdir(parents=True)
    total = 0
    for name in sorted(names):
        data = read(root / name, 64 << 20)
        total += len(data)
        if total > 64 << 20 or not pin(manifest[name]) or digest(data) != manifest[name]:
            raise ValueError('asset byte identity differs')
        target = snapshot / name
        target.write_bytes(data)
        target.chmod(0o444)
    return {'repo': repo, 'revision': revision, 'cache': str(output / 'hf-cache'),
            'snapshot': str(snapshot), 'manifest_sha256': digest(raw), 'files': manifest,
            'revision_basis': 'local content-derived cache alias; upstream label is a declaration'}


def worker(job, progress=None):
    # The parent has pinned code/assets and set offline environment/CPU affinity.
    if os.environ.get('HF_HUB_OFFLINE') != '1' or os.environ.get('TRANSFORMERS_OFFLINE') != '1':
        raise ValueError('worker requires parent offline environment')
    trace_bytes = read(job['config']['trace'])
    if digest(trace_bytes) != job['config']['trace_sha256'] or validate(job['config'], decode(trace_bytes)) != job['rows']:
        raise ValueError('worker population differs from pinned input')
    os.sched_setaffinity(0, sorted(os.sched_getaffinity(0))[:2])
    signal.alarm(30)
    runtime_identity(job['config'])
    if digest(read(Path(__file__).resolve())) != job['adapter_sha256']:
        raise ValueError('adapter source changed before reference dispatch')
    manifest = decode(read(job['config']['asset_manifest']))
    if not isinstance(manifest, dict) or set(manifest) != {'tokenizer.json', 'tokenizer_config.json', 'merges.txt', 'vocab.json'}:
        raise ValueError('worker asset fields differ')
    repo = 'local/aiperf-' + job['config']['asset_manifest_sha256'][:16]
    revision = job['config']['asset_manifest_sha256'][:40]
    snapshot = Path(job['assets']['cache']) / ('models--' + repo.replace('/', '--')) / 'snapshots' / revision
    if job['assets']['repo'] != repo or job['assets']['revision'] != revision or Path(job['assets']['snapshot']) != snapshot or os.environ.get('HF_HUB_CACHE') != job['assets']['cache']:
        raise ValueError('worker private cache identity differs')
    if digest(read(job['config']['asset_manifest'])) != job['config']['asset_manifest_sha256'] or manifest != job['assets']['files']:
        raise ValueError('worker asset manifest differs')
    if any(digest(read(Path(job['assets']['snapshot']) / name, 64 << 20)) != want for name, want in manifest.items()):
        raise ValueError('worker asset snapshot differs')
    from aiperf.common import random_generator as rng
    from aiperf.common.tokenizer import Tokenizer
    from aiperf.dataset.generator.prompt import PromptGenerator
    from aiperf.dataset.loader.hash_ids_synthesis import HashIdsPromptRequest, HashIdsPromptSynthesisMixin

    class ObservedGenerator(PromptGenerator):
        # Transparent capture: execute the actual method exactly once and retain
        # its unchanged result. There is no second RNG-consuming sampling call.
        def _build_token_sequence(self, *args, **kwargs):
            value = super()._build_token_sequence(*args, **kwargs)
            self.intended = list(value)
            return value

        def _decode_to_exact_len(self, tokens, *args, **kwargs):
            self.intended = list(tokens)
            return super()._decode_to_exact_len(tokens, *args, **kwargs)

    rng.reset()
    rng.init(job['config']['seed'])
    tokenizer = Tokenizer.from_pretrained(job['assets']['repo'], revision=job['assets']['revision'],
                                          trust_remote_code=False, resolve_alias=False)
    generators, results = {}, []
    for row in job['rows']:
        result = dict(row, status='UNAVAILABLE')
        try:
            scope = row['scope']
            if scope not in generators:
                pg = ObservedGenerator(prompts=None, prefix_prompts=None, tokenizer=tokenizer)
                pg._hash_id_corpus_rng.set_trace_id(scope)
                pg._cache.clear()
                generators[scope] = pg
            pg = generators[scope]
            pg.intended = []
            result.update(stable_terminator_ids=list(pg._bpe_stable_terminator_tokens), block_separator_id=tokenizer.block_separation_token_id)
            host = HashIdsPromptSynthesisMixin()
            host.prompt_generator = pg
            host._tokenizer_name = job['assets']['repo']
            host._trust_remote_code = False
            host._tokenizer_revision = job['assets']['revision']
            host._block_size = row['block_size']
            # Singleton batches preserve source ordering/shared cache while keeping
            # the unchanged parallel_decode below its 10-item pool threshold.
            text = host.synthesize_prompts_from_hash_ids([HashIdsPromptRequest(row['key'], [int(h) for h in row['hash_ids']], row['target'])])[row['key']]
            ids = tokenizer.encode(text, add_special_tokens=False)
            result.update(text=text, text_sha256=digest(text.encode()), intended_token_ids=pg.intended,
                          hf_token_ids=ids, hf_count=len(ids), stable_terminator_ids=list(pg._bpe_stable_terminator_tokens),
                          block_separator_id=tokenizer.block_separation_token_id)
            if len(ids) != row['target']:
                result['reason'] = 'reference decoded text does not reencode to declared target'
            elif row['hash_ids'] and ids != pg.intended:
                result['reason'] = 'reference cached token IDs changed through decode/reencode'
            else:
                result['status'] = 'HF_VERIFIED'
        except Exception as error:
            result.update(reason='reference algorithm refused: ' + type(error).__name__, reference_error_type=type(error).__name__, reference_error_message=str(error)[:4096])
        results.append(result)
        if progress is not None:
            with progress.open('ab') as stream:
                os.chmod(progress, 0o600)
                stream.write(encoded(result) + b'\n')
    return results


def outputs(output):
    """Return the digest of every output file outside the private cache and TMPDIR."""
    return {p: None if p.is_dir() and not p.is_symlink() else digest(read(p, 64 << 20))
            for p in output.rglob('*') if p.relative_to(output).parts[0] not in ('tmp', 'hf-cache')}


def unlock(path):
    """Clear user flags and add owner rw (rwx for a directory) so path can be moved or removed.

    A symlink or a file with other hard links is left unchanged: its inode may
    be shared with a file outside the output directory.
    """
    try:
        info = os.lstat(path)
        if stat.S_ISLNK(info.st_mode) or (not stat.S_ISDIR(info.st_mode) and info.st_nlink > 1):
            return
        if hasattr(os, 'chflags'):
            os.chflags(path, 0)
        os.chmod(path, info.st_mode | (stat.S_IRWXU if stat.S_ISDIR(info.st_mode) else stat.S_IRUSR | stat.S_IWUSR))
    except (OSError, NotImplementedError):
        pass


def quarantine(output):
    """Move sessions.json and status.json off their canonical names; report whether both are clear.

    Only the parent's final step may leave either file, including ones a worker
    wrote into the shared output directory. It runs after the worker's process
    group is gone, so it first restores the permissions and flags a worker may
    have changed. It never raises: a blocked rename target falls back to a
    numbered name, then to removal.
    """
    unlock(output)
    for name in ('sessions.json', 'status.json'):
        path = output / name
        if os.path.lexists(path):
            unlock(path)
        for n in range(16):
            target = output / ('unqualified-' + name + ('.%d' % n if n else ''))
            if not os.path.lexists(path):
                break
            if os.path.lexists(target):
                continue
            try:
                path.rename(target)
            except OSError:
                pass
        if os.path.lexists(path):
            try:
                if path.is_dir() and not path.is_symlink():
                    for top, directories, files in os.walk(path):
                        for entry in directories + files:
                            unlock(os.path.join(top, entry))
                    shutil.rmtree(path)
                else:
                    path.unlink()
            except OSError:
                pass
    return not any(os.path.lexists(output / name) for name in ('sessions.json', 'status.json'))


def run_child(argv, output, name, environment, seconds):
    if seconds <= 0:
        raise ValueError('aggregate deadline exceeded before dispatch')
    limit = min(30, seconds)
    command = write(output / (name + '.command.json'), {'argv': argv, 'timeout_seconds': limit})
    code = 124
    with (output / (name + '.stdout')).open('xb') as stdout, (output / (name + '.stderr')).open('xb') as stderr:
        process = subprocess.Popen(argv, env=environment, stdout=stdout, stderr=stderr, start_new_session=True,
                                   preexec_fn=lambda: resource.setrlimit(resource.RLIMIT_FSIZE, (MAX_BYTES, MAX_BYTES)))
        try:
            code = process.wait(timeout=limit)
        except subprocess.TimeoutExpired:
            code = 124
        finally:
            # Also clean up if a parent SIGTERM interrupts wait. The worker and
            # any tokenizer children own this process group; no foreign job is touched.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait(timeout=1)
            write(output / (name + '.exit.json'), {'exit_code': code})
    # The child shares the output directory; its command receipt stays parent-authored.
    if digest(read(output / (name + '.command.json'))) != command:
        raise ValueError(name + ' changed its parent-written command receipt')
    return code


def realize(config, rows, assets, output):
    deadline = time.monotonic() + 120
    affinity = sorted(os.sched_getaffinity(0))[:2]
    os.sched_setaffinity(0, affinity)
    environment = {k: v for k, v in os.environ.items() if not any(s in k.upper() for s in ('TOKEN', 'SECRET', 'PASSWORD', 'API_KEY'))}
    environment.update(HF_HUB_OFFLINE='1', TRANSFORMERS_OFFLINE='1', HF_DATASETS_OFFLINE='1',
                       HF_HUB_CACHE=assets['cache'], CUDA_VISIBLE_DEVICES='', USE_TORCH='0', USE_TF='0', USE_FLAX='0',
                       TOKENIZERS_PARALLELISM='false', OMP_NUM_THREADS='1', OPENBLAS_NUM_THREADS='1', MKL_NUM_THREADS='1',
                       GOMAXPROCS='2', TMPDIR=str(output / 'tmp'), PYTHONDONTWRITEBYTECODE='1')
    (output / 'tmp').mkdir()
    write(output / 'worker-job.json', {'config': config, 'rows': rows, 'assets': assets,
                                        'adapter_sha256': digest(read(Path(__file__).resolve()))})
    before = outputs(output)
    code = run_child([sys.executable, str(Path(__file__).resolve()), '--worker', str(output / 'worker-job.json')], output, 'reference', environment, deadline-time.monotonic())
    # The worker shares the output directory. Apart from its own receipts it
    # may not add, change or remove anything there.
    receipts = {output / n for n in ('reference.command.json', 'reference.stdout', 'reference.stderr', 'reference.exit.json', 'reference-progress.jsonl')}
    if {p: v for p, v in outputs(output).items() if p not in receipts} != before:
        raise ValueError('reference changed output files outside its protocol')
    if code:
        raise ValueError('reference subprocess failed; retained command/stdout/stderr')
    results = decode(read(output / 'reference.stdout'))
    if not isinstance(results, list) or len(results) != len(rows):
        raise ValueError('reference returned a different attempted population')
    for result, row in zip(results, rows, strict=True):
        # Bind every admitted field, including nested provenance and JSON types.
        # Worker diagnostics may add fields but cannot replace request policy.
        if not isinstance(result, dict) or any(k not in result for k in row) or encoded({k: result[k] for k in row}) != encoded(row):
            raise ValueError('reference returned a different attempted population')
        check_worker(result, row)
    blocks, prefix_checks = {}, []
    for row in results:
        if row['status'] != 'HF_VERIFIED':
            continue
        for index, hash_id in enumerate(row['hash_ids']):
            key = (row['scope'], hash_id)
            block = row['hf_token_ids'][index*row['block_size']:min((index+1)*row['block_size'], row['target'])]
            equal = key not in blocks or blocks[key] == block
            prefix_checks.append({'row': row['key'], 'scope': row['scope'], 'hash_id': hash_id,
                                  'token_ids': block, 'matches_prior': equal, 'seen_before': key in blocks})
            if not equal:
                row.update(status='UNAVAILABLE', reason='declared shared prefix IDs changed across rows')
            blocks[key] = block
    go_verified = set()
    for row in results:
        if row['status'] != 'HF_VERIFIED':
            continue
        name = 'go-' + row['key']
        write(output / (name + '.input.json'), {'text': row['text']})
        argv = [config['go_tokenizer'], '-input', str(output / (name + '.input.json')),
                '-tokenizer', str(Path(assets['snapshot']) / 'tokenizer.json'), '-tokenizer-sha256', assets['files']['tokenizer.json'],
                '-model', config['model'], '-revision', config['model_revision'], '-ids']
        code = run_child(argv, output, name, environment, deadline-time.monotonic())
        if code:
            row.update(status='UNAVAILABLE', reason='independent Go tokenizer command failed')
            continue
        go = decode(read(output / (name + '.stdout')))
        if not isinstance(go, dict):
            go = {}
        row['go_token_ids'], row['go_count'] = go.get('ids'), go.get('count')
        # Both vectors are exact integer lists here, so equality cannot pass
        # through float, bool or null values.
        if (token_ids(go.get('ids')) and type(go.get('count')) is int and go['count'] == len(go['ids']) == row['target']
                and go['ids'] == row['hf_token_ids'] and go.get('framed_sha256') == row['text_sha256']
                and go.get('tokenizer_sha256') == assets['files']['tokenizer.json'] and go.get('scope') == 'text without postprocessor'):
            row['status'] = 'VERIFIED'
            go_verified.add(row['key'])
        else:
            row.update(status='UNAVAILABLE', reason='independent Go IDs/count/text differ from actual HF result')
    if digest(read(config['go_tokenizer'], 128 << 20)) != config['go_tokenizer_sha256']:
        raise ValueError('Go executable changed during realization')
    runtime_identity(config)
    if any(digest(read(Path(assets['snapshot']) / name, 64 << 20)) != want for name, want in assets['files'].items()):
        raise ValueError('private asset snapshot changed during realization')
    write(output / 'realization.json', {'scope': 'offline text IDs; no framing, engine or cache-hit attestation',
                                       'batch_policy': 'one request per unchanged mixin call; shared generator per scope',
                                       'cpu_affinity': affinity, 'prefix_checks': prefix_checks, 'results': results})
    if any(row['status'] != 'VERIFIED' or row['key'] not in go_verified for row in results):
        return False
    sessions = {}
    for row in results:
        session = sessions.setdefault(row['session_id'], {'session_id': row['session_id'], 'source_sha256': config['source_sha256'],
            'source_policy': 'synthetic complete count snapshots; serial-recorded; timing/delay not scheduled; no generated context or cache-hit claim', 'turns': []})
        body = {'model': config['model'], 'messages': [{'role': 'user', 'content': row['text']}], 'max_tokens': row['output_budget']}
        session['turns'].append({'id': row['turn_id'], 'request': {'path': '/chat/completions', 'protocol': 'openai-chat', 'body': body},
                                'source': row['source'], 'realization': {'scope': row['scope'], 'hf_count': row['hf_count'],
                                                                      'text_sha256': row['text_sha256'], 'prefix_policy': 'token-ID equality within declared source scope'}})
    write(output / 'sessions.json', list(sessions.values()))
    return True


def main():
    if len(sys.argv) == 3 and sys.argv[1] == '--worker':
        value = worker(decode(read(sys.argv[2])), Path(sys.argv[2]).parent / 'reference-progress.jsonl')
        data = encoded(value)
        if len(data) > MAX_BYTES:
            raise ValueError('reference output exceeds budget')
        sys.stdout.buffer.write(data + b'\n')
        return 0
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', type=Path, required=True)
    parser.add_argument('--config-sha256', required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    # The first SIGTERM or SIGINT interrupts realization, which kills any owned
    # child. Later signals, and any once the final phase starts, are only
    # recorded: hashing, the status write and the quarantine always finish, and
    # a recorded signal makes the run unavailable.
    interrupts, closing = [], []
    def interrupted(signum, _frame):
        interrupts.append(signum)
        if not closing and len(interrupts) == 1:
            raise TimeoutError('realization interrupted')
    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    output = args.output.resolve()
    output.mkdir(parents=True, mode=0o700)
    status = {'schema_version': 1, 'status': 'UNAVAILABLE', 'sessions_available': False,
              'adapter_sha256': digest(read(Path(__file__).resolve())),
              'reference_revision': REFERENCE, 'engine_cache_hits': 'UNOBSERVED'}
    final = False
    try:
        try:
            raw = read(args.config)
            if not pin(args.config_sha256) or digest(raw) != args.config_sha256:
                raise ValueError('configuration hash differs')
            config = decode(raw)
            trace_data = read(config.get('trace', ''))
            if digest(trace_data) != config.get('trace_sha256'):
                raise ValueError('normalized trace hash differs')
            rows = validate(config, decode(trace_data))
            if digest(read(config['go_tokenizer'], 128 << 20)) != config['go_tokenizer_sha256']:
                raise ValueError('Go tokenizer executable hash differs')
            identity = runtime_identity(config)
            write(output / 'runtime-identity.json', identity)
            for name, data in [('config.json', raw), ('normalized-trace.json', trace_data)]:
                with (output / name).open('xb') as stream:
                    os.chmod(output / name, 0o600)
                    stream.write(data)
            assets = copy_assets(config, output)
            write(output / 'asset-identity.json', assets)
            available = realize(config, rows, assets, output)
            if digest(read(Path(__file__).resolve())) != status['adapter_sha256']:
                raise ValueError('adapter source changed during realization')
            if os.path.lexists(output / 'status.json'):
                raise ValueError('status.json was written outside the parent')
            status.update(status='SOFTWARE_PASS' if available else 'UNAVAILABLE', sessions_available=available,
                          config_sha256=digest(raw), normalized_trace_sha256=digest(trace_data), requested_rows=len(rows),
                          requested_input_tokens=sum(row['target'] for row in rows))
        except Exception as error:
            # Any failed final check invalidates the runnable export, including an
            # unreadable adapter rather than only a readable hash mismatch.
            status['reason'] = str(error) if isinstance(error, ValueError) else type(error).__name__
        closing.append(True)
        if not status['sessions_available'] and not quarantine(output):
            raise ValueError('cannot clear sessions.json or status.json')
        status['artifacts_sha256'] = {str(p.relative_to(output)): digest(read(p, 64 << 20)) for p in sorted(output.rglob('*'))
                                    if p.is_file() and not p.is_symlink() and p.relative_to(output).parts[0] not in ('tmp', 'hf-cache')}
        write(output / 'status.json', status)
        final = True
    except Exception as error:
        # No final status.json was written, for example after SIGTERM while
        # hashing artifacts or a status.json that could not be cleared.
        status.update(status='UNAVAILABLE', sessions_available=False,
                      reason=str(error) if isinstance(error, ValueError) else type(error).__name__)
    finally:
        closing.append(True)
        if not final:
            quarantine(output)
    # The result is decided once, here: a signal recorded up to this point,
    # including during the quarantine, makes the run unavailable and its final
    # status unqualified. A later signal arrives after the result is decided.
    signalled = bool(interrupts)
    available = final and status['sessions_available'] and not signalled
    if final and signalled:
        quarantine(output)
    print(json.dumps({'status': 'SOFTWARE_PASS' if available else 'UNAVAILABLE', 'sessions_available': available}))
    return 0 if available else 2


if __name__ == '__main__':
    sys.exit(main())
