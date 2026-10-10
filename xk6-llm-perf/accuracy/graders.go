package accuracy

import (
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	numericLiteral = regexp.MustCompile(`^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)(/[+-]?[0-9]+)?$`)
	gsmMarker      = regexp.MustCompile(`#### (-?[0-9.,]+)`)
	gsmNumber      = regexp.MustCompile(`-?[0-9][0-9,]*(\.[0-9]+)?`)
	mathNumber     = regexp.MustCompile(`-?[0-9]+(\.[0-9]+)?(/[0-9]+)?`)
	mathPhrase     = regexp.MustCompile(`(?i)(final\s+answer|the\s+answer\s+is|answer\s*[:=]|answer\s+is)\s*[:=]?\s*`)
	simpleFraction = regexp.MustCompile(`\\frac\{([^{}]+)\}\{([^{}]+)\}`)
	textWrapper    = regexp.MustCompile(`\\(text|mathrm|mathit|mathbf)\{([^{}]*)\}`)
)

// gradeExtended implements the pinned source's choice and GSM8K extraction.
// The numeric math evaluators deliberately have separate names from symbolic math.
func gradeExtended(evaluator, pred, ref string) (correct, unparsed bool, err error) {
	if len(pred) > 65536 || len(ref) > 65536 {
		return false, false, fmt.Errorf("answer exceeds budget")
	}
	pred, ref = strings.TrimSpace(pred), strings.TrimSpace(ref)
	switch evaluator {
	case "gsm8k-v1":
		gold := gsmMarker.FindStringSubmatch(ref)
		if gold == nil {
			return false, false, fmt.Errorf("GSM8K reference lacks #### number")
		}
		p := gsmMarker.FindStringSubmatch(pred)
		answer := ""
		if p != nil {
			answer = p[1]
		} else {
			unparsed = true
			numbers := gsmNumber.FindAllString(pred, -1)
			if len(numbers) > 0 {
				answer = numbers[len(numbers)-1]
			}
		}
		g, gerr := strconv.ParseFloat(strings.ReplaceAll(gold[1], ",", ""), 64)
		v, verr := strconv.ParseFloat(strings.ReplaceAll(answer, ",", ""), 64)
		if gerr != nil || math.IsInf(g, 0) || math.IsNaN(g) {
			return false, unparsed, fmt.Errorf("invalid GSM8K reference number")
		}
		return verr == nil && !math.IsInf(v, 0) && math.Abs(g-v) <= 1e-6, unparsed, nil
	case "mmlu-v1", "mmlu-pro-v1":
		letters := "ABCD"
		if evaluator == "mmlu-pro-v1" {
			letters = "ABCDEFGHIJ"
		}
		if len(ref) != 1 || !strings.Contains(letters, ref) {
			return false, false, fmt.Errorf("invalid multiple-choice reference")
		}
		answer, fallback := extendedChoice(pred, letters, evaluator == "mmlu-v1")
		return answer != "" && answer == ref, fallback, nil
	case "math-numeric-v1", "aime-numeric-v1":
		answer, fallback := extractMath(pred)
		p, g := normalizeNumericMath(answer), normalizeNumericMath(ref)
		if len(p) > 4096 || len(g) > 4096 {
			return false, fallback, fmt.Errorf("numeric expression exceeds budget")
		}
		// This is the stdlib fallback: exact rational equality, not percent variants,
		// numeric tolerance or symbolic equality from the full optional-dependency path.
		ps := strings.NewReplacer("(", "", ")", "").Replace(p)
		gs := strings.NewReplacer("(", "", ")", "").Replace(g)
		x, y := new(big.Rat), new(big.Rat)
		ok, yok := false, false
		if numericLiteral.MatchString(ps) {
			_, ok = x.SetString(ps)
		}
		if numericLiteral.MatchString(gs) {
			_, yok = y.SetString(gs)
		}
		if ok && yok {
			return x.Cmp(y) == 0, fallback, nil
		}
		return p != "" && p == g, fallback, nil
	default:
		return false, false, fmt.Errorf("unsupported extended evaluator")
	}
}

func extendedChoice(text, letters string, mmlu bool) (string, bool) {
	first := strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
	if len(first) == 1 && strings.Contains(letters, first) {
		return first, false
	}
	tier1 := regexp.MustCompile(`answer is \(?([` + letters + `])\)?`)
	matches := tier1.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[len(matches)-1][1], false
	}
	tier2 := regexp.MustCompile(`[aA]nswer:\s*([` + letters + `])`)
	matches = tier2.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return matches[len(matches)-1][1], true
	}
	if mmlu {
		if matches := standaloneChoices(first, letters); len(matches) > 0 {
			return matches[0], true
		}
	}
	if matches := standaloneChoices(text, letters); len(matches) > 0 {
		return matches[len(matches)-1], true
	}
	return "", true
}

func boxedAnswer(text string) (string, bool) {
	start := strings.LastIndex(text, `\boxed{`)
	if start < 0 {
		return "", false
	}
	start += len(`\boxed{`)
	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
			if depth > 128 {
				return "", false
			}
		case '}':
			if depth == 0 {
				return text[start:i], true
			}
			depth--
		}
	}
	return "", false
}

func extractMath(text string) (string, bool) {
	if answer, ok := boxedAnswer(text); ok {
		answer = strings.TrimPrefix(strings.ReplaceAll(answer, "\n", ""), ":")
		answer = strings.TrimSuffix(strings.TrimSuffix(answer, "."), "/")
		return answer, false
	}
	matches := mathPhrase.FindAllStringIndex(text, -1)
	if len(matches) > 0 {
		tail := text[matches[len(matches)-1][1]:]
		// End a phrase at a newline or a sentence dot; retain decimal dots.
		for i, c := range tail {
			if c == '\n' || c == '.' && (i == 0 || tail[i-1] < '0' || tail[i-1] > '9') && (i+1 == len(tail) || tail[i+1] < '0' || tail[i+1] > '9') {
				tail = tail[:i]
				break
			}
		}
		tail = strings.TrimSpace(tail)
		if strings.ContainsAny(tail, `\{}`) {
			return tail, true
		}
		if nums := mathNumber.FindAllString(tail, -1); len(nums) > 0 {
			return nums[len(nums)-1], true
		}
		return tail, true
	}
	if nums := mathNumber.FindAllString(text, -1); len(nums) > 0 {
		return nums[len(nums)-1], true
	}
	return strings.TrimSpace(text), true
}

func normalizeNumericMath(text string) string {
	s := strings.TrimSpace(text)
	s = strings.TrimRight(s, ".,;:!?")
	if len(s) >= 2 && s[0] == '$' && s[len(s)-1] == '$' {
		s = s[1 : len(s)-1]
	}
	s = strings.NewReplacer(`\left`, "", `\right`, "", `\dfrac`, `\frac`, `\tfrac`, `\frac`).Replace(s)
	s = simpleFraction.ReplaceAllString(s, "($1)/($2)")
	s = textWrapper.ReplaceAllString(s, "$2")
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// Python's upstream \w uses Unicode letters and numbers; RE2's \b is ASCII.
func standaloneChoices(text, letters string) []string {
	var result []string
	word := func(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) }
	for i, r := range text {
		if r > 127 || !strings.ContainsRune(letters, r) {
			continue
		}
		if i > 0 {
			previous, _ := utf8.DecodeLastRuneInString(text[:i])
			if word(previous) {
				continue
			}
		}
		if i+1 < len(text) {
			next, _ := utf8.DecodeRuneInString(text[i+1:])
			if word(next) {
				continue
			}
		}
		result = append(result, string(r))
	}
	return result
}
