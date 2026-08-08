package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// bucketSeconds is the transcript bucket width. It matches the
// "yt-timestamps:" source format nlmclips consumes.
const bucketSeconds = 20

// word is one caption token with its start time in milliseconds.
type word struct {
	Text string
	Ms   int
}

// json3 mirrors the fields of YouTube's json3 caption format that
// nlmvideo needs: events carrying segments of utf8 text with offsets.
type json3 struct {
	Events []struct {
		TStartMs int `json:"tStartMs"`
		Segs     []struct {
			UTF8      string `json:"utf8"`
			TOffsetMs int    `json:"tOffsetMs"`
		} `json:"segs"`
	} `json:"events"`
}

func cacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	dir = filepath.Join(dir, "nlmvideo")
	os.MkdirAll(dir, 0o755)
	return dir
}

// fetchCaptions downloads the video's captions with yt-dlp (cached per
// video and language) and returns the timed word stream. Manual subs are
// preferred; auto-generated subs are the fallback.
func fetchCaptions(videoURL, id, lang string) ([]word, error) {
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

// bucketWords groups the word stream into fixed windows of width secs,
// keyed by window start second.
func bucketWords(words []word, secs int) map[int]string {
	byBucket := make(map[int][]string)
	for _, w := range words {
		b := (w.Ms / 1000 / secs) * secs
		byBucket[b] = append(byBucket[b], w.Text)
	}
	out := make(map[int]string, len(byBucket))
	for b, toks := range byBucket {
		out[b] = strings.Join(toks, " ")
	}
	return out
}

// writeTimestamps writes the yt-timestamps markdown source: one line per
// bucket with a ?t= deep link followed by the words spoken in it.
func writeTimestamps(path, id string, buckets map[int]string) error {
	keys := make([]int, 0, len(buckets))
	for b := range buckets {
		keys = append(keys, b)
	}
	sort.Ints(keys)
	var sb strings.Builder
	fmt.Fprintf(&sb, "# yt-timestamps: %s\n", id)
	fmt.Fprintf(&sb, "Each line: [m:ss](https://youtu.be/%s?t=SECONDS) then the words spoken in that ~%ds window.\n\n", id, bucketSeconds)
	for _, b := range keys {
		fmt.Fprintf(&sb, "[%d:%02d](https://youtu.be/%s?t=%d) %s\n", b/60, b%60, id, b, buckets[b])
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// spokenAt returns the bucket text covering second t.
func spokenAt(buckets map[int]string, t float64) string {
	return buckets[(int(t)/bucketSeconds)*bucketSeconds]
}
