# BurstGPT workload targets

`traceworkload.ReadBurstGPT` imports BurstGPT CSV with `Timestamp`,
`Request tokens` and `Response tokens` columns. Additional columns affect the
source hash but do not supply request fields. The importer preserves record
order and seconds since the original trace origin, converting seconds to
milliseconds without rebasing. Equal timestamps are valid.

This contract follows the field and unit meanings in AIPerf
`f10eb50a1776775d80d7f13396b0f668f14e2c03`,
`src/aiperf/dataset/loader/burst_gpt.py` and `loader/models.py`.
The importer rejects an invalid file in full. It does not reproduce upstream
malformed-row skipping, prompt synthesis, prefix graph synthesis or other
trace formats such as Mooncake, Bailian or TraceLab.

For example, this source contains two independent single-turn targets:

```csv
Timestamp,Request tokens,Response tokens
1.25,11,4
2,11,8
```

Convert it with Go and retain both the original CSV and the resulting JSON.
This is a standalone program using the package, not an installed command:

```go
package main

import (
    "encoding/json"
    "log"
    "os"

    "github.com/tmc/misc/xk6-llm-perf/traceworkload"
)

func main() {
    trace, err := traceworkload.ReadBurstGPT(os.Stdin)
    if err != nil {
        log.Fatal(err)
    }
    if err := json.NewEncoder(os.Stdout).Encode(trace); err != nil {
        log.Fatal(err)
    }
}
```

The JSON envelope includes `sourceHash` over all CSV bytes, a versioned format,
explicit source/output units and a count policy. Each row includes a stable
source-hash/record-ordinal `id`, `sourceRow`, `offset`, `inputTokens` and
`outputBudget`. The latter two are declared targets. An output budget is a
generation limit, never an observed output count.

Use independently counted candidate prompts to realize those targets. The
existing [offline tokenizer](../tools/llm-tokenize/README.md) emits receipts for
exact text bytes and a pinned vocabulary. The following k6 initialization uses
those receipts; `receipts` is an array and `candidates` is an array of strings:

```js
import {realize, verifiedCounter, trace} from './lib/workload.js';
const imported = JSON.parse(open('targets.json'));
const receipts = JSON.parse(open('receipts.json'));
const candidates = JSON.parse(open('candidates.json'));
const count = verifiedCounter(receipts, receipts[0].tokenizer_sha256);
const rows = trace(realize(imported.rows, candidates, count, {
    assetHash: receipts[0].tokenizer_sha256,
    tokenizer: receipts[0].measurement.tokenizer,
}));
```

Retain the whole imported envelope with the original CSV and tokenizer receipts.
The rows feed `replayArrival` as described in
[reproducible workloads](../examples/workloads/README.md); dispatch each row's
`prompt` with `max_tokens: row.outputBudget`. Those offsets are absolute relative
to one benchmark epoch. The older `trace-replay.js` example uses per-request
sleep delays and is not this absolute-arrival adapter.

Imports are limited to 16 MiB, 100000 records, 1000000 tokens per count and one
day of nonnegative monotonic offsets. Missing, fractional, negative or excessive
counts and duplicate/missing headers refuse admission. Zero input counts are
valid; output counts must be positive because the realization helper requires a
positive generation budget. Zero-output source rows are explicitly unsupported.
No input is clamped, sorted or skipped. An exact-count candidate that is absent
from the supplied pool causes realization to fail. Raw-text token receipts do
not establish chat-framing counts, provider token counts or cache hits.
