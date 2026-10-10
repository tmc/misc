# Contributing

Keep changes small, testable, and aligned with k6 extension behavior.

## Development Loop

```bash
make check
```

`make check` runs race tests, vet, a signed extension build, HTTP/SSE smoke
checks, and concurrent trace replay against local fixtures. It needs no provider
credentials.

For live provider checks, run the built binary against an OpenAI-compatible endpoint:

```bash
ENDPOINT_URL=http://localhost:8000/v1 \
OPENAI_API_KEY=test \
STREAM_MODE=true \
./dist/k6 run examples/llm-test.js
```

## Code Style

- Prefer standard library types and simple data structures.
- Keep exported API shape compatible with OpenAI-style chat completions.
- Return errors for expected failures; do not panic in request paths.
- Add table-driven tests for config, parsing, metrics, and error behavior.
- Keep examples runnable with environment variables rather than hard-coded
  private endpoints.

## Pull Request Checklist

- Race tests pass with `GOWORK=off go test -race ./...`.
- Static checks pass with `GOWORK=off go vet ./...`.
- `make build` produces `dist/k6`.
- New metrics are documented in `README.md`.
- New runtime behavior has either a unit test or a local k6 smoke path.
- No generated binaries are staged.
