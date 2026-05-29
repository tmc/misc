Assume the exacting but fair reviewer role. The API-specification sources are
ground truth. One other source is a `__ROOT__(4)` Plan 9 / Wanix filesystem
design of that API (named at the end) — the design under review. Judge it ONLY
against the spec sources and Plan 9 discipline. Do NOT rewrite it.

NotebookLM ingestion strips fenced code, so the design's `rc` transcripts are
invisible to you — do NOT fault "missing examples" or absent `% ` lines (the
author checks those). Judge only the prose: tree, per-file semantics, verb
tables, feature mapping, boundary.

The verdict is CATEGORICAL, gated on findings, not on a number. Tag each finding:
- **blocker** — a surface (file/verb/option/type/error) NOT in the sources; a
  wrong tree shape; conflated lifecycle objects (mutable builder vs immutable
  graph, value-type vs runtime handle); or a misstated normative behavior.
- **high** — a real source surface MISSED, or a Plan 9 idiom violation (a file
  with two roles, imported foreign vocabulary, language-wrapping).
- **low** — a polish nit (narrow completeness gap, improvable choice, loose cite).

Look at: spec-fidelity (every load-bearing surface traces to a source symbol),
shape-fit (durable handle→clone/$n; one-shot→req/resp; nouns→resource-tree),
completeness (capability/introspection surfaces represented, not dropped),
plan9-idiom (one role per file, namespace-first, bytes-as-files, capability-is-
access).

CITE-OR-RETRACT: every finding quotes the offending design line AND the source
text that settles it; a finding you can't ground in both is noise — drop it. Be
EXHAUSTIVE in the fix-list (list every real defect, lows included) but FAIR in
the verdict: a sound, grounded design PASSES even with lows noted. The 0-10
scores are ADVISORY trend diagnostics — they do NOT decide the verdict.

Emit EXACTLY this block, nothing else:

```
verdict: <PASS | REVISE>
blockers: <count>
highs: <count>
spec-fidelity: <0-10 advisory>
shape-fit: <0-10 advisory>
completeness: <0-10 advisory>
plan9-idiom: <0-10 advisory>
fixes:
- [blocker|high|low] <defect; quote the surface; say what the spec says (cite)>
- ...
```

`verdict: PASS` iff zero blockers (highs/lows may remain — they are the
punch-list). Any blocker → `verdict: REVISE`.
