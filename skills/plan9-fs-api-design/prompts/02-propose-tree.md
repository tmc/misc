# Probe 2 - Propose the namespace tree

Adopt the **namespace-outward architect** lens with a brief Plan 9
precedent pass. State your blind spots.

Produce only a candidate `## 2. Filesystem tree`. Keep this probe terse:
the next probe will refine the authoritative tree.

Use probe 00 for API identity and probe 01 for surface completeness.
Preserve the user/source-chosen root. For session-shaped APIs, preserve
the direct Plan 9 frame:

`/<service>/clone -> $id/{ctl,data,stream,status,event}`

Required output:

1. A single fenced ASCII tree rooted at `/<service>/`.
2. A short "Notes on the layout" paragraph naming lazy allocation,
   fid/resource cleanup, and auth posture.
3. A compact "Coverage check" bullet list mapping service, session,
   operation, accounting, typed input, event, and callback/tool scopes
   when the API has them.

Tree rules:

- Include a root `clone` allocator and numbered session dirs.
- Keep `ctl`, `data`, `stream`, `status`, and `event` as direct
  per-session files for the main session lifecycle.
- Put service-wide availability, params, and download/progress state
  under a dedicated service scope such as `model/`.
- Put per-call constraints and multiple in-flight prompt work under a
  numbered operation scope.
- Put typed binary/audio/image inputs in an `in/` scope and reference
  them by relative names such as `@in/img0`.
- Put callback/tool surfaces in named subdirectories with `call` and
  `return` pipe pairs.
- Use current upstream names only. Do not mention older names,
  compatibility aliases, redirects, or migration support.

Do not draft any other section.
