# oairt

Go client for the [OpenAI Realtime API](https://platform.openai.com/docs/guides/realtime) — WebSocket transport, typed events, streaming PCM16 audio.

## Status

Pre-1.0. The public API may shift before `v1.0.0`. Current tag: `v0.1.0`.

## Models

Works with `gpt-realtime` and `gpt-realtime-2`. Pass the model id to
`Client.Connect`.

`gpt-realtime-2` exposes a `reasoning.effort` knob (`"minimal"`, `"low"`,
`"medium"`, `"high"`, `"xhigh"`; default `"low"`). Set it on the session:

```go
client.Send(oairt.Event{
    Type: oairt.EventSessionUpdate,
    Session: &oairt.Session{
        Model:     "gpt-realtime-2",
        Reasoning: &oairt.Reasoning{Effort: "high"},
    },
})
```

Input transcription accepts `Language`, `Delay`, and `Prompt` in addition
to `Model`; see `oairt.AudioTranscription`.

## Install

Library:

    go get github.com/tmc/misc/oairt

CLI:

    go install github.com/tmc/misc/oairt/cmd/oairt@latest

## Quickstart (library)

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/tmc/misc/oairt"
)

func main() {
    client := oairt.NewClient(os.Getenv("OPENAI_API_KEY"))
    defer client.Close()

    client.On(oairt.EventResponseTextDelta, func(e oairt.Event) {
        if s, ok := e.TextDelta(); ok {
            fmt.Print(s)
        }
    })

    if err := client.Connect(context.Background(), "gpt-4o-realtime-preview"); err != nil {
        log.Fatal(err)
    }
    if err := client.Send(oairt.Event{Type: oairt.EventResponseCreate}); err != nil {
        log.Fatal(err)
    }
    select {}
}
```

## Quickstart (CLI)

    export OPENAI_API_KEY=sk-...
    oairt

The CLI runs in text mode by default. Pass `-audio` for streaming voice (darwin only).

## Supported platforms

| Platform | Text | Streaming audio |
| -------- | :--: | :-------------: |
| darwin   | yes  | yes (via `ffplay`; mic via TCC) |
| linux    | yes  | best-effort |
| other    | yes  | no |

## Configuration

Environment:

  - `OPENAI_API_KEY` — used by both the library quickstart and the CLI.

CLI flag precedence: `-api-key` flag overrides `OPENAI_API_KEY`.

Library options (pass to `NewClient`):

  - `WithLogger(Logger)` — custom logger; default is silent.
  - `WithURL(string)` — override the Realtime endpoint.
  - `WithUserAgent(string)` — set the `User-Agent` upgrade header.
  - `WithHTTPClient(*http.Client)` — custom HTTP client.
  - `WithDialer(*websocket.Dialer)` — custom dialer.
  - `WithDebug(bool)` — verbose logging through the configured `Logger`.
  - `WithDumpFrames(bool)` — log raw inbound and outbound frames.

## Events

Wire event names are exposed as `EventXxx` constants. Send with `Client.Send`,
register handlers with `Client.On`. Typed accessors `Event.TextDelta()` and
`Event.AudioDelta()` decode the most common payloads.

Coverage as of `v0.1.1`: 9 client→server and 27 server→client events. See
package godoc and `events_test.go::TestEventRoundTrip` for the full list.

`v0.1.1` adds constants for `conversation.item.added`,
`conversation.item.done`, and
`conversation.item.input_audio_transcription.delta`, plus
`Event.PreviousItemID` for the `previous_item_id` field on the
added/done variants.

## Errors

`Send` and `SendAudio` return sentinel errors that callers may match with
`errors.Is`:

  - `ErrNotConnected` — `Connect` has not yet succeeded, or `Close` has run.
  - `ErrSendQueueFull` — outbound queue saturated; retry after a short delay.
  - `ErrClosed` — client closed.

`Connect` wraps handshake failures with `ErrHandshake`.

## Troubleshooting

  - `ffplay: command not found` — install via `brew install ffmpeg`.
  - macOS microphone prompt — granted once via TCC; re-run after granting.
  - 401 Unauthorized — verify `OPENAI_API_KEY`. The Authorization header is
    redacted from `WithDumpFrames` output.

## Examples

  - `examples/text-only/` — minimal text round trip.
  - `examples/voice-loopback/` — darwin-only mic-in to audio-out.
  - Package examples: `go doc github.com/tmc/misc/oairt`.

## Versioning

Semantic Versioning. See [CHANGELOG.md](CHANGELOG.md). Release process:
[RELEASING.md](RELEASING.md).

## Dependencies

oairt depends on [`gorilla/websocket`](https://github.com/gorilla/websocket)
v1.4.2 (archived-but-stable, zero open CVEs). A `v0.2.0` release will
evaluate migration to [`coder/websocket`](https://github.com/coder/websocket).

## License

See [LICENSE](LICENSE).
