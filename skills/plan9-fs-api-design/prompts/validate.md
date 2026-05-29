Assume the exacting but fair reviewer role. The API-specification sources are
the ground truth for the API. One other source is a `__ROOT__(4)` Plan 9 / Wanix
filesystem design of that API (named at the end of this prompt) — that is the
design under review. Judge it ONLY against the specification sources and against
Plan 9 design discipline. Do NOT rewrite the design — return a verdict and a
fix-list.

NotebookLM ingestion strips fenced/preformatted code from sources, so the
design's `rc` worked-example transcripts are NOT visible to you. That is an
ingestion artifact, not a design defect: do NOT fault the design for "missing
examples" or absent `% ` command lines, and do NOT score any dimension on the
examples. Example-coherence is checked separately by the author. Judge only the
prose: the tree, per-file semantics, verb tables, feature mapping, and
boundary.

Score each dimension 0-10, citing a source where the design makes a factual claim:

1. spec-fidelity — does every load-bearing file, ctl verb, option, type, and
   error trace to a real source symbol? Flag any surface NOT in the sources
   (hallucination) and any source surface MISSED.
2. shape-fit — is the tree shape right for the API (durable handle → clone/$n;
   one-shot → req/resp; addressable nouns → resource-tree)? Are distinct
   lifecycle objects (e.g. a mutable builder vs an immutable compiled artifact,
   a value-type vs a runtime handle) kept as DISTINCT nodes, not conflated?
3. completeness — are capability/introspection surfaces (limits, supported-ops,
   feature queries) represented rather than silently dropped or fails-closed?
4. plan9-idiom — one role per file (ctl/data/stream/status/event/req/resp not
   overloaded); namespace-first not language-wrapping; bytes cross as files;
   capability-is-access; no imported vocabulary from a different kind of API.

CITE-OR-RETRACT: before scoring any dimension below __PASS_THRESHOLD__, quote
the exact offending line from the design and the source text that contradicts
it. A complaint you cannot ground in both is reviewer noise — drop it and do not
let it lower the score.

Then emit EXACTLY this block, nothing else (no prose, no rewritten manpage):

```
verdict: <PASS | REVISE>
score: <overall 0-10; equals the lowest dimension score>
spec-fidelity: <0-10>
shape-fit: <0-10>
completeness: <0-10>
plan9-idiom: <0-10>
fixes:
- <one concrete, source-cited defect to fix; quote the offending surface; say what the spec actually says>
- <next fix>
- ...
```

Emit `verdict: PASS` only if every dimension is >= __PASS_THRESHOLD__ and there
are no hallucinated surfaces. Otherwise `REVISE`, and make each fix specific and
actionable (a reviewer's punch-list, not a grade). Cite the source for every
factual fix.
