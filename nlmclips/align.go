package main

import (
	"fmt"
	"regexp"
	"strings"
)

// normalize lowercases a token and strips everything but letters and
// digits, so caption tokens and quote tokens compare loosely.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// alignQuote finds the caption window best matching quote and returns the
// start time in seconds plus the fraction of quote tokens that matched.
// Matching is position-aligned over normalized tokens, which is exact for
// verbatim quotes and degrades (low score) for paraphrases.
func alignQuote(quote string, words []word) (seconds int, score float64) {
	var q []string
	for _, tok := range strings.Fields(quote) {
		if n := normalize(tok); n != "" {
			q = append(q, n)
		}
	}
	if len(q) == 0 || len(words) < len(q) {
		return 0, 0
	}
	stream := make([]string, len(words))
	for i, w := range words {
		stream[i] = normalize(w.Text)
	}
	bestMatches, bestAt := -1, 0
	for i := 0; i+len(q) <= len(stream); i++ {
		matches := 0
		for j, tok := range q {
			if stream[i+j] == tok {
				matches++
			}
		}
		if matches > bestMatches {
			bestMatches, bestAt = matches, i
		}
	}
	return words[bestAt].Ms / 1000, float64(bestMatches) / float64(len(q))
}

// clipLinkRE matches the [label](watch?v=ID&t=Ns) links the ask
// instructions request; the video ID is interpolated per run.
func clipLinkRE(videoID string) *regexp.Regexp {
	return regexp.MustCompile(`\[([^\]]+)\]\(https://www\.youtube\.com/watch\?v=` +
		regexp.QuoteMeta(videoID) + `&t=(\d+)s\)`)
}

// quoteRE grabs quoted spans long enough to align reliably: straight or
// curly double quotes around at least 20 characters.
var quoteRE = regexp.MustCompile(`["\x{201c}]([^"\x{201c}\x{201d}]{20,400})["\x{201d}]`)

// blockquoteRE matches markdown blockquote lines, the shape the ask
// instructions request for each clip's verbatim quote.
var blockquoteRE = regexp.MustCompile(`(?m)^\s*>\s*(.{20,400})$`)

// citationMarkRE matches NotebookLM's inline citation markers ([4],
// [1, 2], [2-5]), which are not part of the quoted transcript text.
var citationMarkRE = regexp.MustCompile(`\[\d+(?:\s*[-,]\s*\d+)*\]`)

// quoteCandidates extracts a clip section's possible verbatim quotes,
// most trustworthy first: blockquote lines, then inline quoted spans
// (explainer prose often quotes short phrases too, so those come last).
func quoteCandidates(section string) []string {
	var out []string
	for _, m := range blockquoteRE.FindAllStringSubmatch(section, -1) {
		out = append(out, citationMarkRE.ReplaceAllString(m[1], ""))
	}
	for _, m := range quoteRE.FindAllStringSubmatch(section, -1) {
		out = append(out, citationMarkRE.ReplaceAllString(m[1], ""))
	}
	return out
}

// refineLinks rewrites each clip link's timestamp to the word-exact time
// of the verbatim quote that follows it in the answer. Links whose quote
// aligns below minScore keep the model's line-start value. It returns the
// rewritten markdown and a per-link report for the user.
func refineLinks(answer string, words []word, videoID string, lead int, minScore float64) (string, []string) {
	re := clipLinkRE(videoID)
	locs := re.FindAllStringSubmatchIndex(answer, -1)
	if len(locs) == 0 {
		return answer, []string{"no clip links found in answer; leaving text unchanged"}
	}
	var out strings.Builder
	var report []string
	prev := 0
	for i, loc := range locs {
		out.WriteString(answer[prev:loc[0]])
		prev = loc[1]

		// The text between this link and the next holds this clip's quote.
		sectionEnd := len(answer)
		if i+1 < len(locs) {
			sectionEnd = locs[i+1][0]
		}
		section := answer[loc[1]:sectionEnd]

		claimed := answer[loc[4]:loc[5]] // t= digits
		link := answer[loc[0]:loc[1]]
		candidates := quoteCandidates(section)
		bestScore := 0.0
		matched := false
		for _, q := range candidates {
			sec, score := alignQuote(q, words)
			if score > bestScore {
				bestScore = score
			}
			if score < minScore {
				continue
			}
			sec -= lead
			if sec < 0 {
				sec = 0
			}
			link = fmt.Sprintf("[%s](https://www.youtube.com/watch?v=%s&t=%ds)",
				hms(sec), videoID, sec)
			report = append(report, fmt.Sprintf("clip %d: t=%ss -> t=%ds (align %.2f)", i+1, claimed, sec, score))
			matched = true
			break
		}
		if !matched {
			if len(candidates) == 0 {
				report = append(report, fmt.Sprintf("clip %d: kept t=%ss (no quote found)", i+1, claimed))
			} else {
				report = append(report, fmt.Sprintf("clip %d: kept t=%ss (best align %.2f below %.2f)", i+1, claimed, bestScore, minScore))
			}
		}
		out.WriteString(link)
	}
	out.WriteString(answer[prev:])
	return out.String(), report
}

// hms formats seconds as h:mm:ss.
func hms(s int) string {
	return fmt.Sprintf("%d:%02d:%02d", s/3600, s%3600/60, s%60)
}
