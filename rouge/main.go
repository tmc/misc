package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math" // Import math for float comparison
	"os"
	"sort"
	"strings"
	"unicode"
)

// ANSI color codes (Bold)
const (
	ColorReset = "\033[0m"
	ColorGreen = "\033[1;32m" // Green (LCS match)
	ColorRed   = "\033[1;31m" // Red (Extra in Candidate)
	ColorBlue  = "\033[1;34m" // Blue (Missing from Candidate perspective / Unused in Ref)
)

/* ---------------------------------------------------------------------- */
/* data structures                                                        */

type RougeScores struct {
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1Score   float64 `json:"f1_score"`
}

// Represents a contiguous block of non-LCS words
type DiffBlock struct {
	Text   string `json:"text"`
	Length int    `json:"length"`
}

type Output struct {
	RougeL  RougeScores `json:"rouge_l"`
	LCS     string      `json:"lcs"`                      // Store the single primary LCS found
	Extra   []DiffBlock `json:"extra_blocks,omitempty"`   // Top E longest blocks in Candidate not in Primary LCS
	Missing []DiffBlock `json:"missing_blocks,omitempty"` // Top M longest blocks in Reference not in Primary LCS
}

// Alignment associated with word index
type WordAlignment struct {
	IsLCS        bool
	IsTopExtra   bool
	IsTopMissing bool
}

/* ---------------------------------------------------------------------- */
/* entry‑point                                                            */

func main() { os.Exit(run()) }

func run() int {
	var threshold float64
	var jsonOutput bool
	var printMode string
	var topNLCS int // Keep flag for potential future use, but logic uses 1 LCS
	var topNExtra int
	var topNMissing int

	flag.Float64Var(&threshold, "t", 0.0, "ROUGE‑L F1 threshold (0–1)")
	flag.BoolVar(&jsonOutput, "json", false, "Emit JSON instead of text")
	flag.StringVar(&printMode, "p", "cand", "Which file's perspective to print ('ref' or 'cand')")
	flag.IntVar(&topNLCS, "n", 1, "Number of top-length LCSs to find/show (Currently finds only 1)") // Updated help text
	flag.IntVar(&topNExtra, "e", 0, "Show top N longest blocks in candidate not in primary LCS (red)")
	flag.IntVar(&topNMissing, "m", 0, "Show top N longest blocks in reference not in primary LCS (blue)")
	flag.Parse()

	// --- Flag Validation ---
	if flag.NArg() != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <reference_file> <candidate_file>\n", os.Args[0])
		return 2
	}
	if printMode != "ref" && printMode != "cand" {
		fmt.Fprintln(os.Stderr, "Error: -p flag must be 'ref' or 'cand'")
		return 2
	}
	// No need to validate topNLCS >= 1 if we only find 1
	if topNExtra < 0 || topNMissing < 0 {
		fmt.Fprintln(os.Stderr, "Error: -e and -m must be >= 0")
		return 2
	}

	// --- File Reading ---
	refPath := flag.Arg(0)
	candPath := flag.Arg(1)
	refText, err := readFile(refPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading reference file '%s': %v\n", refPath, err)
		return 1
	}
	candText, err := readFile(candPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading candidate file '%s': %v\n", candPath, err)
		return 1
	}

	// --- Tokenization & Word Extraction ---
	refTokens := tokenize(refText) // Use the fixed tokenizer
	candTokens := tokenize(candText)
	refWords := getWords(refTokens)
	candWords := getWords(candTokens)

	// --- LCS Calculation ---
	dpTable := buildDPTable(refWords, candWords)
	primaryLCS := findSingleLCS(dpTable, refWords, candWords) // Call the single LCS function

	// --- ROUGE Calculation ---
	rougeL := calculateROUGEL(refWords, candWords, strings.Fields(primaryLCS))

	// --- Alignment (based on word indices) ---
	refWordAlign := make([]WordAlignment, len(refWords))
	candWordAlign := make([]WordAlignment, len(candWords))
	if primaryLCS != "" { // Only align if there's a non-empty LCS
		alignWordsToLCSWords(dpTable, refWords, candWords, len(refWords), len(candWords), refWordAlign, candWordAlign)
	}

	// --- Find Diff Blocks (based on words) ---
	refBlocks := findNonLCSBlocksWords(refWords, refWordAlign)
	candBlocks := findNonLCSBlocksWords(candWords, candWordAlign)
	sort.SliceStable(refBlocks, func(i, j int) bool { return refBlocks[i].Length > refBlocks[j].Length })
	sort.SliceStable(candBlocks, func(i, j int) bool { return candBlocks[i].Length > candBlocks[j].Length })

	topMissing := refBlocks
	if len(topMissing) > topNMissing {
		topMissing = topMissing[:topNMissing]
	}
	topExtra := candBlocks
	if len(topExtra) > topNExtra {
		topExtra = topExtra[:topNExtra]
	}

	// Mark alignment arrays for top blocks
	markTopBlocksWords(refWords, refWordAlign, topMissing, true)
	markTopBlocksWords(candWords, candWordAlign, topExtra, false)

	// --- Output ---
	out := Output{ // Simplified output struct
		RougeL:  rougeL,
		LCS:     primaryLCS, // Store only the single LCS
		Extra:   topExtra,
		Missing: topMissing,
	}

	if jsonOutput {
		j, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(j))
	} else {
		printHumanReadable(out, printMode, topNLCS) // Pass topNLCS for accurate message
		if printMode == "cand" {
			highlight(candTokens, candWordAlign)
		} else {
			highlight(refTokens, refWordAlign)
		}
		fmt.Println() // Add a final newline after highlighted output
	}

	// --- Threshold Check ---
	const floatTolerance = 1e-9 // Define tolerance for comparison
	if !jsonOutput && rougeL.F1Score < threshold && math.Abs(rougeL.F1Score-threshold) > floatTolerance {
		// Check if F1 is strictly less than threshold, considering tolerance
		fmt.Fprintf(os.Stderr, "\nThreshold check failed: F1 %.4f < threshold %.4f\n", rougeL.F1Score, threshold)
		return 1
	}
	return 0
}

/* ---------------------------------------------------------------------- */
/* Tokenization and Word Extraction                                       */

type Token struct {
	Text   string
	IsWord bool
}

// Final Tokenizer: State machine based on rune type. Groups consecutive words/spaces, separates punctuation.
func tokenize(text string) []Token {
	tokens := make([]Token, 0)
	if len(text) == 0 { return tokens }

	var currentToken strings.Builder
	var currentType int // 0:unset, 1:word, 2:space, 3:punct/other

	getCharType := func(r rune) int {
		// Prioritize Letter/Digit for word determination
		if unicode.IsLetter(r) || unicode.IsDigit(r) { return 1 }
		if unicode.IsSpace(r) { return 2 }
		// Treat punctuation and any other symbol as type 3
		if unicode.IsPunct(r) || unicode.IsSymbol(r) { return 3 }
		// Fallback for control characters etc. - treat as non-word/non-space/non-punct
		return 3
	}

	for _, r := range text {
		runeType := getCharType(r)
		if currentToken.Len() == 0 { // Start first token
			currentToken.WriteRune(r)
			currentType = runeType
		} else {
			// Continue token ONLY if it's whitespace following whitespace (type 2)
			// OR if it's a word char following a word char (type 1).
			// Punctuation/Other (type 3) always breaks the sequence.
			shouldContinue := (runeType == currentType && (currentType == 1 || currentType == 2))

			if shouldContinue {
				currentToken.WriteRune(r)
			} else { // Type changed or encountered punctuation/other
				tokens = append(tokens, Token{Text: currentToken.String(), IsWord: (currentType == 1)})
				currentToken.Reset()
				currentToken.WriteRune(r)
				currentType = runeType
			}
		}
	}
	// Add final token
	if currentToken.Len() > 0 {
		tokens = append(tokens, Token{Text: currentToken.String(), IsWord: (currentType == 1)})
	}
	return tokens
}


// getWords extracts only the word strings from a token slice.
func getWords(tokens []Token) []string {
	words := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t.IsWord {
			words = append(words, t.Text)
		}
	}
	return words
}

/* ---------------------------------------------------------------------- */
/* human‑readable report (stderr)                                         */

func printHumanReadable(o Output, printMode string, nFlag int) { // Accept nFlag
	fmt.Fprintf(os.Stderr, "ROUGE-L (Primary LCS) P: %.4f  R: %.4f  F1: %.4f\n", o.RougeL.Precision, o.RougeL.Recall, o.RougeL.F1Score)
	// Adjust message based on nFlag, even though we only calculate 1 LCS now
	lcsMsg := "Longest Common Subsequence:"
	if nFlag > 1 {
		lcsMsg = fmt.Sprintf("Top 1 (of %d requested) Longest Common Subsequence:", nFlag)
	}
	fmt.Fprintf(os.Stderr, "\n%s\n", lcsMsg)

	// Handle case where LCS string might be empty
	if o.LCS == "" && math.Abs(o.RougeL.F1Score-1.0) > 1e-9 { // Only print (None) if F1 isn't 1 (e.g. not identical empty files)
		fmt.Fprintf(os.Stderr, "  (None)\n")
	} else if o.LCS == "" {
		fmt.Fprintf(os.Stderr, "  (Empty)\n")
	} else {
		fmt.Fprintf(os.Stderr, "   1: %s\n", o.LCS)
	}
	if len(o.Extra) > 0 {
		fmt.Fprintf(os.Stderr, "\nTop %d Longest Candidate-Only Blocks (Red Highlights):\n", len(o.Extra))
		for i, b := range o.Extra {
			fmt.Fprintf(os.Stderr, "  %2d (%d words): %s\n", i+1, b.Length, b.Text)
		}
	}
	if len(o.Missing) > 0 {
		fmt.Fprintf(os.Stderr, "\nTop %d Longest Reference-Only Blocks (Blue Highlights):\n", len(o.Missing))
		for i, b := range o.Missing {
			fmt.Fprintf(os.Stderr, "  %2d (%d words): %s\n", i+1, b.Length, b.Text)
		}
	}
	fmt.Fprintf(os.Stderr, "\nHighlighted Output (%s):\n", printMode)
}

/* ---------------------------------------------------------------------- */
/* highlighter (stdout)                                                   */

func highlight(tokens []Token, wordAlignment []WordAlignment) {
	wordTokenIndex := -1 // Track index within the subset of word tokens
	for _, token := range tokens {
		if !token.IsWord {
			// Print non-word tokens directly
			fmt.Print(token.Text)
			continue
		}

		// It's a word token
		wordTokenIndex++
		align := WordAlignment{}                 // Default: not LCS, not Extra, not Missing
		if wordTokenIndex < len(wordAlignment) { // Check bounds
			align = wordAlignment[wordTokenIndex]
		} else {
			fmt.Fprintf(os.Stderr, "\n[Error] Highlight alignment index %d out of bounds (max %d) for token '%s'\n", wordTokenIndex, len(wordAlignment)-1, token.Text)
			// Continue without highlighting this word
		}

		color := ColorReset
		if align.IsLCS {
			color = ColorGreen
		} else if align.IsTopExtra {
			color = ColorRed
		} else if align.IsTopMissing {
			color = ColorBlue
		}

		if color != ColorReset {
			fmt.Printf("%s%s%s", color, token.Text, ColorReset)
		} else {
			fmt.Print(token.Text) // Print uncolored words
		}
	}
}

/* ---------------------------------------------------------------------- */
/* LCS and Diff Block Logic                                               */

func buildDPTable(words1, words2 []string) [][]int {
	m, n := len(words1), len(words2)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if words1[i-1] == words2[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}
	return dp
}

// Final Corrected findSingleLCS using Append/Reverse
func findSingleLCS(dp [][]int, w1, w2 []string) string {
	m, n := len(w1), len(w2)
	if m == 0 || n == 0 || len(dp) <= m || len(dp[m]) <= n { return "" } // Handle empty/invalid DP early

	lcsLen := dp[m][n]
	if lcsLen == 0 { return "" } // Handle no overlap early

	lcsWords := make([]string, 0, lcsLen) // Preallocate slice based on known length

	i, j := m, n
	for i > 0 && j > 0 {
		idx1 := i - 1
		idx2 := j - 1
		
		if w1[idx1] == w2[idx2] {
			lcsWords = append(lcsWords, w1[idx1]) // Append the match
			i--
			j--
		} else if dp[i-1][j] >= dp[i][j-1] { // Tie-break consistent with alignment (prefer skip ref)
			i--
		} else {
			j--
		}
	}

	// Reverse the LCS words (since appended during backtrack)
	for k := 0; k < len(lcsWords)/2; k++ {
		lcsWords[k], lcsWords[len(lcsWords)-1-k] = lcsWords[len(lcsWords)-1-k], lcsWords[k]
	}

	return strings.Join(lcsWords, " ")
}


// Align function working only on word indices
func alignWordsToLCSWords(dp [][]int, w1, w2 []string, i, j int, refAlign, candAlign []WordAlignment) {
	if i == 0 || j == 0 {
		return
	}

	// Check bounds before accessing w1/w2
	idx1 := i - 1
	idx2 := j - 1
	if idx1 < 0 || idx1 >= len(w1) || idx2 < 0 || idx2 >= len(w2) {
		return // Avoid panic on invalid indices
	}


	if w1[idx1] == w2[idx2] {
		if idx1 < len(refAlign) {
			refAlign[idx1].IsLCS = true
		}
		if idx2 < len(candAlign) {
			candAlign[idx2].IsLCS = true
		}
		alignWordsToLCSWords(dp, w1, w2, i-1, j-1, refAlign, candAlign)
	} else if dp[i-1][j] >= dp[i][j-1] { // Tie-break: Prefer skipping ref word
		alignWordsToLCSWords(dp, w1, w2, i-1, j, refAlign, candAlign)
	} else { // dp[i][j-1] > dp[i-1][j]
		alignWordsToLCSWords(dp, w1, w2, i, j-1, refAlign, candAlign)
	}
}

// Find blocks function working only on words
func findNonLCSBlocksWords(words []string, alignment []WordAlignment) []DiffBlock {
	blocks := make([]DiffBlock, 0) // Initialize as empty slice
	var currentBlockWords []string
	inBlock := false

	for i, word := range words {
		if i >= len(alignment) { continue } // Safety check

		isLCS := alignment[i].IsLCS
		if !isLCS && !inBlock {
			inBlock = true
			currentBlockWords = []string{word}
		} else if !isLCS && inBlock {
			currentBlockWords = append(currentBlockWords, word)
		} else if isLCS && inBlock {
			inBlock = false
			if len(currentBlockWords) > 0 {
				blocks = append(blocks, DiffBlock{
					Text:   strings.Join(currentBlockWords, " "),
					Length: len(currentBlockWords),
				})
			}
			currentBlockWords = nil
		}
	}
	if inBlock && len(currentBlockWords) > 0 {
		blocks = append(blocks, DiffBlock{
			Text:   strings.Join(currentBlockWords, " "),
			Length: len(currentBlockWords),
		})
	}
	return blocks
}

// Mark blocks function working only on words
func markTopBlocksWords(words []string, alignment []WordAlignment, topBlocks []DiffBlock, isMissing bool) {
	blockSet := make(map[string]int) // Store block text -> count to handle repeats
	for _, block := range topBlocks {
		blockSet[block.Text]++
	}

	var currentBlockWords []string
	var currentBlockIndices []int
	inBlock := false

	processBlock := func() {
		if len(currentBlockWords) > 0 {
			blockText := strings.Join(currentBlockWords, " ")
			if count, ok := blockSet[blockText]; ok && count > 0 {
				// Mark the corresponding word indices
				for _, wordIndex := range currentBlockIndices {
					if wordIndex < len(alignment) { // Bounds check
						if isMissing {
							alignment[wordIndex].IsTopMissing = true
						} else {
							alignment[wordIndex].IsTopExtra = true
						}
					}
				}
				blockSet[blockText]-- // Decrement count for this instance
			}
		}
		currentBlockWords = nil
		currentBlockIndices = nil
	}

	for i, word := range words {
		if i >= len(alignment) { continue } // Safety check

		isLCS := alignment[i].IsLCS
		if !isLCS && !inBlock {
			inBlock = true
			currentBlockWords = []string{word}
			currentBlockIndices = []int{i}
		} else if !isLCS && inBlock {
			currentBlockWords = append(currentBlockWords, word)
			currentBlockIndices = append(currentBlockIndices, i)
		} else if isLCS && inBlock {
			inBlock = false
			processBlock()
		}
	}
	if inBlock { // Process last block
		processBlock()
	}
}

/* ---------------------------------------------------------------------- */
/* ROUGE-L calculation                                                    */

func calculateROUGEL(refWords, candWords, lcsWords []string) RougeScores {
	lRef := float64(len(refWords))
	lCand := float64(len(candWords))
	lcsLen := float64(len(lcsWords))

	// Handle edge cases first
	if lRef == 0 && lCand == 0 {
		return RougeScores{Precision: 1.0, Recall: 1.0, F1Score: 1.0}
	}
	if lCand == 0 || lRef == 0 || lcsLen == 0 {
		return RougeScores{}
	} // Check denominators and lcsLen early

	precision := lcsLen / lCand // Precision uses CANDIDATE length
	recall := lcsLen / lRef     // Recall uses REFERENCE length
	f1 := 0.0
	if precision+recall > 1e-9 { // Avoid division by zero F1 = 2PR/(P+R)
		f1 = 2 * precision * recall / (precision + recall)
	}
	return RougeScores{Precision: precision, Recall: recall, F1Score: f1}
}

/* ---------------------------------------------------------------------- */
/* utils                                                                  */

func max(a, b int) int {
	if a > b { return a }
	return b
}

func readFile(path string) (string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil { return "", err }
	return string(bytes), nil
}
