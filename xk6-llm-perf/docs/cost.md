# Declared token cost

`llm-output --input run.json --format cost --prices prices.json` derives a
post-run ledger from validated retained measurements. It does not contact a
pricing service or report a provider invoice. No prices are bundled.

```json
{"currency":"USD","source_url":"https://example.com/prices","effective_date":"2026-10-11","models":{"my-model":{"input":2,"cache_read":1,"cache_write":3,"output":4}}}
```

Rates are currency units per million tokens, keyed by exact model names.
A record is priced by the response model the provider reported
(`measurement.work.observed_model`), which is retained when the request carries
a [`work`](work-acceptance.md) policy; `work: {}` retains it without checks.
When that model differs from the requested one and the table has no entry for
it, the record is unavailable with `observed model price missing`; the
requested model's rate is never borrowed for an alias, route or version suffix.
A reported model too long to retain (`observed_model_oversized`) is unavailable
with `observed model not retained`.
Without a retained response model, including older receipts and requests without
`work`, the requested model (`Measurement.Model`) is used. Each eligible ledger
record names the key in `price_model` and its basis, `observed` or `requested`,
in `price_basis`; both are recomputed from the run on every derivation. Ledger
`schema_version` 2 marks this observed-model keying; version 1 ledgers keyed every
record by the requested model. The table
is strict JSON, at most 1 MiB; unknown fields, duplicate members, null rates,
negative rates and nonfinite numbers are rejected. Currency is three uppercase
letters, not a checked currency registry. Source URL and effective date are
caller declarations. The ledger retains these and the SHA-256 of the exact file.

Only successful measurement-phase records with directly reported input and
output counts are eligible. Failures and warmup records receive exclusion
reasons and no amount. Missing counts or prices produce null amounts with
reasons; zero tokens need no rate. A zero rate explicitly declares a free bucket.
A missing model remains unavailable, even for zero tokens.

Input counts include cache reads and writes. Anthropic requires an explicit cache-write count even for zero input,
which establishes the inclusive input convention. Positive input also needs
an explicit cache-read count. Providerwire-v4 requires both cache counts for
positive input; its explicit zero total requires no rates. Old Anthropic receipts
without a cache-write count cannot establish normalized inclusive input and
are unavailable. OpenAI chat, completions and Responses use an explicit cache
read count; cache writes are not applicable, and a cache-write rate is rejected.
An absent cache read is not treated as zero. Other protocol conventions are
unavailable until mapped and validated.

Reasoning tokens are a subset of output. Without a reasoning rate, output
prices apply to the whole completion. With a reasoning rate, explicit reasoning
counts split completion into ordinary and reasoning output, preventing double
billing. Contradictory cache or reasoning counts and inconsistent provider
usage are errors. When present, provider total must equal reported input plus
output. Client
estimator disagreement does not invalidate consistent reported billing counts.
Estimated or derived counts are unavailable.

`known_subtotal` sums available eligible records. `total` is null if any eligible
record is unavailable or records were dropped. The ledger `unavailable` field
names every cause that applies (`incomplete eligible usage or prices`,
`dropped records`, or both separated by `; `); each record keeps its own
reason. This is a retained-run estimate;
boundary exclusions, in-flight requests, failed requests and provider billing
rules can create additional charges outside this ledger. Cost output is JSON
only and cannot be streamed or posted to a sink.

Anthropic input normalization adds `input_tokens`,
`cache_read_input_tokens` and `cache_creation_input_tokens` when reported.
Absent cache buckets remain null in measurements, including older schema-v1
receipts; pricing never upgrades those old receipts by guessing counts.
See the [Anthropic prompt caching usage contract](https://platform.claude.com/docs/en/build-with-claude/prompt-caching).

Amounts use floating-point arithmetic and are not rounded to invoice currency
precision. Compare derived estimates with a numerical tolerance rather than
expecting decimal invoice arithmetic.
