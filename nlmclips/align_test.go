package main

import (
	"strings"
	"testing"
)

func wordsFrom(pairs ...any) []word {
	var out []word
	for i := 0; i < len(pairs); i += 2 {
		for _, tok := range strings.Fields(pairs[i].(string)) {
			out = append(out, word{Text: tok, Ms: pairs[i+1].(int)})
		}
	}
	return out
}

var testWords = wordsFrom(
	"welcome back to the number one podcast", 0,
	"our token costs are doubling every 45 days", 325_000,
	"and that is a real problem for enterprises", 332_000,
)

func TestAlignQuote(t *testing.T) {
	tests := []struct {
		name        string
		quote       string
		wantSeconds int
		minScore    float64
	}{
		{"verbatim", "token costs are doubling every 45 days", 325, 1.0},
		{"punctuation and case", "Token costs are DOUBLING every 45 days!", 325, 1.0},
		{"paraphrase scores low", "expenses have been growing fast lately recently", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sec, score := alignQuote(tt.quote, testWords)
			if tt.minScore >= 0.8 {
				if score < tt.minScore {
					t.Fatalf("score = %.2f, want >= %.2f", score, tt.minScore)
				}
				if sec != tt.wantSeconds {
					t.Errorf("seconds = %d, want %d", sec, tt.wantSeconds)
				}
			} else if score >= 0.8 {
				t.Errorf("score = %.2f, want < 0.8 for paraphrase", score)
			}
		})
	}
}

func TestQuoteCandidatesPrefersBlockquote(t *testing.T) {
	section := "\n> token costs are doubling every 45 days [4]\n\n" +
		`He also says "welcome back to the number one podcast" later on.`
	got := quoteCandidates(section)
	if len(got) != 2 {
		t.Fatalf("candidates = %v, want 2", got)
	}
	if want := "token costs are doubling every 45 days "; got[0] != want {
		t.Errorf("first candidate = %q, want %q (blockquote first, citation stripped)", got[0], want)
	}
}

func TestRefineLinksBlockquoteWinsOverInline(t *testing.T) {
	answer := "### Clip\n" +
		"[0:00:05](https://www.youtube.com/watch?v=vid123&t=5s)\n" +
		"> token costs are doubling every 45 days [4]\n\n" +
		`Explainer notes he said "welcome back to the number one podcast" earlier.` + "\n"
	got, _ := refineLinks(answer, testWords, "vid123", 0, 0.8)
	if want := "&t=325s"; !strings.Contains(got, want) {
		t.Errorf("blockquote (t=325s) should win over inline quote (t=0s):\n%s", got)
	}
}

func TestRefineLinks(t *testing.T) {
	answer := "### Clip 1\n" +
		"[0:05:20](https://www.youtube.com/watch?v=vid123&t=320s)\n" +
		"> \"token costs are doubling every 45 days\"\n" +
		"### Clip 2\n" +
		"[0:00:00](https://www.youtube.com/watch?v=vid123&t=0s)\n" +
		"> \"something the video never actually says here\"\n"
	got, report := refineLinks(answer, testWords, "vid123", 2, 0.8)
	if want := "[0:05:23](https://www.youtube.com/watch?v=vid123&t=323s)"; !strings.Contains(got, want) {
		t.Errorf("refined output missing %q:\n%s", want, got)
	}
	if want := "&t=0s"; !strings.Contains(got, want) {
		t.Errorf("low-score link should keep original t=0s:\n%s", got)
	}
	if len(report) != 2 {
		t.Errorf("report entries = %d, want 2: %v", len(report), report)
	}
}

func TestRenderTranscript(t *testing.T) {
	got := renderTranscript(testWords, "vid123")
	for _, want := range []string{
		"watch?v=vid123",
		"[0:00:00] (t=0s) welcome back",
		"[0:05:25]", // second line starts at the 325s group
	} {
		if !strings.Contains(got, want) {
			t.Errorf("transcript missing %q:\n%s", want, got)
		}
	}
}

func TestParseJSON3(t *testing.T) {
	data := []byte(`{"events":[
		{"tStartMs":1000,"segs":[{"utf8":"hello "},{"utf8":"world","tOffsetMs":500}]},
		{"tStartMs":3000,"segs":[{"utf8":"\n"}]},
		{"tStartMs":4000,"segs":[{"utf8":"again"}]}
	]}`)
	words, err := parseJSON3(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []word{{"hello", 1000}, {"world", 1500}, {"again", 4000}}
	if len(words) != len(want) {
		t.Fatalf("words = %v, want %v", words, want)
	}
	for i := range want {
		if words[i] != want[i] {
			t.Errorf("words[%d] = %v, want %v", i, words[i], want[i])
		}
	}
}
