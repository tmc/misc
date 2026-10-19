"""Strict admission and actual pinned-source/HF/Go realization controls."""
import copy
import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).with_name('aiperf-realize-trace.py')
DELETE = object()
spec = importlib.util.spec_from_file_location('realize', SCRIPT)
bridge = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bridge)


def fixture(targets=(32,), hashes=None, scope='file'):
    hashes = hashes or [[] for _ in targets]
    trace = {'format': 'mooncake-jsonl-v1', 'sourceHash': '1'*64, 'sessions': [
        {'session_id': 'one', 'contextMode': 'live', 'blockSize': 16, 'hashScope': scope,
         'turns': [{'id': str(i), 'sourceRow': i+1, 'inputMode': 'counts', 'inputTokens': target,
                    'inputTarget': None, 'outputBudget': 8, 'hashIDs': ids}
                   for i, (target, ids) in enumerate(zip(targets, hashes, strict=True))]}]}
    config = {'schema_version': 1, 'trace': '/missing', 'trace_sha256': '2'*64, 'source_sha256': '1'*64,
              'assets': '/missing', 'asset_manifest': '/missing', 'asset_manifest_sha256': '3'*64,
              'model': 'Qwen/Qwen3-0.6B', 'model_revision': 'local-pinned-assets', 'go_tokenizer': '/missing',
              'go_tokenizer_sha256': '4'*64, 'python_sha256': '5'*64, 'seed': 18}
    return config, trace


class Admission(unittest.TestCase):
    def test_strict_json(self):
        for data in (b'{"x":1,"x":2}', b'{"x":NaN}', b'{"x":"\xff"}'):
            with self.subTest(data=data), self.assertRaises((ValueError, UnicodeError)):
                bridge.decode(data)

    def test_boundaries_and_exact_hash_strings(self):
        for target in (0, 1, 4096):
            config, trace = fixture((target,))
            self.assertEqual(bridge.validate(config, trace)[0]['target'], target)
        config, trace = fixture((16,), [['9223372036854775807']])
        self.assertEqual(bridge.validate(config, trace)[0]['hash_ids'], ['9223372036854775807'])
        for target in (-1, 4097, True, None):
            config, trace = fixture((target,))
            with self.assertRaises(ValueError):
                bridge.validate(config, trace)

    def test_refusals(self):
        config, trace = fixture()
        for mutate in (
            lambda c, t: c.update(ignored_option=True),
            lambda c, t: t.update(sourceHash='0'*64),
            lambda c, t: t['sessions'][0].update(hashScope='global'),
            lambda c, t: t['sessions'][0]['turns'][0].update(hashIDs=[9007199254740993]),
            lambda c, t: t['sessions'][0]['turns'][0].update(hashIDs=['9223372036854775808']),
            lambda c, t: t['sessions'][0]['turns'][0].update(inputMode='messages'),
            lambda c, t: t['sessions'][0]['turns'][0].update(tools=[{}]),
            lambda c, t: t['sessions'][0]['turns'][0].update(outputBudget=0),
            lambda c, t: t['sessions'][0]['turns'][0].update(outputBudget=33),
        ):
            c, t = copy.deepcopy(config), copy.deepcopy(trace)
            mutate(c, t)
            with self.assertRaises(ValueError):
                bridge.validate(c, t)
        for targets in ((4096,)*5, (1,)*17):
            c, t = fixture(targets)
            with self.assertRaises(ValueError):
                bridge.validate(c, t)

    def test_prefix_namespaces(self):
        c, t = fixture((16,), [['9']], 'session-virtual')
        other = copy.deepcopy(t['sessions'][0])
        other['session_id'] = 'two'
        t['sessions'].append(other)
        rows = bridge.validate(c, t)
        self.assertNotEqual(rows[0]['scope'], rows[1]['scope'])
        for s in t['sessions']:
            s['hashScope'] = 'file'
        rows = bridge.validate(c, t)
        self.assertEqual(rows[0]['scope'], rows[1]['scope'])


class ExportIntegrity(unittest.TestCase):
    """Controlled subprocess fixtures test admission, not tokenizer correctness."""

    def setup_files(self, root, hashes=None):
        executable = root / 'unexecuted-binary-fixture'
        executable.write_bytes(b'fixture only')
        snapshot = root / 'snapshot'
        snapshot.mkdir()
        (snapshot / 'tokenizer.json').write_bytes(b'asset')
        assets = {'cache': str(root / 'cache'), 'snapshot': str(snapshot),
                  'files': {'tokenizer.json': bridge.digest(b'asset')}}
        config, trace = fixture((4,), hashes)
        config.update(go_tokenizer=str(executable), go_tokenizer_sha256=bridge.digest(executable.read_bytes()))
        return config, bridge.validate(config, trace), assets

    def evidence(self, row, **changes):
        result = dict(row, status='HF_VERIFIED', text='x', text_sha256=bridge.digest(b'x'),
                      intended_token_ids=[1, 2, 3, 4], hf_token_ids=[1, 2, 3, 4], hf_count=4,
                      stable_terminator_ids=[5], block_separator_id=None)
        result.update(changes)
        return {k: v for k, v in result.items() if v is not DELETE}

    def go(self, assets, **changes):
        result = {'ids': [1, 2, 3, 4], 'count': 4, 'scope': 'text without postprocessor',
                  'tokenizer_sha256': assets['files']['tokenizer.json'], 'framed_sha256': bridge.digest(b'x')}
        result.update(changes)
        return {k: v for k, v in result.items() if v is not DELETE}

    @contextlib.contextmanager
    def children(self, reference, go, side_effect=None, calls=None):
        """Replace child processes with fixed outputs, keeping run_child's own receipts and checks."""
        class Process:
            pid = None
            def __init__(self, argv, stdout, **_):
                path = Path(stdout.name)
                name = path.name.removesuffix('.stdout')
                if calls is not None:
                    calls.append(name)
                if name == 'reference' and side_effect:
                    side_effect(path.parent)
                # ASCII JSON can carry values, such as lone surrogates, that the
                # adapter's own writer refuses.
                stdout.write(json.dumps(reference if name == 'reference' else go).encode())
            def wait(self, timeout=None):
                return 0
        with patch.object(bridge.subprocess, 'Popen', Process), patch.object(bridge.os, 'killpg'):
            yield

    def realize(self, root, config, rows, assets, reference, go=None):
        """Run realize with fixed child outputs; return its result or error, the child calls and output."""
        output = root / 'out'
        output.mkdir()
        calls = []
        with self.children(reference, go, calls=calls), patch.object(bridge, 'runtime_identity', return_value={}), \
             patch.object(bridge.os, 'sched_getaffinity', return_value={0, 1}, create=True), patch.object(bridge.os, 'sched_setaffinity', create=True):
            try:
                return bridge.realize(config, rows, assets, output), calls, output
            except ValueError as error:
                return error, calls, output

    def test_exact_evidence_exports(self):
        for hashes in (None, [['9']]):
            with self.subTest(hashes=hashes), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                config, rows, assets = self.setup_files(root, hashes)
                available, calls, output = self.realize(root, config, rows, assets, [self.evidence(rows[0])], self.go(assets))
                self.assertIs(available, True)
                self.assertEqual(calls, ['reference', 'go-0'])
                turn = bridge.decode((output / 'sessions.json').read_bytes())[0]['turns'][0]
                self.assertEqual(turn['request']['body']['messages'][0]['content'], 'x')
                self.assertEqual(turn['realization']['text_sha256'], bridge.digest(b'x'))

    def test_worker_cannot_change_admitted_fields(self):
        for field, value in [('output_budget', 9999), ('output_budget', 8.0), ('block_size', 1),
                             ('scope_kind', 'wrong'), ('source', {'id': 'wrong'})]:
            with self.subTest(field=field, value=value), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                config, rows, assets = self.setup_files(root)
                result, calls, output = self.realize(root, config, rows, assets, [self.evidence(rows[0], **{field: value})], self.go(assets))
                self.assertRegex(str(result), 'different attempted population')
                self.assertEqual(calls, ['reference'])
                self.assertFalse((output / 'sessions.json').exists())

    def test_worker_protocol_refusals(self):
        # Each case is a worker protocol violation: the run refuses before any
        # Go call, prefix check or export.
        for name, changes, hashes in [
            ('worker VERIFIED bypass', {'status': 'VERIFIED', 'text': 'not-four-tokens', 'text_sha256': bridge.digest(b'not-four-tokens')}, None),
            ('worker VERIFIED without evidence', {'status': 'VERIFIED', 'text': DELETE, 'text_sha256': DELETE, 'intended_token_ids': DELETE, 'hf_token_ids': DELETE, 'hf_count': DELETE}, None),
            ('non-whitelisted status', {'status': 'GO_VERIFIED'}, None),
            ('status case', {'status': 'hf_verified'}, None),
            ('missing status', {'status': DELETE}, None),
            ('non-string status', {'status': True}, None),
            ('non-whitelisted stage field', {'stage': 'go'}, None),
            ('worker Go evidence', {'go_token_ids': [1, 2, 3, 4], 'go_count': 4}, None),
            ('null ids', {'hf_token_ids': None, 'intended_token_ids': None}, None),
            ('float ids', {'hf_token_ids': [1.0, 2.0, 3.0, 4.0]}, None),
            ('bool ids', {'hf_token_ids': [True, 2, 3, 4]}, None),
            ('negative id', {'hf_token_ids': [-1, 2, 3, 4]}, None),
            ('float count', {'hf_count': 4.0}, None),
            ('bool count', {'hf_token_ids': [1], 'hf_count': True}, None),
            ('length-1 ids count 4', {'hf_token_ids': [1], 'intended_token_ids': [1]}, None),
            ('length-1 ids count 1 target 4', {'hf_token_ids': [1], 'intended_token_ids': [1], 'hf_count': 1}, None),
            ('text sha mismatch', {'text_sha256': bridge.digest(b'y')}, None),
            ('text not string', {'text': ['x']}, None),
            ('lone surrogate text', {'text': '\ud800'}, None),
            ('partial evidence', {'intended_token_ids': DELETE}, None),
            ('verified with error', {'reason': 'x'}, None),
            ('float terminator', {'stable_terminator_ids': [5.0]}, None),
            ('bool separator', {'block_separator_id': False}, None),
            ('unavailable without reason', {'status': 'UNAVAILABLE'}, None),
            ('cached ids differ from intended', {'intended_token_ids': [1, 2, 3, 5]}, [['9']]),
        ]:
            with self.subTest(name), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                config, rows, assets = self.setup_files(root, hashes)
                result, calls, output = self.realize(root, config, rows, assets, [self.evidence(rows[0], **changes)], self.go(assets))
                self.assertIsInstance(result, ValueError)
                self.assertNotRegex(str(result), 'different attempted population')
                self.assertEqual(calls, ['reference'])
                self.assertFalse((output / 'sessions.json').exists())
                self.assertFalse((output / 'realization.json').exists())

    def test_go_evidence_refusals(self):
        # The parent's own Go run ran but its evidence does not match exactly:
        # the row and therefore the whole population is unavailable.
        for name, changes in [
            ('null ids', {'ids': None}),
            ('missing ids', {'ids': DELETE}),
            ('float ids and count', {'ids': [1.0, 2.0, 3.0, 4.0], 'count': 4.0}),
            ('float count', {'count': 4.0}),
            ('bool ids', {'ids': [True, 2, 3, 4]}),
            ('bool count', {'ids': [1], 'count': True}),
            ('length-1 ids count 4', {'ids': [1]}),
            ('short ids and count', {'ids': [1, 2, 3], 'count': 3}),
            ('different ids', {'ids': [1, 2, 3, 5]}),
            ('text sha mismatch', {'framed_sha256': bridge.digest(b'y')}),
            ('missing text sha', {'framed_sha256': DELETE}),
            ('tokenizer differs', {'tokenizer_sha256': '0'*64}),
            ('postprocessor scope', {'scope': 'text with asset-defined single-input postprocessor'}),
        ]:
            with self.subTest(name), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                config, rows, assets = self.setup_files(root)
                available, calls, output = self.realize(root, config, rows, assets, [self.evidence(rows[0])], self.go(assets, **changes))
                self.assertIs(available, False)
                self.assertEqual(calls, ['reference', 'go-0'])
                self.assertFalse((output / 'sessions.json').exists())
                cell = bridge.decode((output / 'realization.json').read_bytes())['results'][0]
                self.assertEqual(cell['status'], 'UNAVAILABLE')
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            config, rows, assets = self.setup_files(root)
            available, _, output = self.realize(root, config, rows, assets, [self.evidence(rows[0])], [1, 2, 3, 4])
            self.assertIs(available, False)
            self.assertFalse((output / 'sessions.json').exists())

    def test_reference_unavailable_skips_go(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            config, rows, assets = self.setup_files(root)
            reference = self.evidence(rows[0], status='UNAVAILABLE', hf_token_ids=[1], hf_count=1,
                                      reason='reference decoded text does not reencode to declared target')
            available, calls, output = self.realize(root, config, rows, assets, [reference], self.go(assets))
            self.assertIs(available, False)
            self.assertEqual(calls, ['reference'])
            self.assertFalse((output / 'sessions.json').exists())

    def main(self, root, reference, side_effect=None, read=None):
        """Run main with fixed child outputs; return its exit code or error and the output directory."""
        config, rows, assets = self.setup_files(root)
        trace = root / 'trace.json'
        trace.write_bytes(b'{}')
        config.update(trace=str(trace), trace_sha256=bridge.digest(b'{}'))
        config_path = root / 'config.json'
        config_path.write_bytes(bridge.encoded(config))
        output = root / 'out'
        original_read = bridge.read
        def hooked_read(path, limit=bridge.MAX_BYTES):
            if read:
                read(Path(path), output)
            return original_read(path, limit)
        argv = ['realize', '--config', str(config_path), '--config-sha256', bridge.digest(config_path.read_bytes()), '--output', str(output)]
        handlers = {n: signal.getsignal(n) for n in (signal.SIGTERM, signal.SIGINT)}
        self.stdout = io.StringIO()
        try:
            with patch.object(sys, 'argv', argv), patch.object(bridge, 'validate', return_value=rows), \
                 patch.object(bridge, 'runtime_identity', return_value={}), patch.object(bridge, 'copy_assets', return_value=assets), \
                 self.children([reference(rows[0])], self.go(assets), side_effect), patch.object(bridge, 'read', hooked_read), \
                 patch.object(bridge.os, 'sched_getaffinity', return_value={0, 1}, create=True), \
                 patch.object(bridge.os, 'sched_setaffinity', create=True), contextlib.redirect_stdout(self.stdout):
                return bridge.main(), output
        except Exception as error:
            return error, output
        finally:
            for n, handler in handlers.items():
                signal.signal(n, handler)

    def test_worker_bypass_fails_main(self):
        with tempfile.TemporaryDirectory() as temporary:
            code, output = self.main(Path(temporary), lambda row: self.evidence(row, status='VERIFIED'))
            status = bridge.decode((output / 'status.json').read_bytes())
            self.assertEqual(code, 2)
            self.assertEqual(status['status'], 'UNAVAILABLE')
            self.assertRegex(status['reason'], 'worker protocol')
            self.assertFalse((output / 'sessions.json').exists())

    def test_worker_output_files_quarantine(self):
        # The worker is adversarial beyond stdout: it shares the output
        # directory and must not leave a runnable or altered artifact there.
        forged = lambda output: (output / 'sessions.json').write_text('[{"forged": true}]')
        unavailable = lambda row: dict(row, status='UNAVAILABLE', reason='x')
        for name, reference, side_effect in [
            ('forged sessions with unavailable row', unavailable, forged),
            ('forged sessions with exact evidence', self.evidence, forged),
            ('forged status', self.evidence, lambda output: (output / 'status.json').write_text('{}')),
            ('forged unqualified sessions', self.evidence, lambda output: (output / 'unqualified-sessions.json').write_text('[]')),
            ('changed worker job', self.evidence, lambda output: (output / 'worker-job.json').write_text('{}')),
            ('removed asset identity', self.evidence, lambda output: (output / 'asset-identity.json').unlink()),
            ('new directory', self.evidence, lambda output: (output / 'extra').mkdir()),
            ('symlink', self.evidence, lambda output: (output / 'link').symlink_to(output / 'config.json')),
            ('forged command receipt', self.evidence, lambda output: (output / 'reference.command.json').write_text('{"argv":["forged"]}')),
        ]:
            with self.subTest(name), tempfile.TemporaryDirectory() as temporary:
                code, output = self.main(Path(temporary), reference, side_effect)
                status = bridge.decode((output / 'status.json').read_bytes())
                self.assertEqual(code, 2)
                self.assertFalse(status['sessions_available'])
                self.assertFalse((output / 'sessions.json').exists())
                self.assertFalse((output / 'realization.json').exists())

    def test_unavailable_path_quarantines_late_sessions(self):
        # A sessions.json that appears without the parent's final export, for
        # example from a surviving worker descendant, is never left runnable.
        def realize(config, rows, assets, output):
            (output / 'sessions.json').write_text('[{"forged": true}]')
            return False
        with tempfile.TemporaryDirectory() as temporary, patch.object(bridge, 'realize', realize):
            code, output = self.main(Path(temporary), self.evidence)
            self.assertEqual(code, 2)
            self.assertFalse((output / 'sessions.json').exists())
            self.assertTrue((output / 'unqualified-sessions.json').exists())

    def test_interrupt_after_export_quarantines(self):
        # SIGTERM while main hashes artifacts after a successful export.
        sent = []
        def read(path, output):
            if path.name == 'sessions.json' and not sent:
                sent.append(path)
                os.kill(os.getpid(), signal.SIGTERM)
        with tempfile.TemporaryDirectory() as temporary:
            code, output = self.main(Path(temporary), self.evidence, read=read)
            self.assertEqual(code, 2)
            self.assertTrue(sent)
            self.assertFalse((output / 'sessions.json').exists())
            self.assertTrue((output / 'unqualified-sessions.json').exists())
            self.assertFalse((output / 'status.json').exists())
            self.assertEqual(json.loads(self.stdout.getvalue()), {'status': 'UNAVAILABLE', 'sessions_available': False})

    def test_second_interrupt_during_quarantine(self):
        # A second SIGTERM arriving inside the final quarantine is held until
        # the quarantine finishes.
        sent = []
        def read(path, output):
            if path.name == 'sessions.json' and not sent:
                sent.append(path)
                os.kill(os.getpid(), signal.SIGTERM)
        lexists = os.path.lexists
        def hooked_lexists(path):
            if Path(path).name == 'sessions.json' and len(sent) == 1:
                sent.append(path)
                os.kill(os.getpid(), signal.SIGTERM)
            return lexists(path)
        with tempfile.TemporaryDirectory() as temporary, patch.object(bridge.os.path, 'lexists', hooked_lexists):
            code, output = self.main(Path(temporary), self.evidence, read=read)
            self.assertEqual(len(sent), 2)
            self.assertEqual(code, 2)
            self.assertFalse((output / 'sessions.json').exists())
            self.assertTrue((output / 'unqualified-sessions.json').exists())

    def test_blocked_quarantine_targets(self):
        forged_status = json.dumps({'schema_version': 1, 'status': 'SOFTWARE_PASS', 'sessions_available': True, 'forged': True})
        def nonempty(path):
            path.mkdir()
            (path / 'x').write_text('x')
        def both(output):
            nonempty(output / 'unqualified-sessions.json')
            (output / 'sessions.json').write_text('[{"forged": true}]')
            (output / 'status.json').write_text(forged_status)
        def status(output):
            nonempty(output / 'unqualified-status.json')
            (output / 'status.json').write_text(forged_status)
        def sessions_directory(output):
            nonempty(output / 'sessions.json')
            (output / 'unqualified-sessions.json').write_text('[]')
        for name, side_effect in [('unqualified sessions directory', both), ('unqualified status directory', status),
                                  ('sessions directory', sessions_directory)]:
            with self.subTest(name), tempfile.TemporaryDirectory() as temporary:
                code, output = self.main(Path(temporary), self.evidence, side_effect)
                self.assertEqual(code, 2)
                self.assertFalse(os.path.lexists(output / 'sessions.json'))
                status_value = bridge.decode((output / 'status.json').read_bytes())
                self.assertNotIn('forged', status_value)
                self.assertFalse(status_value['sessions_available'])
                self.assertEqual(json.loads(self.stdout.getvalue()), {'status': 'UNAVAILABLE', 'sessions_available': False})

    def test_quarantine_never_raises(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            for n in ('', '.1', '.2'):
                (output / ('unqualified-sessions.json' + n)).mkdir()
                (output / ('unqualified-sessions.json' + n) / 'x').write_text('x')
            (output / 'sessions.json').mkdir()
            (output / 'sessions.json' / 'x').write_text('x')
            (output / 'status.json').symlink_to(output / 'missing')
            self.assertTrue(bridge.quarantine(output))
            self.assertFalse(os.path.lexists(output / 'sessions.json'))
            self.assertFalse(os.path.lexists(output / 'status.json'))
            (output / 'sessions.json').write_text('[]')
            with patch.object(bridge.Path, 'rename', side_effect=OSError), patch.object(bridge.Path, 'unlink', side_effect=OSError):
                self.assertFalse(bridge.quarantine(output))

    def test_quarantine_restores_permissions(self):
        # Quarantine runs after the worker's process group is gone, so it may
        # undo permission and flag changes the worker made.
        cases = [('read-only output', lambda output: output.chmod(0o500)),
                 ('read-only status', lambda output: (output / 'status.json').chmod(0o400))]
        if hasattr(os, 'chflags') and hasattr(stat, 'UF_IMMUTABLE'):
            cases.append(('immutable status', lambda output: os.chflags(output / 'status.json', stat.UF_IMMUTABLE)))
        for name, lock in cases:
            with self.subTest(name), tempfile.TemporaryDirectory() as temporary:
                output = Path(temporary) / 'out'
                output.mkdir()
                (output / 'sessions.json').write_text('[]')
                (output / 'status.json').write_text('{}')
                try:
                    lock(output)
                except OSError as error:
                    self.skipTest('cannot set flag: %s' % error)
                self.assertTrue(bridge.quarantine(output))
                self.assertEqual(sorted(p.name for p in output.iterdir()), ['unqualified-sessions.json', 'unqualified-status.json'])

    def test_worker_locked_forgery(self):
        forged_status = json.dumps({'schema_version': 1, 'status': 'SOFTWARE_PASS', 'sessions_available': True, 'forged': True})
        def read_only_output(output):
            (output / 'status.json').write_text(forged_status)
            (output / 'sessions.json').write_text('[{"forged": true}]')
            output.chmod(0o500)
        def immutable_status(output):
            (output / 'status.json').write_text(forged_status)
            os.chflags(output / 'status.json', stat.UF_IMMUTABLE)
        def every_target_taken(output):
            for name in ('sessions.json', 'status.json'):
                for n in range(16):
                    target = output / ('unqualified-' + name + ('.%d' % n if n else ''))
                    target.mkdir()
                    (target / 'x').write_text('x')
            (output / 'sessions.json' / 'locked').mkdir(parents=True)
            (output / 'sessions.json' / 'locked' / 'x').write_text('x')
            (output / 'sessions.json' / 'locked').chmod(0o500)
            (output / 'status.json').write_text(forged_status)
        cases = [('read-only output', read_only_output), ('every target taken', every_target_taken)]
        if sys.platform == 'darwin' and hasattr(stat, 'UF_IMMUTABLE'):
            cases.append(('immutable status', immutable_status))
        for name, side_effect in cases:
            with self.subTest(name), tempfile.TemporaryDirectory() as temporary:
                code, output = self.main(Path(temporary), self.evidence, side_effect)
                self.assertEqual(code, 2)
                self.assertEqual(json.loads(self.stdout.getvalue()), {'status': 'UNAVAILABLE', 'sessions_available': False})
                self.assertFalse(os.path.lexists(output / 'sessions.json'))
                status_value = bridge.decode((output / 'status.json').read_bytes())
                self.assertNotIn('forged', status_value)
                self.assertFalse(status_value['sessions_available'])

    def test_late_status_refuses(self):
        # A status.json that appears after the worker check but before the
        # parent's final write, for example from an escaped descendant.
        realize = bridge.realize
        for name, late in [('file', lambda output: (output / 'status.json').write_text('{"forged": true}')),
                           ('fifo', lambda output: os.mkfifo(output / 'status.json'))]:
            def hooked(config, rows, assets, output):
                available = realize(config, rows, assets, output)
                late(output)
                return available
            with self.subTest(name), tempfile.TemporaryDirectory() as temporary, patch.object(bridge, 'realize', hooked):
                code, output = self.main(Path(temporary), self.evidence)
                self.assertEqual(code, 2)
                self.assertFalse(os.path.lexists(output / 'sessions.json'))
                status_value = bridge.decode((output / 'status.json').read_bytes())
                self.assertEqual(status_value['reason'], 'status.json was written outside the parent')

    def test_interrupts_in_final_phase(self):
        # Any SIGTERM or SIGINT, alone or together, during hashing, the status
        # write or the final quarantine leaves an unavailable result.
        def at_read(*signals):
            def hook(state, path):
                if path.name == 'sessions.json' and not state:
                    state.append(path)
                    for n in signals:
                        os.kill(os.getpid(), n)
            return hook
        for name, signals in [('SIGINT', (signal.SIGINT,)), ('SIGTERM and SIGINT', (signal.SIGTERM, signal.SIGINT))]:
            with self.subTest(name + ' while hashing'), tempfile.TemporaryDirectory() as temporary:
                state, hook = [], at_read(*signals)
                code, output = self.main(Path(temporary), self.evidence, read=lambda path, output: hook(state, path))
                self.assertTrue(state)
                self.assertEqual(code, 2)
                self.assertEqual(json.loads(self.stdout.getvalue()), {'status': 'UNAVAILABLE', 'sessions_available': False})
                self.assertFalse(os.path.lexists(output / 'sessions.json'))
                self.assertFalse(os.path.lexists(output / 'status.json'))
        write = bridge.write
        def interrupted_write(path, value):
            result = write(path, value)
            if Path(path).name == 'status.json':
                os.kill(os.getpid(), signal.SIGINT)
                os.kill(os.getpid(), signal.SIGTERM)
            return result
        with tempfile.TemporaryDirectory() as temporary, patch.object(bridge, 'write', interrupted_write):
            code, output = self.main(Path(temporary), self.evidence)
            self.assertEqual(code, 2)
            self.assertEqual(json.loads(self.stdout.getvalue()), {'status': 'UNAVAILABLE', 'sessions_available': False})
            self.assertFalse(os.path.lexists(output / 'sessions.json'))
            self.assertFalse(os.path.lexists(output / 'status.json'))
            self.assertTrue((output / 'unqualified-status.json').exists())

    def test_child_cannot_rewrite_command_receipt(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary)
            receipt = output / 'child.command.json'
            argv = [sys.executable, '-c', 'import sys; open(sys.argv[1], "w").write("{}")', str(receipt)]
            with self.assertRaisesRegex(ValueError, 'command receipt'):
                bridge.run_child(argv, output, 'child', dict(os.environ), 10)
            self.assertEqual(bridge.run_child([sys.executable, '-c', ''], output, 'control', dict(os.environ), 10), 0)

    def injected(self, *points):
        """Load a copy of the adapter that sends itself SIGTERM before each source line in points."""
        source = SCRIPT.read_text()
        for point in points:
            self.assertEqual(source.count(point), 1, point)
            indent = point[:len(point) - len(point.lstrip())]
            source = source.replace(point, indent + 'os.kill(os.getpid(), signal.SIGTERM)\n' + point)
        directory = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, directory)
        (directory / SCRIPT.name).write_text(source)
        module_spec = importlib.util.spec_from_file_location('injected', directory / SCRIPT.name)
        module = importlib.util.module_from_spec(module_spec)
        module_spec.loader.exec_module(module)
        return patch.object(sys.modules[__name__], 'bridge', module)

    def test_interrupt_after_final_check(self):
        # A signal recorded after the finally block's check, while the result
        # is being decided, still makes the run unavailable.
        for name, reference in [('available', self.evidence), ('unavailable', lambda row: dict(row, status='UNAVAILABLE', reason='x'))]:
            with self.subTest(name), tempfile.TemporaryDirectory() as temporary, \
                 self.injected('    signalled = bool(interrupts)\n'):
                code, output = self.main(Path(temporary), reference)
                self.assertEqual(code, 2)
                self.assertEqual(json.loads(self.stdout.getvalue()), {'status': 'UNAVAILABLE', 'sessions_available': False})
                self.assertFalse(os.path.lexists(output / 'sessions.json'))
                if name == 'available':
                    self.assertFalse(os.path.lexists(output / 'status.json'))

    def test_two_early_interrupts_quarantine(self):
        # A second signal while the first unwinds is only recorded, so the
        # quarantine still runs.
        with tempfile.TemporaryDirectory() as temporary, \
             self.injected("        closing.append(True)\n        if not status['sessions_available']",
                           '        closing.append(True)\n        if not final:'):
            code, output = self.main(Path(temporary), self.evidence)
            self.assertEqual(code, 2)
            self.assertEqual(json.loads(self.stdout.getvalue()), {'status': 'UNAVAILABLE', 'sessions_available': False})
            self.assertFalse(os.path.lexists(output / 'sessions.json'))
            self.assertFalse(os.path.lexists(output / 'status.json'))

    def test_hard_link_outside_unchanged(self):
        # Quarantine must not chmod an inode shared with a file outside the
        # output directory; the link itself is still moved.
        with tempfile.TemporaryDirectory() as temporary:
            outside = Path(temporary) / 'outside'
            outside.write_text('v')
            outside.chmod(0o400)
            run = Path(temporary) / 'run'
            run.mkdir()
            code, output = self.main(run, self.evidence, lambda output: os.link(outside, output / 'sessions.json'))
            self.assertEqual(code, 2)
            self.assertEqual(stat.S_IMODE(outside.stat().st_mode), 0o400)
            self.assertFalse(os.path.lexists(output / 'sessions.json'))
            self.assertEqual((output / 'unqualified-sessions.json').read_text(), 'v')

    def test_output_under_hf_cache_path(self):
        # Only the output's own hf-cache and tmp directories are left out of
        # the artifact digests, not any component of its absolute path.
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / 'hf-cache' / 'tmp'
            root.mkdir(parents=True)
            code, output = self.main(root, self.evidence)
            self.assertEqual(code, 0)
            artifacts = bridge.decode((output / 'status.json').read_bytes())['artifacts_sha256']
            self.assertIn('sessions.json', artifacts)
            self.assertIn('reference.command.json', artifacts)

    def test_final_identity_failure_quarantines_export(self):
        for failure in ('unreadable', 'changed', None):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                config, rows, assets = self.setup_files(root)
                trace = root / 'trace.json'
                trace.write_bytes(b'{}')
                config.update(trace=str(trace), trace_sha256=bridge.digest(b'{}'))
                config_path = root / 'config.json'
                config_path.write_bytes(bridge.encoded(config))
                output = root / 'out'
                exported = False
                original_read = bridge.read
                def read(path, limit=bridge.MAX_BYTES):
                    if exported and Path(path).resolve() == SCRIPT.resolve():
                        if failure == 'unreadable':
                            raise ValueError('controlled unreadable adapter')
                        if failure == 'changed':
                            return b'controlled changed adapter'
                    return original_read(path, limit)
                def realize(config, rows, assets, output):
                    nonlocal exported
                    bridge.write(output / 'sessions.json', [{'session_id': 'fixture', 'turns': []}])
                    exported = True
                    return True
                argv = ['realize', '--config', str(config_path), '--config-sha256', bridge.digest(config_path.read_bytes()), '--output', str(output)]
                with patch.object(sys, 'argv', argv), patch.object(bridge, 'read', read), \
                     patch.object(bridge, 'validate', return_value=rows), patch.object(bridge, 'runtime_identity', return_value={}), \
                     patch.object(bridge, 'copy_assets', return_value=assets), patch.object(bridge, 'realize', realize), \
                     contextlib.redirect_stdout(io.StringIO()):
                    code = bridge.main()
                status = bridge.decode((output / 'status.json').read_bytes())
                self.assertEqual(code, 2 if failure else 0)
                self.assertEqual(status['sessions_available'], not bool(failure))
                self.assertEqual((output / 'sessions.json').exists(), not bool(failure))
                self.assertEqual((output / 'unqualified-sessions.json').exists(), bool(failure))


@unittest.skipUnless(os.environ.get('AIPERF_REALIZE_REFERENCE') == '1', 'actual pinned AIPerf runtime/assets/Go binary required')
class ActualReference(unittest.TestCase):
    def execute(self, trace, directory):
        config, _ = fixture()
        root = Path(directory)
        trace_path = root/'trace.json'
        trace_path.write_bytes(bridge.encoded(trace))
        config.update(trace=str(trace_path), trace_sha256=bridge.digest(trace_path.read_bytes()),
                      assets=os.environ['REALIZE_ASSETS'], asset_manifest=os.environ['REALIZE_ASSET_MANIFEST'],
                      asset_manifest_sha256=bridge.digest(Path(os.environ['REALIZE_ASSET_MANIFEST']).read_bytes()),
                      go_tokenizer=os.environ['REALIZE_GO_TOKENIZER'], go_tokenizer_sha256=os.environ['REALIZE_GO_TOKENIZER_SHA256'],
                      python_sha256=bridge.digest(Path(sys.executable).resolve().read_bytes()))
        config_path = root/'config.json'
        config_path.write_bytes(bridge.encoded(config))
        output = root/'out'
        run = subprocess.run([sys.executable, str(SCRIPT), '--config', str(config_path), '--config-sha256', bridge.digest(config_path.read_bytes()), '--output', str(output)], capture_output=True, timeout=30)
        return run, output, config

    def temporary(self):
        root = Path(os.environ['TMPDIR'])
        if os.environ.get('REALIZE_RETAIN_TEST_ARTIFACTS') == '1':
            return contextlib.nullcontext(tempfile.mkdtemp(dir=root))
        return tempfile.TemporaryDirectory(dir=root)

    def test_count_only_success(self):
        _, trace = fixture((32,))
        with self.temporary() as tmp:
            run, output, _ = self.execute(trace, tmp)
            self.assertEqual(run.returncode, 0, run.stderr + run.stdout + (output/'status.json').read_bytes())
            cells = json.loads((output/'realization.json').read_bytes())['results']
            self.assertEqual(cells[0]['hf_count'], 32)
            self.assertEqual(cells[0]['hf_token_ids'], cells[0]['go_token_ids'])
            sessions = json.loads((output/'sessions.json').read_bytes())
            self.assertEqual(sessions[0]['turns'][0]['request']['body']['messages'][0]['content'], cells[0]['text'])

    def test_actual_singleton_and_batched_source_equivalence(self):
        _, trace = fixture((32, 32, 48), [['17', '18'], ['17', '18'], ['17', '18']])
        with self.temporary() as tmp:
            run, output, config = self.execute(trace, tmp)
            self.assertIn(run.returncode, (0, 2))
            report = json.loads((output/'realization.json').read_bytes())
            cells = report['results']
            assets = json.loads((output/'asset-identity.json').read_bytes())
            os.environ.update(HF_HUB_OFFLINE='1', TRANSFORMERS_OFFLINE='1', HF_HUB_CACHE=assets['cache'],
                              USE_TORCH='0', USE_TF='0', USE_FLAX='0', TOKENIZERS_PARALLELISM='false')
            from aiperf.common import random_generator as rng
            from aiperf.common.tokenizer import Tokenizer
            from aiperf.dataset.generator.prompt import PromptGenerator
            from aiperf.dataset.loader.hash_ids_synthesis import HashIdsPromptRequest, HashIdsPromptSynthesisMixin
            rng.reset()
            rng.init(config['seed'])
            tokenizer = Tokenizer.from_pretrained(assets['repo'], revision=assets['revision'], trust_remote_code=False, resolve_alias=False)
            pg = PromptGenerator(prompts=None, prefix_prompts=None, tokenizer=tokenizer)
            pg._hash_id_corpus_rng.set_trace_id(trace['sourceHash'])
            host = HashIdsPromptSynthesisMixin()
            host.prompt_generator, host._tokenizer_name = pg, assets['repo']
            host._trust_remote_code, host._tokenizer_revision, host._block_size = False, assets['revision'], 16
            expected = host.synthesize_prompts_from_hash_ids([HashIdsPromptRequest(str(i), list(map(int, ids)), n) for i, (n, ids) in enumerate([(32, ['17','18']), (32, ['17','18']), (48, ['17','18'])])])
            self.assertEqual([c['text'] for c in cells], [expected[str(i)] for i in range(3)])
            self.assertEqual(cells[0]['hf_token_ids'], cells[1]['hf_token_ids'])
            self.assertEqual(cells[0]['intended_token_ids'], cells[2]['intended_token_ids'][:32])
            for check in report['prefix_checks']:
                self.assertTrue(check['matches_prior'])
            for cell in cells:
                if cell['status'] == 'VERIFIED':
                    self.assertEqual(cell['hf_token_ids'], cell['go_token_ids'])
                else:
                    self.assertIn('reason', cell)
            self.assertEqual((output/'sessions.json').exists(), all(c['status']=='VERIFIED' for c in cells))

    def test_actual_session_virtual_scopes(self):
        _, trace = fixture((16,), [['17']], 'session-virtual')
        other = copy.deepcopy(trace['sessions'][0])
        other['session_id'] = 'two'
        trace['sessions'].append(other)
        with self.temporary() as tmp:
            run, output, _ = self.execute(trace, tmp)
            self.assertEqual(run.returncode, 0, run.stdout + run.stderr)
            report = json.loads((output/'realization.json').read_bytes())
            cells = report['results']
            self.assertNotEqual(cells[0]['scope'], cells[1]['scope'])
            for cell in cells:
                self.assertEqual(cell['hf_token_ids'], cell['go_token_ids'])
                self.assertEqual(cell['hf_count'], 16)
            self.assertTrue(all(not c['seen_before'] for c in report['prefix_checks']))
            self.assertEqual(len(json.loads((output/'sessions.json').read_bytes())), 2)

    def test_zero_and_conflicting_hash_size_preserve_unavailable(self):
        _, trace = fixture((0, 16, 8, 0), [[], ['17'], ['17'], ['19']])
        with self.temporary() as tmp:
            run, output, _ = self.execute(trace, tmp)
            self.assertEqual(run.returncode, 2)
            self.assertFalse((output/'sessions.json').exists())
            cells = json.loads((output/'realization.json').read_bytes())['results']
            self.assertEqual(cells[0]['target'], 0)
            self.assertEqual(cells[0]['hf_count'], 1)
            self.assertEqual(cells[0]['status'], 'UNAVAILABLE')
            self.assertEqual(cells[2]['reference_error_type'], 'ConfigurationError')
            self.assertIn('already materialized', cells[2]['reference_error_message'])
            self.assertEqual(cells[3]['reference_error_type'], 'ConfigurationError')
            self.assertIn('both be greater than 0', cells[3]['reference_error_message'])


if __name__ == '__main__':
    unittest.main()
