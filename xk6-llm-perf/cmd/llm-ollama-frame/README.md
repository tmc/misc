# Ollama framing replay

`llm-ollama-frame` reconstructs one bounded Ollama 0.12.7/Qwen3 chat-template
lane offline. It accepts the retained one-user text request and `/api/show`
metadata only when their supplied SHA-256 byte pins match. The captured Qwen3
template must have SHA-256
`ae370d884f108d16e7cc8fd5259ebc5773a0afa6e078b11f4ed7e39a27e0dfc4`.
Nonempty or null model-level `system`/`messages`, low/medium/high reasoning
effort in either top-level or nested fields, other templates,
tool/image/system/multiple-message populations, unknown request
fields, null evidence, duplicate JSON members, including case-folded aliases, and invalid UTF-8 are refused.
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
- `template/template.go`: one user message requires no system/message collation.

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
observations. Tool, system and multiturn cells remain explicitly unsupported.
