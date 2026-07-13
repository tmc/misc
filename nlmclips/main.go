// Nlmclips asks a NotebookLM notebook about a YouTube video and turns the
// answer into a note whose key clips deep-link into the video with ?t=
// timestamps.
//
// NotebookLM indexes YouTube sources as plain transcript text with no
// timing information, so its citations cannot say where in the video a
// clip lives. Nlmclips closes that gap: it fetches the video's timed
// captions with yt-dlp, uploads a compact timestamped transcript as an
// extra source, asks the question with instructions to anchor each clip
// to a transcript line and a verbatim quote, then re-aligns every quote
// against the word-level caption timings to make each link second-exact.
//
// Usage:
//
//	nlmclips [flags] <notebook-id> <question>
//
// The flags are:
//
//	-source regexp
//		title or ID pattern selecting the YouTube source
//		(default: the notebook's only YouTube source)
//	-lang code
//		caption language passed to yt-dlp (default "en")
//	-note title
//		create a NotebookLM note with this title
//		(default "Key clips: <video title>"; empty with -print skips)
//	-print
//		print the final markdown to stdout instead of creating a note
//	-lead seconds
//		rewind each matched quote by this many seconds (default 2)
//	-min-score fraction
//		minimum word-match fraction to trust a local alignment;
//		below it the model's own line-start timestamp is kept
//		(default 0.8)
//
// Nlmclips shells out to nlm (github.com/tmc/nlm) and yt-dlp, which must
// both be on PATH and authenticated/installed.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	var (
		sourcePat = flag.String("source", "", "title or ID regexp selecting the YouTube source")
		lang      = flag.String("lang", "en", "caption language for yt-dlp")
		noteTitle = flag.String("note", "", "title for the created note (default derived from video)")
		printOnly = flag.Bool("print", false, "print markdown to stdout instead of creating a note")
		lead      = flag.Int("lead", 2, "seconds of lead-in before each matched quote")
		minScore  = flag.Float64("min-score", 0.8, "minimum alignment score to rewrite a timestamp")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: nlmclips [flags] <notebook-id> <question>\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}
	notebook, question := flag.Arg(0), flag.Arg(1)

	if err := run(notebook, question, *sourcePat, *lang, *noteTitle, *printOnly, *lead, *minScore); err != nil {
		fmt.Fprintf(os.Stderr, "nlmclips: %v\n", err)
		os.Exit(1)
	}
}

func run(notebook, question, sourcePat, lang, noteTitle string, printOnly bool, lead int, minScore float64) error {
	src, err := findYouTubeSource(notebook, sourcePat)
	if err != nil {
		return err
	}
	videoURL, videoID, err := videoRef(notebook, src.ID)
	if err != nil {
		return fmt.Errorf("resolve video url for source %s: %w", src.ID, err)
	}
	fmt.Fprintf(os.Stderr, "nlmclips: video %s (%s)\n", videoID, src.Title)

	words, err := fetchCaptions(videoURL, lang)
	if err != nil {
		return fmt.Errorf("fetch captions: %w", err)
	}
	fmt.Fprintf(os.Stderr, "nlmclips: %d caption words\n", len(words))

	if err := ensureTranscriptSource(notebook, videoID, words); err != nil {
		return fmt.Errorf("upload timestamped transcript: %w", err)
	}

	fmt.Fprintln(os.Stderr, "nlmclips: asking notebook...")
	answer, err := ask(notebook, question, videoID)
	if err != nil {
		return fmt.Errorf("generate chat: %w", err)
	}

	refined, report := refineLinks(answer, words, videoID, lead, minScore)
	for _, line := range report {
		fmt.Fprintf(os.Stderr, "nlmclips: %s\n", line)
	}

	if printOnly {
		fmt.Println(refined)
		return nil
	}
	if noteTitle == "" {
		noteTitle = "Key clips: " + src.Title
	}
	if err := createNote(notebook, noteTitle, refined); err != nil {
		return fmt.Errorf("create note: %w", err)
	}
	fmt.Fprintf(os.Stderr, "nlmclips: created note %q\n", noteTitle)
	return nil
}

// source is one row of nlm source list --json.
type source struct {
	ID    string `json:"source_id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// findYouTubeSource picks the notebook's YouTube source, filtered by an
// optional title/ID regexp. It errors when zero or several match so the
// caller never guesses between videos.
func findYouTubeSource(notebook, pattern string) (source, error) {
	out, err := nlm("source", "list", notebook, "--json")
	if err != nil {
		return source{}, err
	}
	var re *regexp.Regexp
	if pattern != "" {
		re, err = regexp.Compile(pattern)
		if err != nil {
			return source{}, fmt.Errorf("bad -source pattern: %w", err)
		}
	}
	var matches []source
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var s source
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			continue
		}
		if s.Type != "youtube" {
			continue
		}
		if re != nil && !re.MatchString(s.Title) && !re.MatchString(s.ID) {
			continue
		}
		matches = append(matches, s)
	}
	switch len(matches) {
	case 0:
		return source{}, fmt.Errorf("no YouTube source in notebook %s (pattern %q)", notebook, pattern)
	case 1:
		return matches[0], nil
	}
	var titles []string
	for _, m := range matches {
		titles = append(titles, fmt.Sprintf("%s (%s)", m.Title, m.ID))
	}
	return source{}, fmt.Errorf("multiple YouTube sources; disambiguate with -source:\n  %s", strings.Join(titles, "\n  "))
}

var watchURLRE = regexp.MustCompile(`https://www\.youtube\.com/watch\?v(?:=|\\u003d)([\w-]{6,})`)

// videoRef extracts the watch URL and video ID from the raw LoadSource
// response, the only place NotebookLM reports a source's YouTube URL.
func videoRef(notebook, sourceID string) (url, id string, err error) {
	out, err := nlm("dump-load-source", sourceID, notebook)
	if err != nil {
		return "", "", err
	}
	m := watchURLRE.FindStringSubmatch(out)
	if m == nil {
		return "", "", fmt.Errorf("no youtube watch URL in LoadSource response")
	}
	id = m[1]
	return "https://www.youtube.com/watch?v=" + id, id, nil
}

// transcriptSourceName names the uploaded timestamped transcript so
// repeat runs against the same video find and reuse it.
func transcriptSourceName(videoID string) string {
	return "yt-timestamps: " + videoID
}

// ensureTranscriptSource uploads the timestamped transcript unless a
// source with its name already exists in the notebook.
func ensureTranscriptSource(notebook, videoID string, words []word) error {
	name := transcriptSourceName(videoID)
	out, err := nlm("source", "list", notebook, "--json")
	if err != nil {
		return err
	}
	if strings.Contains(out, fmt.Sprintf("%q", name)) {
		fmt.Fprintf(os.Stderr, "nlmclips: transcript source %q already present\n", name)
		return nil
	}
	transcript := renderTranscript(words, videoID)
	cmd := exec.Command("nlm", "source", "add", notebook, "-", "--name", name)
	cmd.Stdin = strings.NewReader(transcript)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nlm source add: %w", err)
	}
	fmt.Fprintf(os.Stderr, "nlmclips: uploaded transcript source %q (%d bytes)\n", name, len(transcript))
	return nil
}

// askInstructions is appended to the user's question so the model anchors
// every clip to the timestamped transcript in a shape refineLinks can
// post-process.
const askInstructions = `

For each key clip, use the "yt-timestamps" transcript source to find where the clip starts, and include:
- a markdown link of the form [h:mm:ss](https://www.youtube.com/watch?v=%s&t=SECONDSs) using the exact t= value from the transcript line where the clip begins
- a short verbatim quote (8-25 words) copied exactly from the transcript, as a blockquote

Format the answer as a clean markdown note.`

func ask(notebook, question, videoID string) (string, error) {
	prompt := question + fmt.Sprintf(askInstructions, videoID)
	cmd := exec.Command("nlm", "generate-chat", "-f", "-", notebook)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("nlm generate-chat: %w", err)
	}
	return string(out), nil
}

func createNote(notebook, title, content string) error {
	cmd := exec.Command("nlm", "note", "create", notebook, title)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nlm note create: %w", err)
	}
	return nil
}

func nlm(args ...string) (string, error) {
	cmd := exec.Command("nlm", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("nlm %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// cacheDir returns a per-user cache directory for downloaded captions.
func cacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "nlmclips")
	os.MkdirAll(dir, 0o755)
	return dir
}
