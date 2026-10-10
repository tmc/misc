# Offline token counting

This separate Go 1.27 command pins
`github.com/tmc/lib/hf/hftokenizer@v0.0.0-20261010162052-7e9483795cd2`.
The k6 extension keeps its own Go version and dependency set. Build once with
`GOWORK=off go build`; the resulting command needs only its local assets at run
time and does not fetch model files.

Pass a local `tokenizer.json`, its required SHA-256, model identity and revision:

```sh
llm-tokenize -tokenizer tokenizer.json -tokenizer-sha256 "$VOCAB_HASH" \
  -model fixture -revision pinned < input.json
```

Input is either `{"text":"hello"}` or a message array with optional
`add_generation_prompt`. Output preserves count, model/revision, tokenizer
module version, asset hashes, scope and framed-input hash. Prompt text is not
exported. `-ids` explicitly includes IDs, bounded to 100000. Text uses
`EncodeRaw`; `-postprocessor` explicitly selects the asset's single-input
postprocessor for text only.

Chat requires `-template frame.json -template-sha256 "$FRAME_HASH"`. The
version-1 role-frame dialect supports a literal overall prefix/suffix, literal
prefix/suffix for each declared role, and a literal generation prompt. The
synthetic example is in `testdata/framing.json`. Rendering and inputs are
bounded; unknown roles, multimodal/tool content, and unpinned assets are refused.
The framing asset must match the serving model. This command does not interpret
Jinja chat templates or infer framing from a model name.

The output's `measurement` object can supply request `measurement` options.
The extension records such values as caller declarations and retains the label;
keep the full preprocessing receipt as a workload asset. This demonstrates
verified local counting for the supported pinned pipeline. Provider tokenizer,
hidden reasoning and model-template compatibility require separate receipts.

`testdata/tokenizer.json` is the synthetic GPT-2 pipeline from the pinned
hftokenizer module's `testdata/gpt2.json`, SHA-256
`1d5a2116200e41511a30659d8bd90a2ba916a141582e3888b4178b743ca5ad8b`.
Expected text IDs come from that module's `tokenizers` 0.22.2 reference fixture;
they are synthetic vocabulary IDs, not the public GPT-2 vocabulary. Chat tests
add the fixture's known BOS/EOS/tag IDs and verify the entire framed sequence.
