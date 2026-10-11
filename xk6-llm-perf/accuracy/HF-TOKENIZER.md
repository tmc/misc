# Pinned Hugging Face token counting

`examples/output/hf_tokenize.py` is an optional preprocessing bridge for real
Hugging Face chat templates. It uses `AutoTokenizer` from an explicitly pinned
local Python runtime. It accepts model message structures and JSON tool schemas,
and uses the asset's `apply_chat_template` before encoding with
`add_special_tokens=False`. Text input has a separate explicit
`add_special_tokens` option, defaulting to false.

Create a dedicated tokenizer asset directory containing only the required local
files. Supply a separate JSON manifest mapping **every** relative file name to
its SHA-256, and pin the manifest's byte hash. Symlinks, Python source files,
unlisted or changed assets and missing `tokenizer_config.json` are refused.
The asset total is limited to 64 MiB. Pin both installed `transformers` and
`tokenizers` versions; the bridge checks them before loading the tokenizer.
`local_files_only=True`, `trust_remote_code=False` and offline environment flags
prevent remote model downloads or custom tokenizer Python execution.

Example on Linux, in a separately prepared pinned runtime:

```sh
timeout 30s prlimit --as=4294967296 --cpu=25 -- \
  python3 examples/output/hf_tokenize.py \
  --assets /path/to/tokenizer-assets --manifest /path/to/manifest.json \
  --manifest-sha256 "$MANIFEST_SHA256" \
  --transformers-version "$TRANSFORMERS_VERSION" \
  --tokenizers-version "$TOKENIZERS_VERSION" \
  --model "$MODEL" --revision "$MODEL_REVISION" < messages.json
```

Inputs:

```json
{"messages":[{"role":"user","content":"hello"}],"add_generation_prompt":true}
```

```json
{"text":"hello","add_special_tokens":false}
```

Chat input also accepts `tools`, `continue_final_message` and the boolean model-specific
`enable_thinking`; unsupported options
or incompatible combinations are refused by the pinned runtime. Structured
multimodal messages are passed to the template, but the result counts **textual
tokens only**. Image/audio encoders and expansion into server-side media tokens
require a separate processor and are not measured by this bridge. Missing chat
templates fail; model names never imply a template.

Output retains worker, manifest, asset-tree and rendered-input hashes, model and
revision labels, dependency versions, count and scope. `--ids` explicitly emits
up to 100000 token IDs. The `measurement` object can become request measurement
options; the extension treats it as a caller declaration, so retain the full
receipt with the workload assets. Prompt text is absent from output and error
messages. Hashes identify supplied assets and software; they do not prove that
those assets match a serving model, or that its reported counts match these IDs.

The bridge bounds reads and checks rendered size after template rendering.
Rendering itself occurs inside Hugging Face and can allocate before that check.
Use an externally enforced memory/CPU/wall-time envelope, such as the Linux
example above, for every execution. Keep the asset directory and runtime
immutable throughout the call; preflight hashes do not prevent concurrent file
replacement. This is local preprocessing, not a sandbox for arbitrary templates
or an attestation of runtime dependencies beyond the two checked package pins.

Run `python3 examples/output/hf_tokenize_test.py` for admission tests. When the
optional runtime is installed, the same suite executes an actual local fast
WordLevel tokenizer and Jinja template with tools, multimodal placeholders and a
generation prompt; returned IDs must equal direct `apply_chat_template` IDs.
That synthetic control proves bridge mechanics. Real model asset compatibility
still needs a pinned model-specific receipt.
