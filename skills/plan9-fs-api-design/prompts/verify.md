Review the draft `__ROOT__(4)` manpage below against the notebook sources as a
hostile verifier, then emit a repaired complete manpage. Work only from the sources.

Check and fix, in order:
1. Hallucinated surfaces — any method, option, verb, file, or event not traceable to a source symbol (a name borrowed from another API counts). Remove or correct.
2. Imported vocabulary — scope/file names belonging to a different kind of API (`model/`, `prompt/`, `tools/`, `@in/img0`, `baudRate`, etc.) without source support. Strip.
3. Missing surfaces — any source-backed surface absent from both tree and feature table. Add, or list fails-closed.
4. Overloaded files — any file carrying two roles the API distinguishes (e.g. a duplex stream on a read-only file). Split.
5. Shape fit — wrong tree shape (a stateless API forced into clone/$n, or a duplex/instance API flattened). Fix.
6. Example coherence — rc transcripts must use real tree paths and ctl verbs that appear in the verb table.
7. Citations — repair malformed bracket markers; do not invent a reference list.

Emit the COMPLETE repaired manpage, same section schema, starting at the H1, no outer code fence. Add a final Caveats bullet `verify-pass: <one line on what changed, or "no changes needed">`. No diff or commentary outside the manpage.
