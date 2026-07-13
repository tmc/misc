# nlmclips

Ask a NotebookLM notebook about a YouTube video and get a note whose key
clips deep-link into the video with `?t=` timestamps.

NotebookLM indexes YouTube sources as plain transcript text, so its answers
can't say *where* in the video anything happens. nlmclips fixes that:

1. Finds the notebook's YouTube source and its video ID (via `nlm`).
2. Fetches the video's timed captions with `yt-dlp` (cached).
3. Uploads a compact timestamped transcript (`yt-timestamps: <video-id>`)
   as an extra source, one line per ~20s of speech.
4. Asks your question with instructions to anchor each clip to a transcript
   line (`[h:mm:ss](...&t=Ns)`) and a short verbatim quote.
5. Re-aligns each quote against the word-level caption timings and rewrites
   every link to the second-exact start of the quote.
6. Creates a NotebookLM note with the result (or prints it with `-print`).

## Install

```sh
go install github.com/tmc/misc/nlmclips@latest
```

Requires [`nlm`](https://github.com/tmc/nlm) (authenticated) and `yt-dlp`
on PATH.

## Usage

```sh
nlmclips <notebook-id> "Make a list of key clips about X and why they matter."
```

```
-source regexp   pick among multiple YouTube sources by title or ID
-lang code       caption language (default "en")
-note title      note title (default "Key clips: <video title>")
-print           print markdown to stdout instead of creating a note
-lead seconds    rewind links this far before each quote (default 2)
-min-score f     min word-match fraction to rewrite a timestamp (default 0.8)
```

Alignment reports go to stderr, e.g.:

```
nlmclips: clip 1: t=327s -> t=323s (align 1.00)
nlmclips: clip 2: kept t=2441s (align 0.62 below 0.80)
```

Links whose quote doesn't align cleanly keep the model's transcript-line
timestamp, which is accurate to ~20 seconds.
