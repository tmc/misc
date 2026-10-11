# Serving comparison confounds

A serving comparison measures the complete HTTP service and client workload.
It does not establish native engine parity or isolate GPU kernel execution.

Record these controls before collecting a comparison:

| Control | Required evidence | Failure handling |
| --- | --- | --- |
| Model, tokenizer, template, workload | Content digests and observed server manifest | Reject equivalence when unknown or different |
| Sampling, output bounds, cache, compile, scheduler, resources | Configured and effective values with evidence | Preserve unknown values; do not substitute defaults |
| Server process | Fresh process, executable digest, readiness and teardown receipt | Retain failed arms; no automatic retries |
| Work performed | Expected model, finish reason, exact usage, output bounds and declared digest checks | Keep rejection distinct from protocol success |
| Client capacity | Measured offered arrivals, drops, lag, CPU, memory, connections and sample delivery | Keep missing observations unqualified |
| Statistical unit | Independent sequential process pair | Never treat requests within a process as independent runs |
| Order and inference | Preregistered balanced order, seed, primary metric and family | Report delta/ratio interval status and simultaneous bounds |

Keep all arms, including failures. Do not discard slow runs, retry a failed arm,
change the metric family after observing results, or pool request latencies to
replace independent process pairs. A zero baseline makes a ratio unavailable;
a finite paired delta can still be reported.

Manifest digests establish the identity of declarations and captured evidence.
They do not authenticate an untrusted server or prove that reported settings
were applied internally. A client capacity probe qualifies only its measured
client, transport, workload and resource window. Local fake-server tests prove
software behavior; they do not qualify a real model, GPU or production service.
