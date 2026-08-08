package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// minGapSeconds is the minimum spacing between selected frame times;
// closer scene changes (scrolling, cursor noise) collapse into one.
const minGapSeconds = 3

// frame is one extracted still and its position in the video.
type frame struct {
	Seconds float64
	Path    string
}

// fetchVideo downloads the video with yt-dlp (cached per video) capped
// at 720p, merged to mp4.
func fetchVideo(videoURL, id string) (string, error) {
	path := filepath.Join(cacheDir(), id+".mp4")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	cmd := exec.Command("yt-dlp", "--no-playlist",
		"-f", "bv*[height<=720]+ba/b[height<=720]",
		"--merge-output-format", "mp4",
		"-o", path, videoURL)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp: %w", err)
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("yt-dlp produced no %s", path)
	}
	return path, nil
}

var ptsTimeRe = regexp.MustCompile(`pts_time:([0-9.]+)`)

// frameTimes returns the seconds at which to grab frames: ffmpeg
// scene-change times (threshold scene, if > 0) merged with a fixed grid
// every grid seconds (if > 0), deduplicated to minGapSeconds spacing.
func frameTimes(video string, scene float64, grid int) ([]float64, error) {
	var times []float64
	if scene > 0 {
		filter := fmt.Sprintf("select='gt(scene,%g)',showinfo", scene)
		cmd := exec.Command("ffmpeg", "-i", video, "-vf", filter, "-f", "null", "-")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("ffmpeg scene detect: %w", err)
		}
		for _, m := range ptsTimeRe.FindAllStringSubmatch(string(out), -1) {
			t, err := strconv.ParseFloat(m[1], 64)
			if err == nil {
				times = append(times, t)
			}
		}
	}
	if grid > 0 {
		dur, err := duration(video)
		if err != nil {
			return nil, err
		}
		for t := float64(grid) / 2; t < dur; t += float64(grid) {
			times = append(times, t)
		}
	}
	return dedupe(times, minGapSeconds), nil
}

// dedupe sorts times and drops any within gap seconds of the previous
// kept time.
func dedupe(times []float64, gap float64) []float64 {
	sort.Float64s(times)
	var out []float64
	for _, t := range times {
		if len(out) == 0 || t-out[len(out)-1] >= gap {
			out = append(out, t)
		}
	}
	return out
}

func duration(video string) (float64, error) {
	out, err := exec.Command("ffprobe", "-v", "error",
		"-show_entries", "format=duration", "-of", "csv=p=0", video).Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe: %w", err)
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

// extractFrames grabs one JPEG per time, scaled to 1024px wide.
func extractFrames(video string, times []float64, dir string) ([]frame, error) {
	var frames []frame
	for _, t := range times {
		path := filepath.Join(dir, fmt.Sprintf("f%08.1f.jpg", t))
		cmd := exec.Command("ffmpeg", "-v", "error",
			"-ss", fmt.Sprintf("%.1f", t), "-i", video,
			"-frames:v", "1", "-vf", "scale=1024:-2", "-q:v", "5",
			path, "-y")
		if err := cmd.Run(); err != nil {
			continue // times near EOF may yield nothing
		}
		if _, err := os.Stat(path); err == nil {
			frames = append(frames, frame{Seconds: t, Path: path})
		}
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frames extracted")
	}
	return frames, nil
}

// redactFaces blurs faces in each frame in place using imgredact.
// NotebookLM's image pipeline silently drops PDF images that contain
// human faces (e.g. a presenter webcam overlay), so unredacted frames
// contribute no visual information to a notebook.
func redactFaces(frames []frame) error {
	if _, err := exec.LookPath("imgredact"); err != nil {
		return fmt.Errorf("imgredact not on PATH")
	}
	for _, f := range frames {
		png := strings.TrimSuffix(f.Path, ".jpg") + ".png"
		cmd := exec.Command("imgredact", "-faces", "-text=false", "-o", png, f.Path)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("imgredact %s: %w", f.Path, err)
		}
		// The PDF writer embeds JPEG (DCTDecode), so convert back.
		cmd = exec.Command("ffmpeg", "-v", "error", "-i", png, "-q:v", "5", f.Path, "-y")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("ffmpeg png to jpeg %s: %w", png, err)
		}
		os.Remove(png)
	}
	return nil
}
