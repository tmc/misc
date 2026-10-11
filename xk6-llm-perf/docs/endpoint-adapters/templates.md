# Structured request templates and endpoint adapters

`renderRequestTemplate` builds an authored JSON body for `Client.request`.
References occupy a whole JSON value: `{"$var":"texts"}` inserts the `texts`
variable as its original JSON type. Strings, object keys, and variables remain
literal data. Missing variables, references with extra keys, root scalars,
JSON over 1 MiB, nesting over 32, and expansions over 100,000 nodes fail.
No files, environment variables, network assets, or code are evaluated.

```js
import { Client, renderRequestTemplate } from 'k6/x/llm-perf';
const client = new Client({ baseURL: 'http://localhost:8000' });
const rendered = renderRequestTemplate({
  name: 'openai-embeddings',
  variables: { model: 'local', texts: ['hello'] },
});
// In the VU function:
const response = client.request({
  path: '/v1/embeddings', protocol: 'openai-embeddings', body: rendered.body,
});
```

Keep `rendered.provenance` in the workload manifest. Its schema version, scope,
name, and SHA-256 hashes identify canonical JSON template, variables, and payload.
Hashes change with the supplied values; they do not anonymize predictable inputs.
These receipts identify authored payloads, not independently verified token counts.

| Name | Variables | Response protocol |
| --- | --- | --- |
| `chat-embeddings` | `model`, `messages` with text/image chat content | `chat-embeddings` |
| `nim-embeddings` | `model`, optional `texts` and `images` as strings | `nim-embeddings` |
| `solido-rag` | `model`, `queries` as strings | `solido-rag` |
| `nv-embedqa` | `texts` | `nv-embedqa` |
| `openai-embeddings` | `model`, `texts` | `openai-embeddings` |
| `cohere-rankings` | `model`, `query`, `passages` as strings | `cohere-rankings` |
| `nim-rankings` | `model`, `query`, `passages` as `{text: ...}` objects | `nim-rankings` |
| `hf-tei-rankings` | `query`, `passages` as strings | `hf-tei-rankings` |
| `nim-image-retrieval` | `images` as `{type:'image_url', url: ...}` objects | `nim-image-retrieval` |

The named response adapters require nonempty bounded results. OpenAI embeddings accept textual inputs, token-ID arrays, and batches of either
form (no mixed batches), bounded to 4,096 items or tokens per input. Token IDs
must be nonnegative integers within int32. `encoding_format` may be `float` or
`base64`. Embedding results require numeric vectors or strict base64 float32
vectors, nonempty and bounded to 65,536 coordinates. Base64 bytes must have a
length divisible by four and finite little-endian float32 coordinates; retained
response bodies preserve their original encoding. OpenAI results require
embedding objects. `nv-embedqa` also accepts the pinned template endpoint
formats `embedding: [...]` and `embeddings: [[...], ...]`. Rankings require numeric scores and unique,
nonnegative integer indices. TEI accepts either a top-level result array or an
object with `results`. Retrieval retains an array of result objects unchanged.
Reported usage remains available with its presence metadata; missing generation
counts and content timing remain unavailable. Streaming is rejected for the embedding, ranking, and image-retrieval
protocols. Image URLs are sent to the inference server; this client does
not fetch them. Endpoint-specific extras can be expressed in an inline template.
Inputs are authored values; providing token IDs does not establish that their
vocabulary matches the server model. No response content becomes generated text.

The source basis is AIPerf commit
`f10eb50a1776775d80d7f13396b0f668f14e2c03`, specifically
`src/aiperf/endpoints/template_endpoint.py`, `openai_embeddings.py`,
`nim_rankings.py`, `cohere_rankings.py`, `hf_tei_rankings.py`, and
`nim_image_retrieval.py`, `nim_embeddings.py`, `chat_embeddings.py`,
`solido_rag.py`, and the embedding extraction in `base_endpoint.py`. AIPerf exposes full Jinja2 request rendering and JMESPath
response selection. This structured subset has no loops, filters, string
interpolation, or response query language. It does not claim Jinja2/JMESPath
compatibility or qualify live vendor servers.

For authored loops, filters, macros, and response query expressions, use the
[offline Jinja/JMESPath workflow](template-compat.md). It prepares payloads and
selects captured response values using pinned libraries; runtime adapter and
token-count qualification remain separate.

`examples/template-endpoints.js` and `TestBinaryTemplateEndpoints` exercise all
nine named protocols against a local HTTP fixture. The test compares exact
payload shapes, retained responses, absent generation counts, receipts, and
missing-variable rejection using the built k6 extension. Focused Go tests cover
invalid shapes, duplicate indices, type preservation, expansion bounds, and
stable hashes. Use `K6_SMOKE_BINARY=./dist/k6 go test . -run
TestBinaryTemplateEndpoints` after rebuilding the extension.

OpenAI input and encoding forms are grounded in the
[official embeddings reference](https://developers.openai.com/api/reference/resources/embeddings/methods/create).
The float32 byte interpretation follows the
[official SDK decoder](https://github.com/openai/openai-node/blob/master/src/internal/utils/base64.ts).
The binary fixture additionally sends single and batched token-ID requests and
retains base64 responses. Malformed encoding, nonfinite coordinates, empty
vectors, mixed/negative/fractional token IDs, and absent NV results have regression
tests.


The `nim-embeddings` template follows the pinned NIM composition rule: text-only
or image-only arrays pass through as `input`; paired text/image arrays must have
equal lengths and are concatenated with one space per pair. Empty inputs,
non-string items, and unequal pairs fail. `chat-embeddings` validates chat messages
containing text or image-URL parts and extracts embeddings from the common formats
recognized by the pinned source, without interpreting an embedding as generated
chat content.

`solido-rag` is a generation adapter, unlike the embedding adapters. Its named
payload contains a query string array, default filters `{family:'Solido',
tool:'SDE'}`, and `inference_model`. Inline templates can supply authored filters
and extras. Synchronous responses require a nonempty `content` string. SSE
`content` values are incremental generated text; a sources/usage trailer creates
no text or termination event. The common `[DONE]` marker terminates streaming.
Sources must be objects or arrays and are retained as authored metadata. Streaming
retains the latest present sources field, bounded by the complete 16 MiB SSE
budget. Provider errors and malformed content/sources fail. Reported token counts
retain presence; generated content contributes TTFT/TTFO, while strict mode keeps
missing counts unavailable. Source documents are never fetched by this client.

The built-k6 fixture now exercises twelve HTTP requests: nine named protocols,
two token-ID/base64 requests, and the SOLIDO SSE lifecycle, including replacement
of initial sources by a final metadata trailer. It compares exact payload shapes,
checks SOLIDO generated content/timing/reported counts, and checks absent
generation counts for embedding/ranking/retrieval results. Live provider
qualification remains separate from these local protocol fixtures.
