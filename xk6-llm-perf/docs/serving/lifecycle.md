# Supervised serving lifecycle

`go run ./cmd/llm-serving lifecycle -config lifecycle.json` runs one fresh
local server process and one workload command, using literal argv. It writes a
JSON receipt even when a phase fails and exits nonzero on failure. `capacity`
evaluates separately supplied client-capacity evidence as diagnostic data. Sequential paired arms
must each invoke a new lifecycle, rather than reuse a warmed server.

The lifecycle requires absolute executable paths and their SHA-256 digests,
explicit deadlines, a loopback HTTP origin with an explicit unoccupied port,
readiness, an observation endpoint, at least one excluded warmup, and drain.
Every probe shares the server origin. Redirects are refused. HTTP request and
response bodies are bounded to 1 MiB. Server and workload combined stdout/stderr
are capped at 64 KiB each for hashing; plaintext output and environment values
are not recorded. Truncation is explicit. Do not put secrets in the observed
manifest: that manifest is retained in the receipt.

`observe` must return a `ServingManifest` with effective settings and evidence
digests. Its exact response bytes must match `expected_observation_sha256`.
The observed binary digest must match the launched executable, its source
revision and backend must be present, and opaque endpoint identity must match `expected_endpoint_identity`. `ServingWorkHash` must accept the observed manifest. A configured
value, a hash, and an HTTP response do not independently authenticate a server
or its settings; server-side probes and evidence still require review.

| Scenario | Control and retained result |
| --- | --- |
| Startup fails or readiness never arrives | Bounded startup deadline; failed receipt; owned group stopped. |
| Effective configuration differs | Observation hash, server identity, and complete-work validation reject before warmup. |
| Warmup fails | No measured workload; failed warmup phase retained. |
| Server exits during workload | Workload stopped; failure preserved. |
| Workload fails or exceeds its deadline | Failure preserved; owned workload and server groups stopped. |
| Cancellation | Context cancels probes/workload; independent bounded cleanup still runs. |
| Drain fails | Failed receipt even if workload succeeded; server still stopped. |
| Descendant ignores termination | SIGTERM followed by SIGKILL to the owned process group; leader reaped. |
| Port already has a server | Refused before launch; existing process untouched. |
| Required GPU lock absent | Refused before launch. |

Timeouts are `startup_ms` (at most one hour per startup/probe phase),
`workload_ms` (at most one day), and `stop_ms` (at most one minute per termination
stage). Process-group supervision is supported on Darwin and Linux. Children
that deliberately escape their process group require an external cgroup or
container supervisor; group cleanup is not a claim that escaped descendants
were reaped. The receipt proves host lifecycle execution, not GPU exclusivity,
physical sensor correctness, equivalent engine work, or native parity.

An optional `lock` is a separately hash-pinned executable with
`lock_protocol: "append-command-v1"`. Its arguments are followed by the server
executable and argv; the wrapper must acquire ownership before launching and
retain it until the server exits. `require_lock: true` refuses runs without that
wrapper. The harness verifies wrapper bytes and records the declared protocol;
it cannot prove the external lock implementation actually excludes foreign
work. A wrapper that exits early fails the lifecycle.

The local subprocess tests exercise actual HTTP startup, observation, warmup,
workload, drain and teardown, plus pinned-byte mismatch, observation mismatch,
early exit, cancellation, missing lock, failed warmup and a stubborn descendant:

```sh
go test -race ./serving -run Lifecycle
```

Keep machine resource topology, lock evidence, model/cache state and server-side
observation trust in the [confound ledger](CONFOUNDS.md).

The entire run has an additional 24-hour context ceiling, including startup,
warmup and drain; cleanup uses its independent stop deadline. Resource settings
are observed and bound, not enforced through this API. Use an external container
or cgroup for CPU, memory and PID limits. Keep executable paths immutable: digest
checks bind the checked bytes but cannot eliminate filesystem mutation between
verification and OS execution. Likewise the prelaunch free-port check narrows,
but cannot eliminate, a competing bind race on an uncontrolled host.

Receipts record a random run nonce, actual launcher PID and process-group ID,
workload PID and group, launch timestamps, and explicit startup phases. Without
a lock wrapper the launcher is the server process. With a wrapper, `server_pid`
is absent: the wrapper's child PID is not inferred or fabricated. PID plus nonce
and timestamp identifies this launch; PID alone is reusable and cannot identify
a process indefinitely. Pairing must retain these observed launch identities.
