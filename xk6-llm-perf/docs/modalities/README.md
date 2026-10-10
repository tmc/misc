# Modality request adapters

The authored `client.request` boundary supports audio transcription, image
creation/editing, image retrieval, and asynchronous video jobs. The fixture is
[`examples/modalities/smoke.js`](../../examples/modalities/smoke.js); run it with
`ENDPOINT_URL` pointing to a compatible endpoint. The real-binary HTTP fixture is
`TestBinaryModalities`, selected by `K6_SMOKE_BINARY`.

`multipart` replaces `body` for file uploads. Provide string form fields and
1–16 files with field name, filename, MIME type, and standard base64. Files are
decoded locally; the adapter never opens a path or fetches an input URL. Fields
are sorted, limited to 64 with 1 MiB per value, and the entire encoded request is
limited to 16 MiB. Filenames cannot contain path separators or header controls.
Audio uses `file`; image edits use `image`. Authored fields remain under caller
control, including model and response format.

`audio-transcription` accepts UTF-8 transcripts and JSON `text` responses.
`image` retains JSON images and SSE partial/final images. Image streams require
an explicit completed event or `[DONE]` and at least one image; partial images
are retained separately, with a maximum 1,024 events and 16 MiB. `retrieval`
retains a nonempty `data` array. These outputs preserve modality metadata and
never reinterpret pixels, vectors, audio seconds, or media bytes as tokens.
Provider-reported token usage can be retained; absent usage remains unavailable.
Image arrival events provide output timing, not token-first timing or ITL.

`video` requires `poll: {interval, maxPolls, download}`. The default interval is
1s and the default maximum is 120 polls. Allowed bounds are 10ms–1min and
1–1,000 polls. The original timeout/cancellation context covers submission,
polling, and optional download. Polling uses authenticated GET requests to the
submission path plus validated job ID. It accepts queued/in_progress/processing
and completed states, fails failed/cancelled/unknown states, and rejects identity
changes. Redirects are refused; provider URLs are never followed. Download
retains only byte count and SHA-256, rather than embedding video into the result.
Each response/download is limited to 16 MiB, and cumulative lifecycle responses
are limited to 64 MiB. The logical request covers the
complete lifecycle; polls do not become extra inference attempts.

This follows AIPerf revision `f10eb50a1776775d80d7f13396b0f668f14e2c03`,
`src/aiperf/endpoints/openai_audio_transcription.py`,
`openai_image_generation.py`, `openai_image_edit.py`,
`openai_video_generation.py`, and `nim_image_retrieval.py`. Local fixtures verify
transport contracts, limits, errors, and cancellation. They do not qualify a
particular provider's model, codec support, image quality, video quality, or
physical runtime performance.
