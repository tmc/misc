# Probe 6 — Section 5 Examples

Adopt the **webfs(4) pragmatist** lens. State your blind spots.

Write `### 5. Worked end-to-end examples` as terse Wanix/rc shell
transcripts for the filesystem root chosen in probe 02b.

Include examples for the source-backed surfaces the API actually has.
For session-shaped APIs, cover:

1. create draft session, write `initial`/`opts`, `start`, prompt,
   read `data`;
2. streaming operation via the operation stream file;
3. per-operation one-shot constraint, if the API has constraints;
4. multimodal input using `in/ctl`, `in/img0`, and relative `@in/img0`;
5. tool `call`/`return`;
6. clone and destroy with `$id/event` observation.

Use only current names. Keep compatibility-name and migration discussion
out of the target design. Every path used in examples must appear in
§ 3, and every ctl verb used here must appear in a § 3 verb table.
