# Native nvtelemetry import contract

The authoritative producer is `github.com/tmc/lib/nvidia/x/nvtelemetry`;
commands remain in `nvidia/x/cmd`. Benchmark translation and efficiency formulas
belong here. No gpufleet extraction or duplicate collector is required.

Baseline native schema 1 was inspected at lib revision
`ee78cfbf2a34a96a7fe8ca403a29940794575ff3`. Raw schema 2 metadata is supported against isolated candidate
`afb8b1976bd958481596d8c7e13f2fe14d618982` (earlier candidate fixtures
are retained). Diagnostic compatibility passes paired portable gates; qualified
benchmark-window joins and native acquisition remain on HOLD. Consumers must reject an
unsupported schema rather than guess meanings from field names.

Native JSONL has one initial manifest, sample and phase envelopes, and one final
manifest. Preserve original bytes and SHA-256. The final manifest supplies
completion/loss accounting; EOF alone does not establish completion. Malformed
or contradictory streams are distinct from parseable incomplete captures.
Retain incomplete captures for diagnostics and refuse complete-window efficiency.

Native timestamps are signed int64 nanoseconds. Sample query start/end and phase
intervals share process-relative monotonic time; they are not capture-relative
or Unix times. Readings may have independent source timestamps. Conversion must
retain raw nanoseconds and use declared domains and mapping provenance.

Host query bounds do not bound sensor observation: cached measurements may
predate the API call. Omitted sensor timestamps/integration intervals stay
unknown. Legacy omitted or explicitly serialized zero uncertainty does not
establish known-zero uncertainty. Manifest start/end clock pairs alone do not
establish remote synchronization or absence of wall-clock steps.

Legacy MIGEnabled=false cannot distinguish disabled mode, unsupported queries
or unknown scope. Retain the raw field without asserting non-MIG identity.
Missing model/driver/backend data are diagnostic gaps; a missing stable UUID
prevents entity joining. Advertised fields are not proof of measured support.

Use IsInt and IntValue for native integer readings, including omitted integer
zero and counters above 2^53. Ignore rounded floating copies for counter math.
Preserve available/unavailable transitions and reason codes as coverage gaps.
Percent-to-ratio, mW-to-W and MHz-to-Hz translations are explicit and versioned.
Memory utilization is separate from DRAM activity. Bitmasks and event codes are
raw evidence and cannot enter numeric means or integration.

A query-timed estimate needs an explicit policy, admitted clock mapping and
coverage. It must never be labeled sensor-exact. Cumulative energy with wider
counter boundaries retains its actual interval/offsets; no interpolation or
exact requested-window claim is allowed. Whole-device energy does not establish
MIG, process or request attribution.

The cross-repository gate retains canonical positive v1/v2 fixtures separately
from negative fixtures. Both sides must agree the same original hash and each
admission/refusal. Cases include unknown sensor time with known query bounds,
legacy uncertainty, synthetic known-zero uncertainty, mapping expiry/wall steps,
integer zero and large counters, availability transitions, mixed/shared-query
fields, MIG states, missing UUID, resets, wider boundaries, bitmasks/events,
truncation, loss and contradictory final manifests. Native hardware and matched
overhead qualification remain separate from portable fixture acceptance.

## Candidate v2 diagnostic compatibility

The reader and CLI preserve additive source/query flags, domains, epoch identity,
calibration scalars, build labels, discrete fields, MIG scope and independent
GI/CI presence. They reject unsupported versions, changed final identity/version
or clock metadata, negative uncertainty and invalid per-reading query intervals.
Declared calibration and provenance are retained claims, not certified mappings.
Known-zero scalar presence is retained without upgrading its evidence.

```
go run ./cmd/llm-telemetry -input native-v2.jsonl -input-format nvtelemetry-v2
```

Canonical v1/v2 fixtures and the raw-valid bitmask fixture are accepted for
diagnostics with identical producer hashes. Truncation, contradictory manifests
and missing UUID fixtures are refused. A mock CLI export from an immutable
archive of candidate a401 is also imported with identical bytes/hash. No numeric
aggregation or exact benchmark-window join is admitted for this candidate.

Candidate 74eb3 adds actual per-query bounds, discovery capability states, typed
metadata failures, better MIG classification and a RealClock process epoch. The
consumer preserves those additions plus mapping-valid-from and build-modified
claims. Remaining producer checks include query containment, all immutable
terminal metadata and independent FakeClock epochs. Clock mappings still lack
paired observation evidence. The positive canonical sensor clock
is not the producer process epoch; its power readings have unknown sensor timing.
These fixtures do not qualify native acquisition, overhead or full AIPerf parity.

The producer-provided remediation export and a separately executed immutable
74eb3 CLI mock export both pass native replay and consumer inspection with
identical original hashes. Host producer/consumer race and vet are separate
receipts from physical GPU/MIG, reference or overhead qualification.

Candidate b9607c88d adds containment and terminal metadata checks. Its mock
export and two additional negative fixtures are retained with pinned raw hashes.
Independent frozen-source controls still find a zero-start containment bypass
and an accepted terminal binary-digest contradiction. FakeClock epoch labels
also need cross-process and zero-value isolation. Consumer checks reject the
stream contradictions independently; producer adoption remains on HOLD.

Candidate afb8b1976 corrects those three reported producer invariants. A fresh
CLI mock export generated from its immutable source archive passes paired
inspection with identical original hash; canonical and contradiction fixtures
pass the same diagnostic gate. This qualifies raw v2 diagnostic compatibility
for the tested producer and consumer revisions. It does not qualify sensor
clock mappings, physical acquisition, expanded metric coverage or observer cost.

## Managed and DCGM lineage diagnostics

The additive `reading.source` envelope from producer revision
`404d70ca1edf44f37317604acf651cce5027c2b7` is retained in schema2 captures.
The importer validates supported source schema, bounded identities and labels,
positive mapping scales, the original mapping digest and exact decimal
publication timestamp conversion. Raw NaN, +Inf and -Inf are retained only for
`unavailable` readings with reason `non_finite` and no canonical measurement.
Measured or derived non-finite lineage and contradictory values are rejected. Unknown fields, duplicate keys,
explicit null evidence and inconsistent mapping names are rejected. Native DCGM
metadata remains diagnostic evidence. `managed_lineage_unqualified` explicitly
marks these imports. Publication timestamps never establish sensor acquisition.

This compatibility does not certify raw-value conversion, native DCGM status or
entity scope, producer authenticity, hardware support, or sensor timing. Window
admission leaves every field with managed/DCGM lineage unavailable, including
energy, until those independent predicates have a qualified contract. Other
qualified fields remain independently usable. Raw source bytes and integer
readings remain available for diagnostic inspection.
