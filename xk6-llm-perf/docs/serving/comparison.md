# Compare saved runs

`llm-compare` validates two to 64 saved run files and keeps each run's metric
populations, unavailable observations, measurement policies and failure counts.
It plots individual run means. It does not pool requests, infer independent
process runs or qualify a performance difference.

```sh
go run ./cmd/llm-compare -format html -metric ttft outputs/a.json outputs/b.json > comparison.html
go run ./cmd/llm-compare -format svg -metric ttft outputs/a.json outputs/b.json > comparison.svg
go run ./cmd/llm-compare -format json outputs/a.json outputs/b.json > comparison.json
```

HTML retains all metric means and sample counts, plus each run's measurement,
window and warmup policies. SVG labels unavailable metrics `N/A`; genuine zero
has a zero-width bar and its original positive sample count. No confidence
interval or ratio is implied. Compare policies and available populations before
interpreting bars. A changed policy remains a descriptive comparison. Observed numeric SLO thresholds and their completeness are retained per run.
Zero limits are disabled; missing limits remain unavailable. Known SLO
disagreement within one run is rejected. Differences across runs remain
visible in the descriptive comparison; no policy equivalence is asserted.

PNG export uses an explicitly chosen `rsvg-convert` executable and its SHA256:

```sh
go run ./cmd/llm-compare -format png -metric ttft \
  -png-renderer /absolute/path/rsvg-convert \
  -png-renderer-sha256 SHA256 outputs/a.json outputs/b.json > comparison.png
```

The renderer receives the same SVG on stdin. On Unix its owned process group
has a ten-second deadline, bounded stdout/stderr and cleanup after leader exit.
Output must decode as PNG, with each dimension at most 4096 pixels. The
current 80-pixel row layout allows at most 49 runs in a PNG; JSON, HTML and
SVG comparisons accept up to 64 runs. Larger PNG layouts fail explicitly. Renderer
hashes identify its executable bytes; they do not pin dynamic libraries or fonts.
Keep the executable path immutable during execution and retain renderer version,
library/font versions and the JSON/SVG alongside the PNG. Escaped process groups
require external confinement. Renderer/hash failures produce an error rather
than a partial PNG on stdout.

For an updating saved-run comparison:

```sh
go run ./cmd/llm-compare -serve 127.0.0.1:8767 outputs/a.json outputs/b.json
```

Open the exact printed address in a browser. The dashboard refreshes validated
snapshots every two seconds and offers a metric selector. It displays units,
means, sample counts, failure/drop/boundary populations and policies. Invalid or
partial current files clear the displayed values and show an unavailable status.
Use regular files and publish snapshots by atomic replacement. The server binds
only a literal loopback address and rejects other Host/Origin values. It provides
no authentication or remote access. Saved-run comparison is Unix-only because
nonblocking file admission prevents a FIFO from wedging a refresh request.
