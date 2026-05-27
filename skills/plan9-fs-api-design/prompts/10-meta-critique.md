# Probe 10 — Meta-critique of the chat instructions themselves

Adopt **two lenses in sequence**: first **hostile reviewer (un-plan9
hunter)**, then **Plan 9 historian**. State blind spots for each pass.
This probe is run periodically against the chat instructions
(`chat-instructions.md`) and the prompts (`prompts/*.md`), not against
any specific proposal — it sharpens the lever rather than any one
output.

Inputs (all already in the notebook):

- The current `chat-instructions.md` (the seed prompt).
- The current `prompts/*.md` (probe set).
- The source manifest/source brief pattern used by recent runs.
- One or more recent dogfood outputs from this skill (the synthesized
  `<service>(4).md` files under `$WORK/out/`).

### Pass 1 — hostile reviewer

Find every place the chat instructions or probes enable un-plan9-like
output. Verdict labels:

- **LEAK** — a probe or rule permits a class of un-plan9 output that the
  inviolable rules say should be forbidden. Quote the permissive text.
- **DULL** — a rule is so general that the model can satisfy it with
  boilerplate. Propose tightening.
- **CONFUSING** — a rule contradicts another rule, or a verdict
  vocabulary is ambiguous. Quote both sides.
- **MISSING-LENS** — a class of API the skill should handle well isn't
  served by any lens in the seed list. Name the lens that should be
  added.
- **MISSING-SOURCE-GATE** — a prompt can proceed with the wrong or
  incomplete source corpus, or can cite a manifest/brief instead of a
  primary source.

For each finding, propose a concrete diff to `chat-instructions.md` or
the relevant `prompts/*.md` (one to three lines added/removed).

### Pass 2 — Plan 9 historian

Find every place the chat instructions or probes lean on lore without a
citation a competent reviewer can verify. Verdict labels:

- **UNCITED** — claim about Plan 9 design without manpage/paper backing.
  Name the source that should be cited.
- **MISLEADING-PRECEDENT** — a precedent invoked in the wrong context
  (e.g. citing `acme(4)` for a non-message-routing surface).
- **MISSING-PRECEDENT** — a Plan 9 primitive that should be reachable
  from the seed lenses but isn't named. Propose adding it.

### Synthesis

Bulleted list of concrete edits, ordered by leverage. Each edit:

```
- **File:** `chat-instructions.md` (or `prompts/NN-name.md`)
- **Change:** <one-line diff summary>
- **Reason:** <verdict label + source quote>
- **Expected effect on output quality:** <one line>
```

The skill author applies the edits, re-runs the dogfood, and re-runs
this probe. The loop terminates when this probe surfaces zero **LEAK**,
**CONFUSING**, or **UNCITED** findings (DULL and MISSING-LENS are
quality knobs, not floor violations — they can persist).
