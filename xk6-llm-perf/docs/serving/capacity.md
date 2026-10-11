# Client capacity qualification

A capacity cell measures the client against a bounded null or fixed-response
HTTP server. It does not establish an inference server's capacity or identify
which component caused latency in a real-model run.

Before running, declare the offered open-load rate, duration, executor and VU
limits, request and response shape, connection limit, resource sampler, and
bounds. Retain these as `parameters` and `preconditions` alongside the exact
client binary, server fixture, host topology, and command receipts.

`llm-serving capacity -input cell.json` checks a JSON object with `evidence`
and `bounds` fields. It rejects contradictory populations and invalid numbers.
A cell that passes checks remains DIAGNOSTIC until bound to an actual measured
probe receipt. Passing checks requires complete offered-arrival accounting, no failures or
dropped arrivals, no requests left in flight, complete sample delivery, and
complete CPU, RSS, connection and dispatch-lag observations within the declared
bounds. CPU percent uses 100 percent per logical core. Connection utilization
must remain below the declared fraction of the configured limit.

The command evaluates submitted observations; it cannot authenticate external
sampler output. Bind the cell to retained raw observations and binary hashes
before using its result as campaign evidence. Missing observations fail
qualification. Do not substitute request latency for parsing or serialization
cost: those optional fields remain unavailable until measured separately.

| Cell | Expected result | Parameters and preconditions | Confounds to retain |
| --- | --- | --- | --- |
| Fixed bounded response, dedicated client, complete resource sampling | DIAGNOSTIC when all bounds pass; measured receipt required | declared rate/duration/VUs, configured connection cap, setup/drain brackets | response size, TLS, connection reuse, host CPU count |
| Full metric channel with cancellation | UNQUALIFIED; cancelled sample count increases | bounded unbuffered collector, explicit context cancellation | collector throughput and cancellation boundary |
| Insufficient allocated VUs / dropped arrivals | UNQUALIFIED | offered rate exceeds preallocated VUs, finite duration | executor configuration and scheduled population |
| Missing CPU, RSS or connection receipt | UNQUALIFIED | one required observation deliberately omitted | sampler scope and missing coverage |
| Actual model server | DIAGNOSTIC | same declared offer, separate inference server | inference scheduling, warmup, model state and network |

The metrics pipeline maintains bounded counters per VU metrics instance.
`PipelineSnapshot` reports sample containers, including extension metrics and
extension-owned builtin HTTP trails, not requests. Both paths stop on cancellation and record
delivery waits. A captured maximum wait must be present; an omitted or null value
cannot substitute for measured zero. No additional metric
sample is emitted to measure the pipeline itself. `ResetPipeline` requires all
producers to have stopped; reset and snapshot explicitly around a declared run,
then aggregate VU receipts by summing counts and waits and taking maximum wait.
Snapshot while producers run is diagnostic, not an exact population receipt.

Sample delivery waits for the collector while the VU is active and exits when
its context is cancelled. The legacy k6 `PushIfNotDone` checks cancellation
before an unconditional blocking send; this extension uses a cancellation-aware
select so a full collector cannot indefinitely prevent VU teardown.

`python3 scripts/capacity-probe.py --k6 /path/to/k6 --output /task/new-cell`
runs a bounded registered-k6 open-load control against an owned fixed-response
fixture. On Linux it retains raw client-PID CPU tick, RSS and socket descriptor
samples, exact wire timestamps, per-VU delivery counters, summary and input
hashes. Resource maxima are sampled peaks, not continuous bounds. The fixture
has a separate PID but shares the host; retain that contention confound.
Parsing and serialization operation times are accumulated from retained
measurements. Parsing excludes socket reads and SSE framing; token counting
and digest calculation are separate costs. These wall-time sums are not CPU
utilization measurements.

`Config.maxConnsPerHost` opts into a connection pool shared across VUs for that
cap. Zero keeps the default unlimited pool; a positive cap excludes
`useK6Transport`. The cap applies per host to the shared pool, not once per VU.
Receipts identify the transport profile, effective configured cap, stream mode,
and encoded request / consumed response body bytes. Response bytes include SSE
framing; they exclude headers and unread bytes. Multiple hosts or pool profiles
require separate capacity accounting.

The bounded probe uses an inclusive-end arrival policy: a constant-rate cell
may schedule rate times duration, or one additional endpoint iteration.
Every scheduled iteration must have one successful accepted response and wire
request. The archived producer hash must be pinned before execution; hashes
identify artifacts and do not authenticate a runtime. The initial geometry is
fixed OpenAI-chat JSON with a one-byte prompt, two-byte response content and
one reported output token. Other geometry cannot inherit this cell.

`EvaluateVerifiedProbe` returns `SAMPLED_CAPACITY` only when a verified measured
cell passes the preregistered bounds and every record binds a positive
`maxConnsPerHost` to the shared HTTP/2-enabled transport profile. The cap is
process-wide for one host, shared across VUs; it is not multiplied by VU count.
Socket-descriptor occupancy conservatively includes unrelated client sockets.
The result qualifies sampled CPU/RSS/socket observations at the declared
cadence, complete offered arrivals, measured parsing/encoding operation sums,
and per-VU sample delivery. It does not bound unsampled resource spikes.
Unknown transport caps or missing observations remain unqualified. A source
or binary pin mismatch rejects the archive.

The complete functional control chains a capacity archive from the same client
binary through four fresh supervised fixed-response server arms and two
independent balanced pairs:

```
python3 scripts/serving-qualification-control.py \
  --k6 /absolute/path/k6 \
  --llm-serving /absolute/path/llm-serving \
  --capacity /absolute/path/capacity-cell \
  --output /absolute/path/new-control-directory
```

The control invokes the real registered extension and `llm-serving paired`.
Each server emits a live observed manifest, receives an actual HTTP warmup,
serves nonvacuous accepted work at a declared open arrival rate, drains, and
exits under lifecycle supervision. `handleSummary` writes the actual retained
measurement run; the driver does not construct accepted run summaries. The
control archives the plan, raw runs, wire request evidence, lifecycle receipts
and paired result. Its 90-second outer deadline supervises an owned process
group. These two pairs demonstrate qualification mechanics, not an inference
server performance result or native engine parity.
