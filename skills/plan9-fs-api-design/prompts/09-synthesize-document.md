# Probe 9 — Final Document

Adopt the **senior systems architect / technical writer** lens. State
blind spots in one line.

Emit a complete manual-page-style design:

`# <service>(4) — A Plan 9 / Wanix Synthetic Filesystem for <API name>`

Use the API identity and service root from probe 00 plus the authoritative
tree from probe 02b. Do not substitute names from prior runs. If the
user explicitly framed a root such as `/<service>`, preserve it unless
the primary sources make that impossible.

This is final integration, not a new design pass. Carry forward the prior
probes' final section material:

- § 1 and § 6 from probe 07;
- § 2 from probe 02b, repaired by probe 04;
- § 3 from probe 05a;
- § 4 from probe 05b;
- § 5 from probe 06;
- Recommendations and Caveats from probe 08.

Write only the TL;DR and Key Findings fresh. If two prior probes conflict,
prefer probe 00 for API identity/source truth, probe 02b for tree shape,
probe 04 for repairs, and probe 01 for API-surface completeness. Do not
silently invent a third shape.

Sections:

- `## TL;DR`
- `## Key Findings`
- `## Details`
- `### 1. Philosophy and design approach`
- `### 2. Filesystem tree`
- `### 3. Per-file / per-directory semantics`
- `### 4. Feature-by-feature mapping table`
- `### 5. Worked end-to-end examples`
- `### 6. Notes on deviations from pure Plan 9 style`
- `## Recommendations`
- `## Caveats`

Hard gates:

- direct frame for session-shaped APIs:
  `/<service>/clone -> $id/{ctl,data,stream,status,event}`;
- include the service-wide capability/model/provider scope chosen in
  02b, with progress/event files when the upstream exposes progress;
- include the numbered directory files chosen in 02b, including
  creation-time files and per-session event stream when source-backed;
- include the accounting/context scope chosen in 02b when source-backed;
- include per-operation numbered dirs when the API supports multiple
  in-flight operations;
- include callback/tool `call`/`return` pairs when the API has runtime
  callbacks;
- input files are input, `data` is complete output, and `stream` is
  incremental output;
- creation-time files freeze after `$id/ctl start` when upstream creation
  has immutable options;
- binary references are relative, for example `@in/img0`;
- no loose root files for service-wide state when a scope directory owns
  that lifecycle;
- use current names only.

Before emitting the final document, perform a private consistency pass:
every § 5 path must have a § 3 entry, every § 5 ctl verb must appear in
a § 3 verb table, every § 4 API surface must appear in the inventory from
probe 01 or be marked fails-closed, and every deliberate omission must
appear in the supported/fails-closed block.

If a consistency check fails, keep the closest prior-probe material and
add a Caveat naming the mismatch. Do not "fix" a missing source-backed
surface by inventing files that were not in the 02b/04 tree contract.

Keep compatibility-name and migration discussion out of the target design.
If you would explain name history, delete that sentence instead.

Use no outer code fence. Start at the H1 and end after Caveats.
