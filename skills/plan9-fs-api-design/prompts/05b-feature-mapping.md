# Probe 5b - Section 4 Mapping Table

Adopt the **source-cartographer** lens. State your blind spots.

Write `### 4. Feature-by-feature mapping table`.

Use probe 01's current API surface inventory as the source surface list.
Use the root and path families established by the 02b/05a design:
`/<service>/model/`, `/<service>/$id/{ctl,data,stream,status,event}`,
`/<service>/$id/ctx/`, `/<service>/$id/in/`,
`/<service>/$id/prompt/$n/`, and `/<service>/$id/tools/`.

Emit one markdown table:

`| API surface | file(s) | Read returns | Write does |`

Rules:

- Include every method, field, event, option, typed input, and callback
  category visible in probe 01.
- Keep rows terse; combine closely related enum values or dictionary
  fields only when they map to the same path and semantics.
- Use current upstream names only.
- If a surface is intentionally unsupported, write `fails closed` in the
  file column and name the reason.

After the table, add:

`Completeness audit: <mapped count> mapped; <fails-closed count> fails closed.`
