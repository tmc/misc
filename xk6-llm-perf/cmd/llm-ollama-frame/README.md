# Ollama framing replay

`llm-ollama-frame` reconstructs the pinned Ollama 0.12.7/Qwen3 text renderer
offline, before context truncation. It accepts text system/user/assistant
messages and model-level system/message defaults only when the request and
`/api/show` metadata SHA-256 byte pins match. The captured Qwen3 template must
have SHA-256
`ae370d884f108d16e7cc8fd5259ebc5773a0afa6e078b11f4ed7e39a27e0dfc4`.

Low/medium/high reasoning effort in either top-level or nested fields, other
templates, nonempty tools, tool/image messages, structured reasoning fields,
nonempty parser/renderer/remote overrides, unknown request/default-message
fields, null request content, duplicate JSON members (including case-folded
aliases) and invalid UTF-8 are refused. Model defaults may be absent, null or
empty: the pinned API represents system as a string and messages as a slice,
so JSON null decodes to their zero values. Input is limited to 64 combined
messages and 64 KiB per text/system value; output remains limited to 1 MiB.
Input opens are nonblocking, reject symlinks and require regular files; reads and
rendered writes are limited to 1 MiB. Only this pinned finite template executes.

```sh
go run ./cmd/llm-ollama-frame \
  -model-show model-show.json -model-show-sha256 "$SHOW_SHA256" \
  -request request.json -request-sha256 "$REQUEST_SHA256" \
  -provider-version 0.12.7 -tokenizer-input rendered-input.json
```

The explicit `tokenizer-input` file contains sensitive rendered prompt text. It
is created exclusively with permissions 0600; existing destinations are refused.
Pass it to the pinned HF tokenizer bridge as raw text without special tokens.
The schema 2 stdout receipt contains hashes, source-derived reconstructed settings,
source revision and an explicit `engine_observation_status: "unobserved"`; it omits prompt text. Input model/version
labels and hashes identify supplied evidence and do not attest to a server.

The source policy is Ollama tag `v0.12.7`, revision
`c88647104d1621b5021d6e3ca18fb8a1f5cdd394`:

- `openai/openai.go`, `FromChatRequest`: reasoning effort `none` sets Think=false;
  omitted effort leaves it unset. Only absent/empty effort and `none` are admitted in this Qwen lane.
  Nested `reasoning.effort` takes precedence within that admitted population.
  The pinned provider refuses string efforts for non-Harmony parsers; this lane
  does not attest Harmony and refuses low/medium/high in either field.
- `server/routes.go`, ChatHandler: thinking-capable models default unset Think=true.
- `server/prompt.go`, renderPrompt: passes Think and IsThinkSet to the template.
- `server/routes.go`: model messages precede request messages; a nonempty model
  system is prepended unless the first request message is system. Qwen3's
  `filterThinkTags` strips leading thinking from assistant history before the
  final user message, using the complete content as one parser input.
- `template/template.go`: collects all system text with double newlines and
  merges consecutive same-role messages with double newlines. Assistant prefill
  after the last user is preserved.

For the retained prompt ending `/no_think`, the serving template appends a
second command: default thinking produces `/no_think /think`. Official pinned
Qwen tokenizer and the Go tokenizer both yield 21 identical token IDs for this
rendering; the unmodified HF chat template yielded 19. All 64 retained provider
responses report 21 prompt tokens. Explicit `reasoning_effort: "none"` reconstructs
`/no_think /no_think` and an empty `<think>` block: both offline tokenizers yield
27 identical IDs. That explicit-none cell is an offline negative/positive policy
control, not a newly observed provider response.

A prompt-text `/no_think` suffix is therefore insufficient to declare the
server's thinking flag disabled. For this pinned provider use the OpenAI request
field `reasoning_effort: "none"` when disabling thinking; support is provider
specific, so verify the effective policy for each comparison arm.

The template fixture is the exact `/api/show` template retained for Ollama
0.12.7/Qwen3:0.6b, with its byte hash tested. It is a Qwen/Ollama ChatML template;
upstream Qwen3 and Ollama sources use Apache-2.0 and MIT licensing respectively.
The txtar CLI fixture derives only template and capability fields from those
bytes; it exercises the compiled command, pins, both policies and exclusive
output creation. Full captured metadata/raw wire receipts remain task-local.

This resolves the count discrepancy through a source-and-template reconstruction.
The actual engine prompt, applied flags and GGUF tokenizer IDs were not captured.
Matching offline IDs and provider counts do not establish those identities,
engine configuration attestation, full GGUF/HF parity or performance equivalence.
The lane performs no model/GPU calls and does not change the physical HOLD.

Receipt schema 2 replaces schema 1's `effective_think` with `reconstructed.think`.
Old schema 1 receipts retain their original meaning: the field was a reconstruction,
not an observed engine setting. Consumers must branch on `schema_version`; no
schema 1 receipt is rewritten or promoted by this change.

| Schema 2 field | Meaning |
| --- | --- |
| `reconstructed.think` | Pinned source/template policy within the admitted request population |
| `reconstructed.temperature` | Pinned OpenAI conversion: declared value, otherwise 1.0 |
| `reconstructed.max_tokens` | Declared value; null means the server/model default remains unknown |
| `reconstructed.stream` | Declared OpenAI streaming flag; omission means false |
| `engine_observation_status` | Always `unobserved`; none of these fields attest engine application |

`max_tokens` values above the pinned 64-bit OpenAI parser's signed integer range
are refused before rendering. This is a parser compatibility bound, not a claim
that the server can generate that many tokens. Model defaults, context truncation,
tokenizer special tokens and effective generation settings still require separate
observations. Nonempty tools, alternate renderers/parsers and modality processing remain unsupported.

The receipt keeps schema 2 and adds
`context_policy: "untruncated-renderer-reconstruction"`. Existing single-user
rendered bytes and reasoning policies remain unchanged. Historical schema 2
receipts without this field remain scoped by their original labels; they do not
gain system/history coverage retroactively.

The renderer follows model-default insertion, Qwen3 assistant-thinking filtering
and consecutive-role collation. For example, a request beginning with a system
message suppresses the model system but preserves model message history. Two
consecutive user messages become one content string joined by `\n\n`; only that
last user message receives `/think` or `/no_think`. Leading complete, incomplete
and partial `<think>` tags in assistant history are handled like the pinned
one-shot parser. The final assistant prefill is not filtered.

Thirty-six exact prompt fixtures were generated by the actual pinned Ollama
`template.Parse`/`Execute` and `thinking.Parser`, with a task-local adapter for
the source's default/filter order. They cover default/none thinking, request
system override, model defaults, consecutive and interspersed roles, assistant
prefill, empty content, leading/partial/malformed thinking tags and Unicode
whitespace. The oracle used Go 1.27.2 with cached task-only dependencies,
including a different x/crypto version needed to compile unused authentication
code. It is a renderer/parser source oracle, not a full upstream-runtime or
server qualification. The project gained no dependency.

`testdata/qwen3-prompt-ids.jsonl` retains the token IDs of those 36 prompts,
computed offline by the pinned `tools/llm-tokenize`
(hftokenizer v0.0.0-20261010162052-7e9483795cd2) from the Qwen3-0.6B
`tokenizer.json` at revision c1899de289a04d12100db370d81485cdf75e47ca
(SHA-256 aeb13307…), without added special tokens. The same tokenizer
reproduces the retained 21-ID single-user control. Tests check that each row
names the SHA-256 of the exact rendered prompt and that its Qwen3 added-token
IDs match the literal markers in that text. A pinned Hugging Face runtime
second oracle for these IDs is task-local and is not run by the tests. These
are offline tokenizer IDs; GGUF engine IDs remain unobserved.

`chatPrompt` can remove context using the engine tokenizer and resolved
`NumCtx`. This command has neither observation and therefore does not claim to
reconstruct that truncation. Matching these untruncated prompts cannot prove
that the server retained every turn. Tools need processed tool/parser state and
exact structured call serialization; modality processors need pinned assets and
reference counts. These remain separate gaps, as do actual engine prompt/token
IDs and effective generation settings.
