# Summarize emitted accuracy grades

Use an existing AIPerf `accuracy_export.jsonl` file to summarize emitted grades
without running generation or a grader:

```sh
llm-accuracy -graded-records accuracy_export.jsonl -accuracy-phase profiling
```

The phase must be explicitly `warmup`, `profiling` or `all`. Each input record
must also carry its own known `benchmark_phase`; missing phase metadata is an
error. The pinned upstream JSONL writer exports profiling records only. Selecting
`warmup` from that file therefore returns `summary: null` unless it contains
explicit warmup records collected through another retained graded-record path.

The importer follows AIPerf
`f10eb50a1776775d80d7f13396b0f668f14e2c03`'s `AccuracyRecordsData` fields and
`AccuracyAccumulator._build_summary` population. The denominator is the number
of emitted grades in the selected phase. It counts each response independently:
several responses to one problem remain several samples. An emitted incorrect
grade contributes one evaluated response and zero passed responses. An emitted
grade marked `unparsed` still contributes to both the denominator and the
unparsed count; it can also be correct. Records with no task label contribute
to the overall summary but not per-task summaries.

A failed request can still produce a grade upstream. Such a grade belongs in
this summary. The graded stream alone does not identify request failures or
requests for which no grade was emitted. The report preserves that uncertainty
with null `failed_requests` and `unavailable_requests` plus an explicit
`request_population_status`. It does not manufacture zero counts for those
populations or imply all-attempt accuracy.

This mode is separate from the default artifact evaluator. The default still
divides correct responses by all declared cases and reports failures,
unavailable predictions and coverage from that artifact. Graded-record mode
cannot be combined with artifact, generation or worker flags. A `-` input path
reads graded JSONL from stdin.

For Go consumers, `accuracy.SummarizeGraded(reader, phase)` returns a
`GradedReport`. It retains the source SHA-256, all record receipts with their
selection status, selected/excluded counts, grader labels and an optional phase
summary. The summary is null for an empty selected population. The summary's
`grader_name` follows upstream's first selected record; `grader_names` lists all
selected labels, so a mixed-grader stream is not presented as uniform.

The report omits answer text. Supplied nonempty `x_request_id` values identify
samples and must be unique, including across phases. Repeated `conversation_id`
values are valid. When a request ID is absent or empty, the receipt uses the
original source hash and physical line ordinal; it does not deduplicate by
problem. Keep the original graded stream alongside its hash and the request
ledger when request-level completeness matters. A hash identifies retained
bytes; it does not authenticate their producer or certify grading quality.

Imports are limited to 16 MiB, 100000 records, 1 MiB per line, 256-byte identity
fields and 64 KiB per answer field. Unknown fields, duplicate or case-aliased
members, duplicate supplied sample identities, missing/null/unknown phases,
missing or non-boolean scores and invalid numeric evidence refuse the complete
import. The parser accepts the pinned serialized fields, not arbitrary future
AIPerf schema additions. Missing optional `unparsed` retains the pinned false
default; a missing `passed` never becomes false.
