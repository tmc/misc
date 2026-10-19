# Retained missing-usage cells

`scripts/aiperf-missing-usage-verify.py ARCHIVE --plan POLICY --plan-sha256 SHA256` checks a separate C1 streaming
OpenAI chat population. It does not change the full-usage reasoning verifier's
population or tolerances. Historical failed cells, including Q2, retain their
original verdicts. A new run needs a new identity and frozen source/binary plan.

Each arm has two excluded sequential warmups and four measured requests, with
identical forwarded JSON bodies containing `stream: true`. Both tools must use
an actually supported matched request configuration. Inspect AIPerf's automatic
`stream_options.include_usage` insertion in the capture; merely omitting that
option in an authored input does not establish matched requests. The verifier
refuses different forwarded JSON bodies. Any non-null usage object, including
reported zero, partial usage, or an empty object, makes this missing-usage cell
unavailable. Accounting checks still run independently; honest reported usage
can produce `status: SOFTWARE_PASS` with `missingusage: UNAVAILABLE`. Never remove provider usage to obtain a passing population.

The archive contains `wire.jsonl`, `xk6/run.json`,
`aiperf/profile_export.jsonl`, and `aiperf/profile_export_aiperf.json`.
The observer records each request body's exact bytes as `request_body_base64`,
status, header pairs, monotonic start/end nanoseconds, Unix `start_wall_ns`, and
ordered timestamped chunks as `{ns, base64}`. Transfer framing is removed by the
HTTP library; response entity bytes are retained unchanged. Header secrets are
redacted. Use only the synthetic authored nonsensitive prompts in the approved
plan: body bytes are deliberately retained.

`scripts/aiperf-live-endpoints.py` observes one tool at a time on loopback and
forwards to an existing HTTP origin with a literal IPv4/IPv6 address.
Hostnames, including localhost, are refused. Connect uses one numeric socket
address and the remaining accepted-request deadline, without DNS or address
fallback; TLS handshake also receives the remaining timeout. It sends no requests by itself. It accepts
at most six requests and runs at most five minutes, with a 30-second absolute accepted-request
deadline (including request line/headers), capped by the remaining cell time,
and a 4 MiB request/response entity bound. A deadline timer shuts down accepted
sockets before parsing and upstream sockets during response headers/body;
downstream writes share the same deadline. Slow byte trickles cannot reset it. It never adds usage options or
retries. Its arguments include `--upstream`, `--output` (must not exist),
`--arm xk6|aiperf`, `--port`, `--lease-receipt` and `--lease-sha256`.
The hashed lease receipt requires nonempty `source_revision`, `binary_sha256`,
`aiperf_revision`, `runtime_identity`, `model_digest`, `plan_sha256`, and
`coordinator_lease` strings. This checks retained identity, not authorization:
the coordinator must separately grant the shared DGX lease before either tool
is dispatched. Retain the hashed plan, actual binary versions, model digest,
runtime identity, scripts, native exports, and observer log for that exact run.

The verifier checks complete populations, phases, C1 within each arm,
record-to-wire starts within 15 ms, unavailable xk6 input/output counts and
throughputs independently, and absence of reported AIPerf usage metrics.
AIPerf's tokenizer estimates are a separate population; these checks do not
validate their values. `SOFTWARE_PASS` means retained consistency checks passed,
not independent proof of a live producer. Run replay, slice/export consumers,
cost direction availability, partial-usage direction cells, cancellation,
provider errors and other endpoint families need separate controls. There is
no statistical performance claim.

The verification policy is a JSON object with nonempty `source_revision`,
`binary_sha256`, `aiperf_revision`, `model_digest`, `runtime_identity`,
`payload_sha256`, and `preregistration_sha256` strings. Payload identity is the
SHA256 of UTF-8 JSON serialized with sorted keys and compact separators.
Preregistration identity binds `ARCHIVE/PREREGISTRATION.md`. These hashes check
retained identity; they do not prove when a policy was authored. Keep a policy
written after execution labeled as a verification mapping to the original plan.
The two-warmup/four-measured C1 population is fixed in the verifier.
