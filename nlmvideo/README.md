# nlmvideo

Turn a video into NotebookLM-ready sources: a timestamped transcript and
a frames PDF with a searchable text layer.

NotebookLM refuses unlisted, private, and caption-less YouTube videos,
and its converter errors on image-only PDFs. nlmvideo works around both:

1. Downloads captions (json3, word-level timing) and the video itself
   with `yt-dlp` (cached).
2. Picks frame times by ffmpeg scene detection merged with a baseline
   sampling grid, deduplicated to 3s spacing.
3. Writes a self-contained markdown bundle — `frames-<id>.md` plus a
   `frames/` image directory — one section per frame: deep-linked
   timestamp heading, still image, and the words spoken around that
   moment. This is the canonical, reviewable output; render it with any
   markdown toolchain (pandoc, md2html).
4. Also renders `frames-<id>.pdf` from that markdown. When
   [`md2html`](https://github.com/tmc/md2html) and a PDF-capable
   [`churl`](https://github.com/tmc/cdp) are on PATH it renders through
   Chrome (md2html → HTML, churl `Page.printToPDF`) for real typography
   and clickable links; otherwise a built-in dependency-free writer is
   used. Either way every page carries a text layer, which is what makes
   NotebookLM accept it (its converter errors on image-only PDFs).
5. Writes `yt-timestamps-<id>.md`: one transcript line per ~20s with
   `?t=` deep links, in the format [nlmclips](../nlmclips) consumes —
   so nlmclips' deep-link Q&A works even for videos NotebookLM cannot
   ingest directly.

## Install

```sh
go install github.com/tmc/misc/nlmvideo@latest
```

Requires `yt-dlp` and `ffmpeg` on PATH; [`nlm`](https://github.com/tmc/nlm)
too when uploading.

## Usage

```sh
nlmvideo https://youtu.be/VIDEOID
nlmvideo -notebook <notebook-id> https://youtu.be/VIDEOID   # also upload
```

```
-frames        write the frames PDF (default with -timestamps)
-timestamps    write yt-timestamps markdown (default with -frames)
-notebook id   upload the artifacts as sources via nlm
-scene 0.18    ffmpeg scene-change threshold (0 disables)
-grid 20       baseline sampling interval in seconds (0 disables)
-lang en       caption language
-o dir         output directory
```
