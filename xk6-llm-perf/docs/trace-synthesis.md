# Offline empirical-prefix synthesis

`scripts/aiperf-synthesize.py` invokes the actual AIPerf `Synthesizer`, `RadixTree`
and optional `RollingHasher` from pinned revision
`f10eb50a1776775d80d7f13396b0f668f14e2c03`. It generates trace count/cache targets,
not realized text prompts, tokenizer IDs or observed serving-cache hits. It never
runs a model, downloads data or reconstructs the algorithm from a graph sketch.

Use an existing qualified AIPerf Python environment:

```sh
/path/to/pinned-venv/bin/python scripts/aiperf-synthesize.py input.json \
  --output outputs/synthesis
```

The output directory must not exist. Input has exactly these envelope fields:

```json
{"schema_version":1,"seed":7,"grouped":false,"params":{"block_size":4,"prefix_len_multiplier":1.5,"prompt_len_multiplier":0.5,"prefix_root_multiplier":2},"traces":[{"timestamp":15.5,"input_length":12,"output_length":5,"hash_ids":[1,2,3]},{"timestamp":30,"input_length":12,"output_length":7,"hash_ids":[1,2,4]}]}
```

`params` accepts only actual pinned `SynthesisParams` fields. Omitted values are
filled by that class before budgeting. The adapter verifies the full 1025-file
Python source hash, plugin-default hash and AIPerf package version before
importing synthesis code. It calls actual `rng.reset()` and `rng.init(seed)`;
`Synthesizer` derives its RNG under `dataset.synthesis.synthesizer`. Receipts pin
the source, Python and complete package versions, effective params, explicit seed
and derivation label.

With `grouped:true`, traces is an ordered object mapping session IDs to row
lists. The actual grouped synthesizer flattens rows with authoritative group
`session_id`, invokes synthesis, then removes that field while regrouping.
Embedded row session IDs are therefore overwritten/removed upstream. Empty
sessions and group insertion order survive in `traces.json`. `traces.jsonl`
flattens the result with group keys attached as `session_id`; empty groups have
no JSONL row, a declared export transformation. Row order remains unchanged.

Outputs are `traces.json`, `traces.jsonl`, and `receipt.json`. The JSON/JSONL trace
files contain upstream Mooncake field names and preserve arbitrary pass-through
fields except those intentionally modified by upstream. Hashes identify complete
input bytes, canonical input/params, adapter, reference source and actual output
bytes. Canonical hashes sort object keys; emitted grouped JSON preserves session
insertion order. Numeric hash IDs are never converted through floating point.
JSONL can be supplied to a compatible Mooncake trace importer; actual importer
admission remains a separate check, especially for arbitrary text/messages/payload
rows outside an importer's count-target subset.

The upstream transformation retains its exact behavior:

- Shared IDs are those appearing more than once; prefix scanning stops at the
  first unshared ID. Prefix length stretch/squeeze, unique prompt blocks, root
  redistribution and optional rolling renormalization use the pinned code.
- Unique prompt tokens truncate with `int`, prefix blocks use upstream
  ceiling/slicing, and scaled output lengths use Python round-to-even then clamp
  to at least one. A positive speedup ratio converts timestamps with `int`;
  zero leaves timestamps unchanged.
- `max_isl` caps the reported input length after hash allocation; it does not
  truncate the generated hash vector. `max_osl` caps rounded output length.
  Text/messages/payload inputs do not acquire a synthetic `input_length`.

The adapter is deliberately bounded more narrowly than upstream: 8 MiB input,
16 MiB per output, nesting64, 1000 rows/groups, 1024 input hash IDs per row and
50000 total input/projected output blocks. Counts are integers in0..1000000;
hash IDs are nonnegative exact JavaScript-range integers. Input length must cover
all declared hash blocks. Multipliers are finite0..16, root width1..64 and block
size1..4096. Positive speedup must be at least1/16; original and projected
timestamps stay within one day. Before upstream path allocation, a conservative
bound assumes both all blocks shared and all input tokens needing unique prompt
blocks. Projected token lengths stay within1000000 and projected hash IDs within
the exact integer range. Caps cannot waive preallocation bounds. Excessive
amplification fails rather than truncating rows or silently applying new caps.

Actual synthesis runs in a subprocess with CPU20 seconds, wall30 seconds,
address space2 GiB and file size32 MiB, plus one BLAS/OpenMP thread. Worker
admission is bounded before algorithm allocations. Missing runtime/source pins,
invalid input or exhausted budgets fail without a successful output directory.
These are resource limits, not a general sandbox or a generated-code grader.

Reference controls exercise actual flat/grouped functions, shared prefixes,
fractional/zero multipliers, root randomness, rolling hashes, clamp/rounding,
empty sessions, pass-through fields, repeated seeds and abuse refusals. They
establish software transformation behavior only; real traces, realized prompt
counts, tokenizer assets and cache performance require separate evidence.
