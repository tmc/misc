// Nlmvideo turns a video into NotebookLM-ready sources: a timestamped
// transcript and a frames PDF with a searchable text layer.
//
// NotebookLM refuses unlisted, private, and caption-less YouTube videos,
// and its converter errors on image-only PDFs. Nlmvideo works around
// both: it downloads the video and captions locally with yt-dlp, picks
// frame times by ffmpeg scene detection merged with a baseline grid,
// and writes a PDF whose pages pair each frame with real text — the
// timestamp, a ?t= deep link, and the words spoken around that moment.
// The companion yt-timestamps markdown source (one line per bucket,
// deep-linked) matches the format nlmclips looks for, so the two tools
// compose.
//
// Usage:
//
//	nlmvideo [flags] <video-url>
//
//	-frames        write the frames PDF (default with -timestamps)
//	-timestamps    write yt-timestamps markdown (default with -frames)
//	-notebook id   also upload the artifacts as sources via nlm
//	-scene 0.18    ffmpeg scene-change threshold (0 disables)
//	-grid 20       baseline sampling interval in seconds (0 disables)
//	-lang en       caption language
//	-o dir         output directory (default ".")
//	-redact-faces  blur faces in frames with imgredact (default true)
//
// Face redaction matters because NotebookLM silently drops PDF images
// that contain human faces; a presenter webcam overlay is enough to
// make every frame invisible to notebook chat.
//
// Requires yt-dlp and ffmpeg on PATH; nlm too when -notebook is given,
// and imgredact for -redact-faces.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

func main() {
	var (
		frames     = flag.Bool("frames", false, "write the frames PDF")
		timestamps = flag.Bool("timestamps", false, "write yt-timestamps markdown")
		notebook   = flag.String("notebook", "", "upload artifacts to this notebook via nlm")
		scene      = flag.Float64("scene", 0.18, "ffmpeg scene-change threshold (0 disables)")
		grid       = flag.Int("grid", 20, "baseline sampling interval in seconds (0 disables)")
		lang       = flag.String("lang", "en", "caption language")
		outDir     = flag.String("o", ".", "output directory")
		redact     = flag.Bool("redact-faces", true, "blur faces in frames with imgredact (required for NotebookLM image ingestion)")
	)
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: nlmvideo [flags] <video-url>\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	if !*frames && !*timestamps {
		*frames, *timestamps = true, true
	}
	if err := run(flag.Arg(0), *frames, *timestamps, *notebook, *scene, *grid, *lang, *outDir, *redact); err != nil {
		fmt.Fprintf(os.Stderr, "nlmvideo: %v\n", err)
		os.Exit(1)
	}
}

func run(videoURL string, frames, timestamps bool, notebook string, scene float64, grid int, lang, outDir string, redact bool) error {
	id, err := videoID(videoURL)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	words, err := fetchCaptions(videoURL, id, lang)
	if err != nil {
		return err
	}
	buckets := bucketWords(words, bucketSeconds)

	var outputs []string
	if timestamps {
		path := filepath.Join(outDir, "yt-timestamps-"+id+".md")
		if err := writeTimestamps(path, id, buckets); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "nlmvideo: wrote %s (%d lines)\n", path, len(buckets))
		outputs = append(outputs, path)
	}
	if frames {
		video, err := fetchVideo(videoURL, id)
		if err != nil {
			return err
		}
		times, err := frameTimes(video, scene, grid)
		if err != nil {
			return err
		}
		dir, err := os.MkdirTemp("", "nlmvideo-frames-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		jpegs, err := extractFrames(video, times, dir)
		if err != nil {
			return err
		}
		if redact {
			if err := redactFaces(jpegs); err != nil {
				fmt.Fprintf(os.Stderr, "nlmvideo: face redaction skipped: %v (NotebookLM will drop face-bearing images)\n", err)
			}
		}
		mdPath, err := writeFramesMarkdown(outDir, id, jpegs, buckets)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "nlmvideo: wrote %s (%d frames)\n", mdPath, len(jpegs))
		pdfPath := filepath.Join(outDir, "frames-"+id+".pdf")
		if err := renderPDF(outDir, mdPath, pdfPath, id, jpegs, buckets); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "nlmvideo: wrote %s\n", pdfPath)
		outputs = append(outputs, pdfPath)
	}
	for _, path := range outputs {
		if notebook == "" {
			continue
		}
		name := filepath.Base(path)
		cmd := exec.Command("nlm", "source", "add", "--name", name, notebook, path)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("nlm source add %s: %w", path, err)
		}
	}
	return nil
}

var videoIDRe = regexp.MustCompile(`(?:v=|youtu\.be/|/shorts/)([A-Za-z0-9_-]{11})`)

// videoID extracts the 11-character YouTube video ID from a URL.
func videoID(url string) (string, error) {
	m := videoIDRe.FindStringSubmatch(url)
	if m == nil {
		return "", fmt.Errorf("no YouTube video ID in %q", url)
	}
	return m[1], nil
}
