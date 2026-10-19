# Responses stream timing

The `openai-responses` adapter accepts text deltas and done-only text parts.
Text from a matching `response.output_text.done` snapshot is admitted once;
it does not duplicate text already received in deltas. Conflicting snapshots,
repeated done events and changing part identities refuse the request.

Default content timings describe recognized text, reasoning and tool events.
The adapter also records a separate content-response view matching AIPerf
`f10eb50a1776775d80d7f13396b0f668f14e2c03`. Its Responses mapper retains an
empty text response for a matching done snapshot after deltas. That response
affects AIPerf timing even though its text is suppressed. Treating the two
populations as identical would hide this boundary difference.

| Optional measurement field | Meaning |
| --- | --- |
| `aiperf_ttft` | First mapped content-response receipt, in milliseconds from request start |
| `aiperf_last_content_latency` | Last mapped content-response receipt, using the same anchor and unit |
| `aiperf_content_events` | Number of responses in this mapped population |
| `aiperf_content_policy` | `aiperf-responses-content/f10eb50a` |

The mapped population includes nonempty output-text, reasoning-text and
function-argument deltas, done-only nonempty text, and the retained done
response after matching deltas. Additional supported reasoning-summary events
remain in the ordinary view and do not silently enter this pinned view.

An observed empty population has count zero and absent first/last timings.
Older records without the optional fields have no observed view. Recorded
failures can retain their partial view; its presence does not make them
successful. `ReadRun` validates policy, protocol, count and timing consistency.
These checks do not authenticate the producer or qualify clock accuracy.

The stream state is limited to 4096 text parts, 16 MiB of newly admitted text
across parts, 16 MiB per event and 2048 bytes per item ID. Existing transport
limits also apply. This adapter and its software controls do not establish
compatibility with an untested live provider.
