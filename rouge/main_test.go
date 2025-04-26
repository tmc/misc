// main_test.go
package main

import (
	"math"
	"reflect"
	"testing"
)

const floatTolerance = 1e-9

// Helper function to compare RougeScores with tolerance
func scoresEqual(s1, s2 RougeScores) bool {
	return math.Abs(s1.Precision-s2.Precision) < floatTolerance &&
		math.Abs(s1.Recall-s2.Recall) < floatTolerance &&
		math.Abs(s1.F1Score-s2.F1Score) < floatTolerance
}

// Helper function to compare slices of DiffBlock (order matters)
func diffBlocksEqual(b1, b2 []DiffBlock) bool {
	// Handle nil vs empty slice comparison
	if (b1 == nil && len(b2) == 0) || (len(b1) == 0 && b2 == nil) || (len(b1) == 0 && len(b2) == 0) {
		return true
	}
	return reflect.DeepEqual(b1, b2)
}

// Helper function to compare slices of Tokens (order matters)
func tokensEqual(t1, t2 []Token) bool {
	// Handle nil vs empty slice comparison
	if (t1 == nil && len(t2) == 0) || (len(t1) == 0 && t2 == nil) || (len(t1) == 0 && len(t2) == 0) {
		return true
	}
	return reflect.DeepEqual(t1, t2)
}

// --- Test Cases ---

func TestTokenize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Token
	}{
		{
			name:  "Simple words",
			input: "hello world",
			want:  []Token{{Text: "hello", IsWord: true}, {Text: " ", IsWord: false}, {Text: "world", IsWord: true}},
		},
		{
			name:  "Words with punctuation",
			input: "hello. world?",
			want:  []Token{{Text: "hello", IsWord: true}, {Text: ".", IsWord: false}, {Text: " ", IsWord: false}, {Text: "world", IsWord: true}, {Text: "?", IsWord: false}},
		},
		{
			name:  "Multiple spaces and punctuation",
			input: " A  B. C \t D! ",
			// Expect: " "(s), A(w), "  "(s), B(w), .(p), " "(s), C(w), " \t "(s), D(w), !(p), " "(s)
			want: []Token{{Text: " ", IsWord: false}, {Text: "A", IsWord: true}, {Text: "  ", IsWord: false}, {Text: "B", IsWord: true}, {Text: ".", IsWord: false}, {Text: " ", IsWord: false}, {Text: "C", IsWord: true}, {Text: " \t ", IsWord: false}, {Text: "D", IsWord: true}, {Text: "!", IsWord: false}, {Text: " ", IsWord: false}},
		},
		{
			name:  "Leading/trailing spaces",
			input: "  word  ",
			want:  []Token{{Text: "  ", IsWord: false}, {Text: "word", IsWord: true}, {Text: "  ", IsWord: false}},
		},
		{
			name:  "Only non-words",
			input: "  .?! \n ",
			// Expect: "  "(s), .(p), ?(p), !(p), " \n "(s) -> Corrected based on final tokenizer
			want: []Token{{Text: "  ", IsWord: false}, {Text: ".", IsWord: false}, {Text: "?", IsWord: false}, {Text: "!", IsWord: false}, {Text: " \n ", IsWord: false}},
		},
		{
			name:  "Empty string",
			input: "",
			want:  []Token{},
		},
		{
			name:  "No trailing space",
			input: "end",
			want:  []Token{{Text: "end", IsWord: true}},
		},
		{
			name:  "Only spaces",
			input: "   ",
			want:  []Token{{Text: "   ", IsWord: false}},
		},
		{
			name:  "Only punctuation",
			input: "?!.",
			want:  []Token{{Text: "?", IsWord: false}, {Text: "!", IsWord: false}, {Text: ".", IsWord: false}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenize(tt.input)
			if !tokensEqual(got, tt.want) {
				t.Errorf("tokenize(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

// Explicit slices for potentially problematic test cases
var refCvD = []string{"the", "quick", "brown", "fox", "jumps", "over", "the", "lazy", "dog"} // 9 words
var candCvD = []string{"the", "slow", "brown", "cat", "jumps", "over", "a", "lazy", "dog"}   // 10 words
var lcsCvD = []string{"the", "brown", "jumps", "over", "lazy", "dog"}                        // 6 words
var wantCvD = RougeScores{Precision: 6.0 / 9.0, Recall: 6.0 / 9.0, F1Score: (2 * (6.0 / 9.0) * (6.0 / 9.0)) / (6.0/9.0 + 6.0/9.0)}

var refAvB = []string{"hi", "there", "buddy", "how", "ya", "doin"}          // 6 words
var candAvB = []string{"hi", "there", "friend", "how", "are", "ya", "doin"} // 7 words
var lcsAvB = []string{"hi", "there", "how", "ya", "doin"}                   // 5 words
var wantAvB = RougeScores{Precision: 5.0 / 7.0, Recall: 5.0 / 6.0, F1Score: (2 * (5.0 / 7.0) * (5.0 / 6.0)) / (5.0/7.0 + 5.0/6.0)}

func TestCalculateROUGEL(t *testing.T) {
	tests := []struct {
		name string
		ref  []string
		cand []string
		lcs  []string
		want RougeScores
	}{
		{
			name: "Perfect Match",
			ref:  []string{"a", "b", "c"},
			cand: []string{"a", "b", "c"},
			lcs:  []string{"a", "b", "c"},
			want: RougeScores{Precision: 1.0, Recall: 1.0, F1Score: 1.0},
		},
		{
			name: "No Overlap",
			ref:  []string{"a", "b", "c"},
			cand: []string{"d", "e", "f"},
			lcs:  []string{},
			want: RougeScores{Precision: 0.0, Recall: 0.0, F1Score: 0.0},
		},
		{
			name: "Partial Overlap (a vs b case)",
			ref:  refAvB,
			cand: candAvB,
			lcs:  lcsAvB,
			want: wantAvB,
		},
		{
			name: "Partial Overlap (c vs d case)",
			ref:  refCvD,
			cand: candCvD,
			lcs:  lcsCvD,
			want: wantCvD,
		},
		{
			name: "Empty Ref",
			ref:  []string{},
			cand: []string{"a", "b"},
			lcs:  []string{},
			want: RougeScores{Precision: 0.0, Recall: 0.0, F1Score: 0.0},
		},
		{
			name: "Empty Cand",
			ref:  []string{"a", "b"},
			cand: []string{},
			lcs:  []string{},
			want: RougeScores{Precision: 0.0, Recall: 0.0, F1Score: 0.0},
		},
		{
			name: "Both Empty",
			ref:  []string{},
			cand: []string{},
			lcs:  []string{},
			want: RougeScores{Precision: 1.0, Recall: 1.0, F1Score: 1.0},
		},
		{
			name: "LCS longer than Cand (impossible case check)",
			ref:  []string{"a", "b"},
			cand: []string{"a"},
			lcs:  []string{"a", "b"},
			want: RougeScores{Precision: 2.0 / 1.0, Recall: 2.0 / 2.0, F1Score: (2 * 2.0 * 1.0) / (2.0 + 1.0)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateROUGEL(tt.ref, tt.cand, tt.lcs)
			if !scoresEqual(got, tt.want) {
				// Use %#v for more detailed struct output on failure
				t.Errorf("calculateROUGEL(%d,%d,%d words) = %#v, want %#v", len(tt.ref), len(tt.cand), len(tt.lcs), got, tt.want)
			}
		})
	}
}

// Test findSingleLCS instead of findAllLCS
func TestFindSingleLCS(t *testing.T) {
	tests := []struct {
		name string
		s1   string
		s2   string
		want string // Expect a single LCS string (space-separated words)
	}{
		{
			name: "Simple Match",
			s1:   "A B C",
			s2:   "A X B C",
			want: "A B C",
		},
		{
			name: "No Match",
			s1:   "A B C",
			s2:   "D E F",
			want: "",
		},
		{
			name: "From Tests (a vs b)", // Already space-separated words
			s1:   "hi there buddy how ya doin?",
			s2:   "hi there friend how are ya doin?",
			want: "hi there how ya doin",
		},
		{
			name: "From Tests (c vs d)", // Already space-separated words
			s1:   "the quick brown fox jumps over the lazy dog",
			s2:   "the slow brown cat jumps over a lazy dog",
			want: "the brown jumps over lazy dog",
		},
		{
			name: "Multiple LCS Example 1",
			s1:   "A B C B D A B",
			s2:   "B D C A B",
			// Note: The exact result depends on the tie-breaker in backtracking.
			// Based on preferring `i--` on tie, it should find one of these.
			// Let's assume it finds "B C A B" or "B D A B". We'll list one in `want`
			// and check against a map of possibilities in the test logic.
			want: "B C A B",
		},
		{
			name: "Multiple LCS Example 2 (AGCAT vs GAC)",
			s1:   "A G C A T",
			s2:   "G A C",
			// Note: Possible LCSs are "G C", "G A", "A C".
			// Based on preferring `i--` on tie, it might find "G C".
			want: "G C",
		},
		{
			name: "From Tests (g vs h)", // Already space-separated words
			s1:   "A B C D E F",
			s2:   "A X C D Y F",
			want: "A C D F",
		},
		{
			name: "Empty Strings",
			s1:   "",
			s2:   "",
			want: "",
		},
		{
			name: "One Empty String",
			s1:   "A B C",
			s2:   "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the *actual* tokenizer for this test now
			w1 := getWords(tokenize(tt.s1))
			w2 := getWords(tokenize(tt.s2))
			dp := buildDPTable(w1, w2)
			got := findSingleLCS(dp, w1, w2)

			// Check against the single expected `want` string first
			if got == tt.want {
				return // Simple match, test passes
			}

			// If simple match fails, check specific multi-LCS cases
			// Update the maps to contain space-separated word strings
			if tt.name == "Multiple LCS Example 1" {
				validLCS := map[string]bool{"B C A B": true, "B D A B": true}
				if !validLCS[got] {
					t.Errorf("findSingleLCS(%q, %q) = %q, want one of %v", tt.s1, tt.s2, got, []string{"B C A B", "B D A B"})
				}
				return // Handled multi-LCS case 1
			}

			if tt.name == "Multiple LCS Example 2 (AGCAT vs GAC)" {
				validLCS := map[string]bool{"A C": true, "G A": true, "G C": true}
				if !validLCS[got] {
					t.Errorf("findSingleLCS(%q, %q) = %q, want one of %v", tt.s1, tt.s2, got, []string{"A C", "G A", "G C"})
				}
				return // Handled multi-LCS case 2
			}

			// If none of the above matched (simple or specific multi-LCS checks), then it's a general failure.
			t.Errorf("findSingleLCS(%q, %q) = %q, want %q", tt.s1, tt.s2, got, tt.want)
		})
	}
}

// Explicit slices for FindNonLCS tests
var alignRefAvBWords = []string{"hi", "there", "buddy", "how", "ya", "doin"}
var alignRefAvBAlign = []WordAlignment{{IsLCS: true}, {IsLCS: true}, {IsLCS: false}, {IsLCS: true}, {IsLCS: true}, {IsLCS: true}}
var alignCandAvBWords = []string{"hi", "there", "friend", "how", "are", "ya", "doin"}
var alignCandAvBAlign = []WordAlignment{{IsLCS: true}, {IsLCS: true}, {IsLCS: false}, {IsLCS: true}, {IsLCS: false}, {IsLCS: true}, {IsLCS: true}}

func TestFindNonLCSBlocksWords(t *testing.T) {
	tests := []struct {
		name      string
		words     []string
		alignment []WordAlignment // Corresponds 1:1 with words
		want      []DiffBlock
	}{
		{
			name:      "Simple block",
			words:     []string{"a", "b", "c", "d"},
			alignment: []WordAlignment{{IsLCS: true}, {IsLCS: false}, {IsLCS: false}, {IsLCS: true}},
			want:      []DiffBlock{{Text: "b c", Length: 2}},
		},
		{
			name:      "Multiple blocks",
			words:     []string{"a", "b", "c", "d", "e", "f"},
			alignment: []WordAlignment{{IsLCS: true}, {IsLCS: false}, {IsLCS: false}, {IsLCS: true}, {IsLCS: false}, {IsLCS: true}},
			want:      []DiffBlock{{Text: "b c", Length: 2}, {Text: "e", Length: 1}},
		},
		{
			name:      "Block at start",
			words:     []string{"a", "b", "c", "d"},
			alignment: []WordAlignment{{IsLCS: false}, {IsLCS: false}, {IsLCS: true}, {IsLCS: true}},
			want:      []DiffBlock{{Text: "a b", Length: 2}},
		},
		{
			name:      "Block at end",
			words:     []string{"a", "b", "c", "d"},
			alignment: []WordAlignment{{IsLCS: true}, {IsLCS: true}, {IsLCS: false}, {IsLCS: false}},
			want:      []DiffBlock{{Text: "c d", Length: 2}},
		},
		{
			name:      "All LCS",
			words:     []string{"a", "b", "c", "d"},
			alignment: []WordAlignment{{IsLCS: true}, {IsLCS: true}, {IsLCS: true}, {IsLCS: true}},
			want:      []DiffBlock{},
		},
		{
			name:      "No LCS",
			words:     []string{"a", "b", "c", "d"},
			alignment: []WordAlignment{{IsLCS: false}, {IsLCS: false}, {IsLCS: false}, {IsLCS: false}},
			want:      []DiffBlock{{Text: "a b c d", Length: 4}},
		},
		{
			name:      "Empty input",
			words:     []string{},
			alignment: []WordAlignment{},
			want:      []DiffBlock{},
		},
		{
			name:      "Reference side (a vs b)",
			words:     alignRefAvBWords,
			alignment: alignRefAvBAlign,
			want:      []DiffBlock{{Text: "buddy", Length: 1}},
		},
		{
			name:      "Candidate side (a vs b)",
			words:     alignCandAvBWords,
			alignment: alignCandAvBAlign,
			want:      []DiffBlock{{Text: "friend", Length: 1}, {Text: "are", Length: 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findNonLCSBlocksWords(tt.words, tt.alignment)
			// Compare content ignoring order for this test, as block order might vary slightly
			// Convert to map for comparison? Or sort both got and want? Sort is easier.
			// sort.Slice(got, func(i, j int) bool { return got[i].Text < got[j].Text })
			// sort.Slice(tt.want, func(i, j int) bool { return tt.want[i].Text < tt.want[j].Text })
			// Sticking to DeepEqual as order *should* be deterministic from findNonLCSBlocksWords

			if !diffBlocksEqual(got, tt.want) {
				t.Errorf("findNonLCSBlocksWords() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
