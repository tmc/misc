# Security

`xk6-llm-perf` sends prompts, model names, and API keys to configured
OpenAI-compatible endpoints. Treat load-test inputs and outputs as sensitive.

## Reporting

Report security issues privately to the maintainers of this repository. Do not
open public issues for credential leaks, request smuggling, or endpoint
exfiltration reports.

## Operational Guidance

- Pass API keys through environment variables, not checked-in scripts.
- Do not publish k6 summaries containing sensitive prompts or completions.
- Use `ENDPOINT_URLS` only for trusted endpoints.
- Review trace replay files before sharing them; they may contain production
  prompts.
- Keep generated binaries out of git.

## Scope

The extension does not implement authentication protocols itself. It forwards
Bearer tokens to the configured chat completion endpoint and records metrics
from request/response timing and provider usage metadata.
