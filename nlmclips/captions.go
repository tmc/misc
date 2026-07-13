package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// word is one caption token with its start time in milliseconds.
type word struct {
	Text string
	Ms   int
}

// json3 mirrors the fields of YouTube's json3 caption format that
// nlmclips needs: events carrying segments of utf8 text with offsets.
type json3 struct {
	Events []struct {
		TStartMs int `json:"tStartMs"`
		Segs     []struct {
			UTF8      string `json:"utf8"`
			TOffsetMs int    `json:"tOffsetMs"`
		} `json:"segs"`
	} `json:"events"`
}

// fetchCaptions downloads the video's captions with yt-dlp (cached per
// video and language) and returns the timed word stream. Manual subs are
// preferred; auto-generated subs are the fallback.
func fetchCaptions(videoURL, lang string) ([]word, error) {
	id := videoURL[strings.LastIndex(videoURL, "=")+1:]
	base := filepath.Join(cacheDir(), id+"-"+lang)
	path := base + "." + lang + ".json3"
	if _, err := os.Stat(path); err != nil {
		cmd := exec.Command("yt-dlp", "--skip-download",
			"--write-subs", "--write-auto-subs",
			"--sub-format", "json3", "--sub-langs", lang,
			"-o", base, videoURL)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("yt-dlp: %w", err)
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("yt-dlp produced no %s captions for %s", lang, videoURL)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseJSON3(data)
}

func parseJSON3(data []byte) ([]word, error) {
	var doc json3
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse json3: %w", err)
	}
	var words []word
	for _, ev := range doc.Events {
		for _, seg := range ev.Segs {
			text := strings.TrimSpace(seg.UTF8)
			if text == "" {
				continue
			}
			ms := ev.TStartMs + seg.TOffsetMs
			for _, tok := range strings.Fields(text) {
				words = append(words, word{Text: tok, Ms: ms})
			}
		}
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("no caption text in json3")
	}
	return words, nil
}

// transcriptLineSeconds is the target speech duration of one rendered
// transcript line. Coarse enough to keep the upload small, fine enough
// that a line-start timestamp is a usable fallback link.
const transcriptLineSeconds = 20

// renderTranscript renders the word stream as the timestamped transcript
// uploaded to NotebookLM. Every line starts with [h:mm:ss] and the exact
// t= value the model is told to copy into links.
func renderTranscript(words []word, videoID string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Timestamped transcript of YouTube video https://www.youtube.com/watch?v=%s\n", videoID)
	b.WriteString("Each line begins with [h:mm:ss] and the exact YouTube deep-link parameter (t=SECONDSs).\n")
	fmt.Fprintf(&b, "To link to a moment, use https://www.youtube.com/watch?v=%s&t=<seconds>s\n\n", videoID)

	lineStart := -1
	var line []string
	flush := func() {
		if len(line) == 0 {
			return
		}
		s := lineStart / 1000
		fmt.Fprintf(&b, "[%d:%02d:%02d] (t=%ds) %s\n", s/3600, s%3600/60, s%60, s, strings.Join(line, " "))
		line, lineStart = nil, -1
	}
	for _, w := range words {
		if lineStart < 0 {
			lineStart = w.Ms
		}
		line = append(line, w.Text)
		if w.Ms-lineStart > transcriptLineSeconds*1000 {
			flush()
		}
	}
	flush()
	return b.String()
}
