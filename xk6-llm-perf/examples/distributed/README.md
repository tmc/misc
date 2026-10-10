# Distributed workers

`worker.js` runs the eight portable rows once across k6 execution segments.
The process worker identity prefixes every retained request ID. All workers
use an explicit common Unix millisecond window, run ID, seed, workload and
SHA-256 of `worker.js`, one NUL byte, then `rows.json`. A mismatched asset hash
fails before requests. Each worker exports a complete versioned run.

Build the extension using the repository Makefile. From this directory,
compute the hash and a sufficiently future start time, then launch both
workers concurrently:

```sh
HASH=$(python3 -c 'import hashlib; print("sha256:"+hashlib.sha256(open("worker.js","rb").read()+b"\0"+open("rows.json","rb").read()).hexdigest())')
START=$(python3 -c 'import time; print(int(time.time()*1000)+10000)')
export TRACE_HASH="$HASH" WINDOW_START_MS="$START" WINDOW_DURATION_MS=3000
export ENDPOINT_URL=http://localhost:8000 RUN_ID=local-example
mkdir -p results
WORKER_ID=a RUN_FILE=results/a.json ../../dist/k6 run --execution-segment 0:1/2 --execution-segment-sequence 0,1/2,1 worker.js &
A=$!
WORKER_ID=b RUN_FILE=results/b.json ../../dist/k6 run --execution-segment 1/2:1 --execution-segment-sequence 0,1/2,1 worker.js &
B=$!
wait "$A"; wait "$B"
```

Shell exit status and request failure accounting are separate receipts. Save
both statuses. Do not infer complete participation from files merely existing:
use a new result directory and expected worker identities for every run.

Construct a portable JSON manifest next to the worker files:

```json
{
  "schema_version": 1,
  "expected_workers": ["a", "b"],
  "max_clock_uncertainty_ms": 0,
  "workers": [
    {"file":"a.json", "sha256":"REPLACE_WITH_FILE_SHA256", "clock":{"offset_ms":0,"uncertainty_ms":0,"basis":"REPLACE_WITH_CLOCK_RECEIPT"}},
    {"file":"b.json", "sha256":"REPLACE_WITH_FILE_SHA256", "clock":{"offset_ms":0,"uncertainty_ms":0,"basis":"REPLACE_WITH_CLOCK_RECEIPT"}}
  ]
}
```

Run `go run ../../cmd/llm-merge -manifest results/workers.json > results/merged.json`.
Paths must stay inside the manifest directory, including after symlink
resolution. SHA-256 is checked before parsing. Clocks must explicitly include
both offset and uncertainty in milliseconds; the declared uncertainty must
meet the manifest bound. Zero is a fixture input, not a default proof of
synchronized machines. Obtain a clock receipt that covers the actual window;
a fixed offset cannot repair clock steps.

Missing workers fail by default. To retain a partial run, remove absent files
from `workers`, keep their identities in `expected_workers`, and explicitly
pass `-allow-partial`. The aggregate retains `missing_workers`; it does not
scale observed counts to simulate missing traffic. Failed requests remain in
accounting. Repeated IDs, dropped records, different windows/assets/runtime
versions and warmup policies fail merging.

## Kubernetes operator

The [k6 Operator configuration contract](https://grafana.com/docs/k6/latest/set-up/set-up-distributed-k6/usage/configure-testrun-crd/)
assigns equal execution segments to `parallelism` runners. The
[extensions guide](https://grafana.com/docs/k6/latest/set-up/set-up-distributed-k6/usage/extensions/)
requires a custom image containing the extension. The
[troubleshooting guide](https://grafana.com/docs/k6/latest/set-up/set-up-distributed-k6/troubleshooting/)
requires all script assets to be accessible to the runners. These examples use
a ConfigMap and a shared results PVC. The upstream AIPerf lifecycle reference
at `docs/kubernetes/workflow.md` separates validation, preflight, execution,
result retrieval and deletion; apply the same separation here.

1. Freeze the source and run `make build-linux` from the repository root to
   create `dist/k6-linux` for the cluster architecture (set `LINUX_ARCH` when
   needed). Then build `examples/distributed/Dockerfile` from that same root. Record source,
   binary, base image and resulting image digests. A macOS binary will not run
   in this image. Make the pinned image available to your cluster.
2. Confirm the installed TestRun CRD/operator version, custom image inspection,
   endpoint access, capacity, clock receipts and a compatible ReadWriteMany
   storage class. If shared storage is unavailable, configure another durable
   per-worker artifact sink before execution.
3. Render every uppercase placeholder in `testrun.yaml`. Set a common epoch far
   enough ahead for all runners to initialize; include measured clock bounds
   in the later merge manifest. A late worker has missed the admitted window;
   do not silently shift only its window.
4. Create assets with `kubectl create configmap llm-perf-assets --from-file=worker.js --from-file=rows.json --dry-run=client -o yaml`.
   Validate the rendered resources with `kubectl apply --dry-run=server`.
5. Apply assets, `results-pvc.yaml`, then the rendered TestRun. Watch its status
   and runner Jobs; retain pod identities, exit statuses, events and logs.
   `POD_NAME` supplies each worker identity via the downward API.
6. Retrieve every worker JSON from the PVC before teardown. Hash files and build
   the expected-worker manifest from the actual runner identities. Merge only
   after the clock, participation and accounting gates pass.
7. To cancel, delete the TestRun and verify its runner Jobs and pods terminated.
   Cancellation before `handleSummary` can leave no worker file: record that
   worker as missing, rather than importing a stale receipt. After retaining
   results, remove the ConfigMap and delete the PVC only if its artifacts are
   no longer needed. Verify no remaining runner pods.

No cluster was deployed by these examples' host tests. Operator/CRD validation,
image execution, storage, real clock bounds, startup, cancellation and teardown
on a real cluster are separate open qualification gates. The host integration
test runs two actual local k6 processes and verifies unique partitioned rows,
identities, shared window and reconstructed counts.
