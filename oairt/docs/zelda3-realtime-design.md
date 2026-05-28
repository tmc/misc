# Zelda3 Realtime Controller Design

This document describes `cmd/zelda3-realtime`, a local controller that connects
OpenAI Realtime to the Zelda3 MCP server. The goal is a hackable voice loop that
can observe the game, narrate what is happening, and issue bounded controller
inputs through MCP.

## Goals

- Use Realtime voice for low-latency narration and operator interaction.
- Let the model control Zelda3 through a small, reversible MCP tool set.
- Feed the model enough game state to act: compact observations every turn and
  screenshots when visual context matters.
- Keep the implementation local and debuggable by default, with a hosted MCP
  path available for a tunnel-backed demo.

## Existing Surfaces

Zelda3 already exposes the control surface needed for a first pass.

- `zelda3 --mcp-http 8123` starts an HTTP MCP server by default.
- `GET /health` verifies that the server is alive.
- `POST /mcp` accepts MCP JSON-RPC over streamable HTTP.
- `GET /openai/realtime-session` returns a Realtime session template with a
  Zelda3 MCP tool declaration.
- `GET /stream/frame.rgba?once=1` and `GET /stream/wram?once=1` provide direct
  frame and memory snapshots for local adapters.

The relevant MCP tools are:

- `observe`: return a compact textual game observation.
- `run_input`: press buttons for a bounded number of frames, optionally wait,
  release, and return a fresh observation.
- `set_buttons`: hold or release a named button set.
- `release_all_inputs`: clear all controller inputs.
- `step_frame`: advance deterministic frames.
- `get_frame`: return a PNG screenshot as MCP image content.
- `get_game_state`, `read_memory`, `capture_snapshot`, and `restore_snapshot`
  for diagnostics and recovery.

The default Realtime controller should not expose memory writes, teleports, item
grants, or other high-level cheats unless a debugging flag explicitly opts in.

## Architecture

`cmd/zelda3-realtime` is a separate process. It does not embed the game. It
talks to the local game over HTTP and to OpenAI over the Realtime WebSocket.

```text
microphone/stdin
      |
      v
cmd/zelda3-realtime  <------ audio/text/image/tool events ------> OpenAI Realtime
      |
      | MCP JSON-RPC over HTTP
      v
zelda3 --mcp-http 8123  -----> game loop, frame buffer, WRAM
```

This shape gives us a useful failure boundary: if Realtime disconnects, the game
continues running and the controller can release inputs before exit.

The first UI can be command-line only. A SwiftUI front end should sit beside the
controller rather than replace it: show the live frame, transcript, current
observation, model intent, active tool call, and manual release/pause controls.
The SwiftUI app should talk to a local controller endpoint, not directly hold an
OpenAI API key unless it is using a short-lived Realtime client secret.

## Startup Flow

1. Parse flags:
   - `-mcp-url http://127.0.0.1:8123/mcp`
   - `-model gpt-realtime-2`
   - `-voice <voice>`
   - `-effort minimal|low|medium|high|xhigh`
   - `-vision every-turn|on-demand|off`
   - `-mode local-tools|hosted-mcp`
2. Call `GET /health`.
3. In local-tools mode, call MCP `initialize`, then `tools/list`, and verify the
   required tools exist.
4. Connect to Realtime with `gpt-realtime-2`.
5. Send `session.update` with instructions, voice/audio settings, reasoning
   effort, and the tool strategy for the chosen mode.
6. In hosted MCP mode, wait for `mcp_list_tools.completed` or a
   `conversation.item.done` item of type `mcp_list_tools` before starting a turn
   that depends on Zelda3 tools.
7. Send an initial `observe` result as text. If vision is enabled, also send the
   current frame as an `input_image` content part with `image_url`.
8. Start the audio, event, and control loops.

## Tool Strategy

There are two useful modes. Keep both in the design because they optimize for
different constraints.

`hosted-mcp` is the fastest hack-demo path if we have a secure tunnel and are
comfortable exposing the selected tools. Zelda3 already has
`/openai/realtime-session`, which emits a Realtime `type: "mcp"` tool
declaration. In this mode OpenAI imports the MCP tools and runs them. The
`server_url` must be reachable by the OpenAI service; a loopback URL will not
work from the hosted service path.

`local-tools` is the safer local path. In this mode `cmd/zelda3-realtime`
advertises ordinary Realtime function tools, receives tool-call events from the
model, calls the local MCP server itself, and sends function outputs back to
Realtime. This requires a small MCP JSON-RPC adapter, but it avoids exposing the
game server through a public tunnel and lets us enforce stricter local policy.

Default to `local-tools` for repeatable development. Use `hosted-mcp` for a hack
demo only when the tunnel is authenticated or single-use and the allowed tool
list is narrow.

The default allowed action tools should be:

- `observe`
- `run_input`
- `set_buttons`
- `release_all_inputs`
- `get_frame`
- `capture_snapshot`
- `restore_snapshot`

`run_input` should be preferred over long-lived `set_buttons` calls. It is
atomic enough for narration: apply buttons, step a bounded frame count, release,
optionally wait, and return a new observation.

## Visual Input

Realtime can accept image inputs, but it does not provide a continuous video
stream primitive. The controller should sample game frames deliberately.

The first implementation should support two paths:

- `get_frame` via MCP, using the returned PNG image content.
- `/stream/frame.rgba?once=1`, converted locally to PNG when the MCP image
  result is not convenient for Realtime input events.

Frames should be sent as `conversation.item.create` user messages containing an
`input_image` content part whose `image_url` is a Base64 data URI. The
controller should throttle these events. A good default is on-demand screenshots
after `observe` says the room/module changed, after a failed action, or when the
model asks for visual context.

## Prompt Shape

The system instructions should be short and operational:

- You are narrating and playing Zelda3.
- Speak naturally, like a calm co-pilot.
- First observe, then choose one bounded action.
- Prefer `run_input` with 1-30 frames.
- Always release inputs after held-button plans or errors.
- Ask for a screenshot only when state text is not enough.
- Do not use memory writes or high-level cheat tools in normal play.

Use `reasoning.effort=low` by default. Raise it only for puzzle-solving modes.
For live play, lower latency matters more than deep deliberation.

Use a short preamble at session start that establishes domain understanding:
button names, the meaning of `run_input`, what counts as a bounded action, and
the narration style. Keep longer game notes in ordinary conversation context and
refresh them when a new dungeon, room, or mode is detected.

## Controller Loop

The controller needs a single serialized game-action path. Audio and Realtime
events can be concurrent, but MCP calls that mutate inputs should run through
one goroutine.

1. Receive Realtime events.
2. For local tools, wait for `response.function_call_arguments.done` and parse
   the complete arguments payload. Ignore partial deltas unless they are useful
   for logging.
3. For hosted MCP, handle `response.mcp_call_arguments.done`,
   `response.mcp_call.in_progress`, `response.mcp_call.failed`, and
   `response.output_item.done` items of type `mcp_call`.
4. If hosted MCP approval is enabled, answer `mcp_approval_request` items by
   sending a `conversation.item.create` event containing an
   `mcp_approval_response` item before the tool call can continue. The first
   demo should use `require_approval: "never"` only on a protected tunnel.
5. For local tools, validate the requested tool and arguments before calling
   MCP. For hosted MCP, rely on the Realtime MCP import/tool policy and watch
   failure events.
6. Execute local MCP calls with a short timeout.
7. On timeout, validation error, disconnect, or cancellation, call
   `release_all_inputs`.
8. Send the tool result back to Realtime when using local tools.
9. Trigger follow-up `response.create` only when the client owns response timing:
   either VAD is disabled with `turn_detection: null`, or VAD remains enabled
   with automatic response creation disabled by setting
   `turn_detection.create_response=false` and
   `turn_detection.interrupt_response=false`. If the live session schema rejects
   those fields, fall back to `turn_detection: null`.

`observe` and `get_frame` can be read-only, but they should still be rate
limited to avoid crowding the Realtime context with repeated state.

Do not rely on parallel tool calls for controller mutation. Parallel read-only
observations are useful, but button mutations should be serialized so a
late-arriving tool result cannot leave contradictory buttons held.

## oairt Changes Needed

The current `oairt` client already has the core WebSocket connection, audio
streaming, Realtime event dispatch, `gpt-realtime-2` defaulting in the CLI, and a
`reasoning.effort` field. `cmd/zelda3-realtime` needs a small API expansion:

- Add image content fields to `ItemContent`, including `image_url`.
- Add typed helpers for function call completion and function output items.
- Add constants and typed handling for hosted MCP events:
  `mcp_list_tools.*`, `response.mcp_call_arguments.done`,
  `response.mcp_call.in_progress`, `response.mcp_call.failed`, and
  `mcp_approval_request` / `mcp_approval_response` items.
- Add `interrupt_response` and `create_response` turn-detection fields, or allow
  raw session JSON for this mode.
- Add support for the current nested session shape (`audio.input`,
  `audio.output`, and `audio.input.turn_detection`) while preserving the legacy
  top-level fields until the package API is cleaned up.
- Add MCP tool schema fields or keep hosted MCP as raw JSON until the API is
  stable.
- Keep `Event.Raw` available so new Realtime events can be handled before every
  field is modeled.
- Add a local MCP client package or internal helper that supports initialize,
  tools/list, and tools/call over streamable HTTP.
- Add optional Realtime client-secret minting only for browser/WebRTC or SwiftUI
  flows that should not hold the long-lived API key.

For the first hack build, raw JSON is acceptable at the Realtime edges if the
typed additions would slow the demo down.

## Feature Map

- WebRTC: needed for browser or SwiftUI media capture paths. The Go CLI can use
  WebSocket first.
- Preamble: keep a compact session instruction block and refresh domain context
  as game mode changes.
- Controllable reasoning: expose `-effort`, default to `low`, and use higher
  effort only for puzzles.
- Parallel tool calling: allow read-only observation in parallel, but serialize
  controller mutations.
- Domain understanding: seed Zelda3 button/action vocabulary and feed compact
  WRAM-derived observations every turn.
- Context over turns: keep a rolling summary of objective, room, previous action,
  and last failed action; do not resend every frame.
- Natural voice: default to a current high-quality Realtime voice and keep
  output short during active play.
- Controllable tone: expose a `-tone` flag that changes prompt style without
  changing tool policy.

## Safety And Cleanup

- Bind Zelda3 MCP to loopback for the default demo.
- Treat `server_url` in hosted MCP mode as public surface; do not point it at a
  writable game server without a tunnel/auth story.
- If a public tunnel is used, protect it. A bare Zelda3 MCP URL can control the
  game and read state.
- Reject `frames` above a low default cap in the controller even though the MCP
  server accepts larger values.
- Call `release_all_inputs` on cancellation, Realtime disconnect, tool error,
  or process exit.
- Keep memory-writing and high-level mutation tools behind explicit flags.
- Prefer snapshots before risky autonomy modes.

## Test Plan

- Unit-test the MCP client against `httptest.Server`.
- Unit-test Realtime tool-call handling with `internal/mockrt`.
- Add a script test for `cmd/zelda3-realtime -help`.
- Add a fake MCP server test that verifies `run_input` calls are serialized and
  that cancellation triggers `release_all_inputs`.
- Run a smoke test against a live Zelda3 instance:

```sh
zelda3 --mcp-http 8123 --pause-on-start
go run ./cmd/zelda3-realtime -mcp-url http://127.0.0.1:8123/mcp -model gpt-realtime-2 -effort low
```

- Run a hosted-MCP smoke test only with a protected tunnel:

```sh
curl -o /tmp/zelda3-realtime-session.json 'http://127.0.0.1:8123/openai/realtime-session?model=gpt-realtime-2&approval=never&server_url=https://example-tunnel/mcp'
go run ./cmd/zelda3-realtime -mode hosted-mcp -session-template /tmp/zelda3-realtime-session.json
```

## Open Questions

- Whether OpenAI-hosted Realtime MCP will consume image content returned by MCP
  tools in the way we need. If not, the local controller should send screenshots
  separately as `input_image` content parts with `image_url` data URIs.
- Whether the Zelda3 MCP HTTP server should add an auth token before any public
  tunnel demo.
- Whether the game control path needs a stricter single-threaded command queue
  in Zelda3 itself, or whether serializing calls in `cmd/zelda3-realtime` is
  enough for the hack.
- Whether the Realtime session template in Zelda3 should default to
  `gpt-realtime-2` instead of `gpt-realtime`.
