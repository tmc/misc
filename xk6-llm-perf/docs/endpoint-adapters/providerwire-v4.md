# ProviderWire v4

Use `raw.protocol: "providerwire-v4"` with an authored request body and API path
(usually `/language-model`). The adapter does not translate OpenAI messages.
`model` is required; `raw.stream` controls streaming. Model, streaming and
specification version are sent in `Ai-Language-Model-*` headers, not in the body.

```javascript
client.complete({model: 'model-id', raw: {
  protocol: 'providerwire-v4', path: '/language-model', stream: true,
  body: {prompt: [{role: 'user', content: [{type: 'text', text: 'Hello'}]}]}
}});
```

The supported subset includes system/user/assistant text prompts, assistant
reasoning, function-tool declarations, text and reasoning output, and function
calls. File/custom/source parts, tool-result output, tool-history prompts,
provider-executed or dynamic tools, header overrides and raw chunks are refused.
Unknown fields and events fail rather than being silently omitted.

Streams require `stream-start`, matching text/reasoning boundaries, complete
tool calls, and a `finish` event. EOF or `[DONE]` without `finish` fails. Tool input
is assembled once and checked against the completed call. Reasoning starts TTFT;
text or tool output starts TTFO. Absent token fields remain unavailable, including
cache read/write and reasoning counts; explicit zero remains measured zero.
Reported bucket sums must be consistent with supplied totals.

The dialect is pinned to Grafana AI SDK
`bb0cacce07ab19e8018112902ddd0ee3806e00bb`; schema URLs and SHA-256 hashes are in
`testdata/providerwire-v4/schema-pins.json`. The adapter is originally authored
against those schemas. Fixtures establish bounded software compatibility;
a hosted gateway or provider requires separate live qualification.
