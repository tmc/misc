package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// writeFramesMarkdown writes the canonical frame-notes document: one
// section per frame with a deep-linked timestamp heading, the frame
// image, and the words spoken around that moment. Frame JPEGs are
// copied next to the markdown so the bundle is self-contained. The PDF
// output is a rendering of this same model.
func writeFramesMarkdown(dir, id string, frames []frame, buckets map[int]string) (string, error) {
	imgDir := filepath.Join(dir, "frames")
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		return "", err
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Video frames: %s\n\n", id)
	fmt.Fprintf(&sb, "One section per frame: timestamp deep link, still image, and the words spoken in that ~%ds window.\n", bucketSeconds)
	for _, f := range frames {
		base := filepath.Base(f.Path)
		if err := copyFile(f.Path, filepath.Join(imgDir, base)); err != nil {
			return "", err
		}
		secs := int(f.Seconds)
		fmt.Fprintf(&sb, "\n## [%d:%02d](https://youtu.be/%s?t=%d)\n\n", secs/60, secs%60, id, secs)
		fmt.Fprintf(&sb, "![frame at %d:%02d](frames/%s)\n\n", secs/60, secs%60, base)
		if text := spokenAt(buckets, f.Seconds); text != "" {
			fmt.Fprintf(&sb, "%s\n", text)
		}
	}
	path := filepath.Join(dir, "frames-"+id+".md")
	return path, os.WriteFile(path, []byte(sb.String()), 0o644)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
