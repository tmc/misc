# Offline trace prompt realization

`scripts/aiperf-realize-trace.py` turns bounded normalized count/cache snapshots
into authored requests using the actual pinned AIPerf `PromptGenerator` and
`HashIdsPromptSynthesisMixin`. It requires a local tokenizer/corpus and verifies
the resulting text with both actual Hugging Face and an independent Go tokenizer.
It makes no HTTP, inference or GPU calls.

The reference is AIPerf `f10eb50a1776775d80d7f13396b0f668f14e2c03`. All 1025 installed
Python source files must match its pinned tree, and the Shakespeare corpus must
match its byte hash. The adapter checks Python executable identity and dependency
versions before importing the generator. There is no fake counter, builtin
replacement tokenizer, model download or arbitrary tokenizer Python execution.

## Inputs and bounds

First normalize a source file with `llm-import-trace -output normalized`. Preserve
the original source bytes separately. The normalized byte hash and its original
`sourceHash` declaration are independent pins; the adapter checks both against
configuration. It does not reconstruct or independently authenticate missing
original source bytes.

The adapter accepts only `inputMode: "counts"`, a file or session-virtual hash
scope, and an explicit output budget of 1–32. Inputs represent independent complete
prompt snapshots. Mooncake's generated-count `contextMode: "live"` is retained as
source metadata, but the output policy explicitly uses complete synthetic
snapshots: responses never become later context. Tools, modality inputs,
extra-body fields and authored/mixed input modes are refused.

Maximum population is 16 turns, 4096 target input tokens per turn, 16384 total,
64 hash IDs per turn, and block sizes 1–512. Targets must be explicit: missing
and zero differ. Zero reaches the actual source algorithm; it is not replaced
with filler or an invented empty prompt. The pinned no-hash path floors zero to
one and the hashed path refuses zero, so exact-zero realization is unavailable.

Illustrative configuration (replace all placeholder paths, labels and SHA256s);
these are the complete supported fields:

```json
{
  "schema_version": 1,
  "trace": "/absolute/path/normalized.json",
  "trace_sha256": "SHA256_OF_NORMALIZED_BYTES",
  "source_sha256": "ORIGINAL_SOURCE_HASH_FROM_NORMALIZATION",
  "assets": "/absolute/path/local-tokenizer-assets",
  "asset_manifest": "/absolute/path/asset-manifest.json",
  "asset_manifest_sha256": "SHA256_OF_MANIFEST_BYTES",
  "model": "Qwen/Qwen3-0.6B",
  "model_revision": "DECLARED_ASSET_ORIGIN_LABEL",
  "go_tokenizer": "/absolute/path/fresh-llm-tokenize",
  "go_tokenizer_sha256": "SHA256_OF_GO_EXECUTABLE",
  "python_sha256": "SHA256_OF_RESOLVED_PYTHON_EXECUTABLE",
  "seed": 18
}
```

Use a dedicated asset directory with exactly `tokenizer.json`,
`tokenizer_config.json`, `merges.txt`, and `vocab.json`. Supply every file's SHA256
in the separate manifest. Symlinks, extra/unlisted files and changed files fail;
assets total at most 64 MiB. Model/revision labels are caller declarations;
manifest byte hashes identify the actual assets.

The pinned tokenizer's offline loader resolves Hugging Face cache snapshots,
including when a caller supplies an absolute directory. The adapter therefore
copies the verified files into its own output directory's HF cache, under a
local content-derived repo/revision alias. It uses the actual unmodified loader,
with offline flags, `resolve_alias=False` and `trust_remote_code=False`. The
local cache revision is not represented as an upstream model commit.

Invoke the script with the pinned runtime and a new output directory:

```sh
python scripts/aiperf-realize-trace.py \
  --config config.json --config-sha256 "$CONFIG_SHA256" \
  --output /path/to/task/realized
```

Reference and Go children inherit at most two allowed CPU cores. Child processes have a
30-second deadline and 8 MiB file-output limit; the adapter has a 120-second
aggregate child-dispatch deadline. A timeout kills the owned child process group,
including interruptions of the parent. Output JSON is limited to 8 MiB. Failed
attempt directories are retained; use a new directory for a changed attempt.

## What is verified

The source generator samples real corpus IDs. A transparent subclass records the
unchanged return of the actual `_build_token_sequence` and initial input to the
actual `_decode_to_exact_len`, without sampling twice or modifying their result.
The mixin is called in singleton batches: this preserves input/RNG ordering and
shared hash cache state while keeping the unchanged `parallel_decode` below its
10-sequence process-pool threshold. Functional controls compare that policy with
the actual unwrapped generator/mixin in a batched call.

File-scoped IDs share one generator/cache across sessions. Session-virtual IDs
use a source-hash/session namespace. All hash IDs stay decimal strings in
artifacts; only the source call converts them to exact Python integers. Conflicting
sizes for one hash ID remain the source algorithm's error.

The diagnostic records sampled intended IDs, literal decoded text, re-encoded HF
IDs/count, actual stable-terminator probe IDs, block separator, and independent Go
IDs/count. No-hash generation may perform the source's own bounded length
correction; cached sequences are never repaired. Export requires the exact target
count, unchanged cached IDs through decoding/re-encoding, equality of repeated
scope/hash blocks, and exact Go/HF ID equality. The Go tokenizer uses raw text
without the asset postprocessor.

A terminator probe is evidence about that source probe only. The pinned hash
sequence builder does not apply the probed terminator, so the adapter does not
stamp one into cached blocks. The pinned correction loop can also return a
mismatched result after its tenth iteration. Either source behavior remains
visible; it never becomes a manufactured successful count.

## Outputs and replay scope

`realization.json` retains every attempted turn, including source exceptions and
ID/count drift. A per-row `reference-progress.jsonl` also survives an interrupted
reference subprocess. `status.json`, per-command stdout/stderr/exit receipts, package,
corpus, asset and executable hashes accompany it. `sessions.json` exists only
when **all** requested turns pass; there is no favorable-only partial export.
An unavailable turn returns exit 2 and preserves the rest of the diagnostic.

A verified `sessions.json` has the existing raw-session runner's authored request
shape and can be supplied to `examples/workloads/raw-sessions.js` in a separately
authorized run. Each body contains one synthetic user message and the declared
output budget. That file's existence authorizes no dispatch.

Counts describe textual content with the pinned offline tokenizer. Chat framing,
server defaults, context truncation and engine tokenizer behavior can add or
change tokens. Recorded cache IDs preserve declared content reuse; they do not
prove engine cache hits, cache locality, real source prompt recovery, timing
replay, agent/tool behavior or performance parity. Source arrival/delay is not
scheduled by this adapter or the serial raw-session runner.

Use Python 3.10 or later for admission controls (the pinned runtime is required
for actual-source controls):

```sh
python -m unittest discover -s scripts -p aiperf_realize_trace_test.py
```

Actual-source controls additionally require `AIPERF_REALIZE_REFERENCE=1`,
`REALIZE_ASSETS`, `REALIZE_ASSET_MANIFEST`, `REALIZE_GO_TOKENIZER`,
`REALIZE_GO_TOKENIZER_SHA256`, and a task-local `TMPDIR`, using the pinned runtime.
They exercise real counting, singleton/batched source equivalence, shared-prefix
IDs, zero-target behavior and conflicting hash sizes. They never call a model.

## Worker trust boundary

The reference worker runs pinned AIPerf and Hugging Face code in a child process.
The parent treats it as adversarial, not only its stdout. The worker shares the
output directory, so the parent hashes that directory (outside the private HF
cache and `tmp/`) before dispatch and refuses the run if the worker added,
changed or removed anything other than its `reference.stdout`,
`reference.stderr` and `reference-progress.jsonl`. The `reference.command.json`
and `reference.exit.json` receipts are parent-authored: the parent writes the
command receipt before starting each child and refuses the run if its bytes
differ after the child exits, and writes the exit receipt only after killing the
child's process group. The parent also treats its stdout as untrusted
evidence and refuses the whole run
(status `UNAVAILABLE`, exit 2, no `sessions.json`) unless every returned row
satisfies the worker protocol. A protocol refusal happens before
`realization.json` is written; the raw worker output stays in `reference.stdout`.

- Every admitted input field, including output budget, block size, source
  metadata and JSON value types, equals the admitted row.
- Status is exactly `HF_VERIFIED` or `UNAVAILABLE`. A worker cannot report
  `VERIFIED` or any other status, and cannot return fields outside the protocol,
  such as Go IDs.
- Evidence fields are all present or all absent. Token ID vectors are lists of
  non-boolean, nonnegative int32-range integers; `hf_count` is a non-boolean integer
  equal to the HF vector length; `text_sha256` is the SHA256 of the UTF-8 text.
  Null, float, boolean and truncated values refuse.
- `HF_VERIFIED` additionally requires the HF vector length to equal the target
  and, for hashed rows, intended and HF IDs to be equal. `UNAVAILABLE` requires
  a reason.

Only the parent assigns `VERIFIED`, and only to a row for which it ran its own Go
tokenizer on the returned text. That run must report a non-boolean integer count
and integer IDs, with count, ID length and target equal, IDs exactly equal to the
HF IDs, `framed_sha256` equal to the row's `text_sha256`, the pinned tokenizer
hash, and raw-text scope. Repeated scope/hash blocks are compared by the parent
from the checked HF IDs. Any mismatch leaves that row, and therefore the
population, unavailable.

These checks bind returned evidence to the admitted row and to the parent's own
Go run. They do not attest that the worker executed the pinned generator: a
worker that returns well-formed text with matching HF and Go IDs is accepted,
and the Go tokenizer is the independent check on that text. The intended IDs of
a no-hash row are recorded but not compared.

Only exit status 0 together with the parent's stdout result
`{"status": "SOFTWARE_PASS", "sessions_available": true}` means `sessions.json`
is runnable. Files on disk are diagnostics; any other outcome is unavailable.

Only the parent's final step leaves a `sessions.json` or a `status.json` at
those names. On every unavailable path, and whenever the final `status.json` was
not written, any file or directory at either name is moved to
`unqualified-sessions.json` or `unqualified-status.json`. Because this runs
after the worker's process group is gone, the parent first clears user flags
and restores owner permissions on the output directory and the target, which
undoes a worker's `chmod` or `chflags uchg`. It leaves symlinks and files with
other hard links unchanged, since their inode may be shared with a file outside
the output directory; the link itself is still moved. If the name is taken, a numbered
`unqualified-<name>.<n>` is used; failing that, the target is removed. If even
that fails, the run exits 2 and its stdout reports it unavailable. A
`status.json` already present before the parent's final write also makes the
run unavailable.

The first SIGTERM or SIGINT interrupts realization and kills any owned child.
Later signals, and any signal once the final phase starts (artifact hashing,
the status write and the quarantine), are only recorded, so that phase always
completes. The result is decided once, after the quarantine: if any signal was
recorded by then, any `sessions.json` and `status.json` are quarantined, stdout
reports an unavailable result, and the run exits 2. A signal after that point
arrives after the result is decided and does not change it.

Residual limits: the worker is killed by process group after it exits, and a
descendant that leaves that group can still write later. A stray `sessions.json`
or `status.json` is caught only until the final status is written, and other
late changes are not detected. Root-only flags such as `schg` cannot be cleared,
and a hard-linked or flagged target that cannot be moved or removed stays at its
canonical name; the run then exits 2. SIGKILL can leave files at the canonical
names without a final result. Only exit 0 with the stdout result above is
authoritative. These checks are not a filesystem sandbox.

Admission controls exercise corrupted worker output, worker-written and locked
files, Go evidence, interruption and final identity failures with controlled
subprocess fixtures; those controls do not attest tokenizer behavior.
