# Native telemetry reference checks

`reference.py` records bounded, timed `nvidia-smi` queries and compares a native
capture to the nearest same-device query. It uses Python's standard library.
Run inside an owned GPU-lock lane alongside the producer:

```sh
python3 reference.py record --output reference.jsonl --duration 10 &
reference_pid=$!
nvtelemetry record -output native.jsonl -duration 10s -interval 100ms
wait "$reference_pid"
python3 reference.py compare --reference reference.jsonl --native native.jsonl > comparison.json
```

The output distinguishes PASS, GAP, UNAVAILABLE, and UNTESTED for each reading.
A complete, lossless native final manifest is required. A GAP exits 1; successful
analysis does not imply all metrics passed. Unavailable and untested readings
remain explicit. Run the producer's strict replay validator separately.

Predeclared maximum timestamp pairing skew is 250 ms. Diagnostic absolute
unit tolerances are 15 percentage points for utilization, 10,000 milliwatts for
power, 3 Celsius degrees for temperature, and 300 MHz for clocks. Do not tune
these tolerances after observing a run; changes require a new declared campaign.
Readings outside the pairing interval are untested rather than silently dropped.

These are asynchronous API consistency checks. Both tools use NVML, so
`nvidia-smi` is a separate executable, **not an independent physical sensor**.
The checks do not qualify sensor-exact energy, MIG attribution, or schema 2
clock mappings. Unsupported GB10 memory fields and absent MIG require capable
hardware; an external calibrated reference is required for physical accuracy.
Native adapter admission requires its own verified schema 2 provenance and clock
mapping. This script cannot manufacture those prerequisites from schema 1.

Observer impact qualification additionally needs balanced enabled/disabled
blocks, identical payload/model/warmup/concurrency, clean compute admission,
per-request wire timing, and collector CPU/RSS/query/cadence/loss receipts.
Desktop graphics and intercell residency gaps must be retained as confounds.

```sh
python3 -m unittest discover -s examples/qualification -p 'test_*.py'
```
