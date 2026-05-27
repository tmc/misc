# Probe 8 — Recommendations And Caveats

Adopt the **Wanix capability hardliner** lens with a release-planning
pass. State your blind spots.

Write final-section material for `## Recommendations` and `## Caveats`.

Recommendations: four implementation stages for a Wanix 9P service
using the root and feature scopes chosen in probe 02b. Start with the
smallest clone/session or request prototype, then add streaming and
accounting, then callbacks/tools and typed input when source-backed, then
hardening/docs.

Caveats: 6-10 concrete risks covering spec drift, browser/runtime
availability, model download behavior, concurrency, nondeterminism,
filesystem boundary loss for live host objects/functions, permissions,
and source coverage.

Use current names only. Keep compatibility-name and migration discussion
out of the target design.
