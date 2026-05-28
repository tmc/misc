# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.1] - 2026-05-07

### Added
- `Session.Reasoning` (`*Reasoning` with `Effort` field) for
  `gpt-realtime-2` reasoning-effort control. Documented values:
  `"minimal"`, `"low"`, `"medium"`, `"high"`, `"xhigh"`.
- `AudioTranscription.Language`, `.Delay`, `.Prompt` (all `string`,
  `omitempty`) matching the expanded Realtime API transcription config.
- Event constants `EventConversationItemAdded`,
  `EventConversationItemDone`, and
  `EventConversationItemInputAudioTranscriptionDelta`.
- `Event.PreviousItemID` for the `previous_item_id` field on the new
  `conversation.item.added` / `conversation.item.done` events.
- Runnable example `Example_reasoningEffort` demonstrating
  `Session.Reasoning` against `gpt-realtime-2`.

## [0.1.0] - 2026-05-07

### Added
- Initial public release of the `oairt` library and `cmd/oairt` CLI.
- `Client` with `Connect`, `Close` (idempotent), `Send`, `SendAudio`, `On`.
- Functional options: `WithLogger`, `WithURL`, `WithUserAgent`,
  `WithHTTPClient`, `WithDialer`, `WithDebug`, `WithDumpFrames`.
- Sentinel errors `ErrNotConnected`, `ErrSendQueueFull`, `ErrHandshake`,
  `ErrClosed` (matchable with `errors.Is`).
- Typed Realtime events: 9 client→server and 24 server→client wire types
  exposed as `EventXxx` constants, with typed accessors `Event.TextDelta`
  and `Event.AudioDelta`.
- Round-trip and fuzz coverage in `events_test.go`, `events_fuzz_test.go`,
  `dispatch_test.go`.
- darwin streaming audio via `ffplay`; cross-platform text mode.
- Runnable examples under `examples/` and `example_test.go`.

### Changed
- Split former `package main` into library (`package oairt`) plus
  `cmd/oairt`.
- Removed local `replace` directives from `go.mod`.

### Security
- Redacted `Authorization` header from `WithDumpFrames` output.

[Unreleased]: https://github.com/tmc/misc/compare/oairt/v0.1.1...HEAD
[0.1.1]: https://github.com/tmc/misc/releases/tag/oairt/v0.1.1
[0.1.0]: https://github.com/tmc/misc/releases/tag/oairt/v0.1.0
