# Probe 1 - Survey the input API

Adopt the **source-cartographer / technical writer** lens. State your
blind spots.

Build a compact, current-name inventory of the input API as it appears in
the scoped notebook sources. Use probe 00 as the source contract. If probe
00 identified gaps, keep them visible and do not fill them from memory.

Do not propose a filesystem tree yet. This probe only names the upstream
surfaces and the lifecycle pressure they create for the later filesystem
design.

Use current upstream names only. If sources discuss name history, ignore
that material for this probe. Do not mention older names, aliases, or
migration history. When sources show two live spellings for the same
capability, keep the source-preferred current spelling in the table and
note the paired spelling once in completeness notes; do not drop the
capability or classify a live spelling as historical unless the source
explicitly does so.

## Required output

Emit exactly these sections.

### Current API surface inventory

Use one markdown table with these columns:

`| surface | kind | owner | shape | lifecycle scope | filesystem pressure |`

Rules:

- Include every public interface method, readonly attribute, event-handler
  attribute, dictionary option, callback/tool surface, enum, and named
  event type visible in the sources.
- Keep each row terse. The `shape` cell should be a signature, type, or
  quoted source phrase of 12 words or fewer.
- Use source-backed current names. If a surface is unclear, include it and
  mark `source gap` in the `shape` or `filesystem pressure` cell.
- For `lifecycle scope`, choose the nearest real scope: service, draft
  session, live session, operation, input item, callback, event, or
  accounting.
- For `filesystem pressure`, choose a short tag: clone/session, ctl verb,
  input file, output file, stream file, status file, event file,
  call/return pipe, typed blob, immutable option, one-shot option,
  fail-closed, or no-op.

### Lifecycle notes

Four bullets:

- What creates each entity.
- What starts work.
- What streams or emits events.
- What aborts, destroys, or cleans up resources.

### Completeness notes

Two to four bullets naming any source gaps, uncertain surfaces, or
deliberate omissions. If none were found, write `none found`.

This inventory is the downstream completeness contract for probes 05b and
09.
