# Effective serving manifests

Attach a version-1 `servingManifest` to `beginMeasurement` to retain effective
server configuration in the exported `serving_manifest`. Existing runs without
this optional field remain readable. Distributed workers must carry identical
manifests; different artifacts or evidence cannot silently merge.

Use SHA-256 identities for the model, tokenizer, chat template, workload and
comparable environment. `arm` identifies the server binary, source revision,
backend and endpoint without making those artifacts equivalent. Include native
libraries, hardware, drivers and resource conditions in the environment digest.
Do not put credentials, raw prompts or generated text in the manifest. Use an
opaque endpoint identity rather than an authenticated URL. Endpoint identities
accept only ASCII letters, digits, dot, underscore and hyphen; URLs and query
strings are rejected.

Each setting separates `configured` from `effective`, with `evidence_digest`
identifying the receipt that observed the effective value. A requested value is
not evidence that the server used it. Explicit numeric zero is preserved.
Omission means unknown; record an observed inapplicable setting explicitly (for
example `{"applicable":false}` for a structured cache setting).

```javascript
llm.beginMeasurement({
  runId: 'serving-arm-a', duration: '30s', maxRecords: 1000,
  servingManifest: JSON.parse(open('./effective-serving.json')),
});
```

The Go APIs `ReadServingManifest` and `ValidateServingManifest` reject unknown
fields, duplicate keys, null evidence, invalid numeric settings and oversized
inputs. The input is limited to 64 KiB, each setting to 4 KiB and nesting to 16.

`ServingWorkHash` requires all five identities and effective, evidence-backed
values for cache, compile, device, dtype, max_output_tokens, resources, scheduler,
seed, temperature, top_k and top_p. Declared confounds prevent qualification.
It hashes effective values and comparable identities, excluding the arm,
configured values and probe receipt identity. Settings with reordered object
keys hash identically. Different JSON numeric representations remain distinct,
which conservatively refuses equivalence rather than inferring it.

A matching hash proves matching retained declarations. It does not authenticate
receipts, attest the server, establish native engine parity, or qualify physical
performance. Collect and independently check the referenced server-side evidence
before interpreting a serving comparison.

Sanitized output documents retain a canonical `manifest_digest` and either a
`work_hash` or `work_unavailable` reason. They do not copy arbitrary endpoint or
setting strings. The full manifest stays in the retained run artifact; its hash
binds that artifact's serving metadata to the sanitized summary.
