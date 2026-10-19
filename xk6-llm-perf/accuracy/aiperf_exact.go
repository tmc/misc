package accuracy

import "strings"

// pythonSpace matches the complete str.strip whitespace set in the pinned
// AIPerf Python 3.11 runtime. Keep the set explicit: Python also strips U+001C
// through U+001F, which Go's unicode.IsSpace and strings.TrimSpace do not.
func pythonSpace(r rune) bool {
	switch {
	case r >= '\t' && r <= '\r', r >= 0x1c && r <= 0x20, r == 0x85,
		r == 0xa0, r == 0x1680, r >= 0x2000 && r <= 0x200a,
		r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000:
		return true
	}
	return false
}

func aiperfExactMatch(prediction, reference string) (correct, unparsed bool) {
	prediction = strings.TrimFunc(prediction, pythonSpace)
	reference = strings.TrimFunc(reference, pythonSpace)
	return prediction != "" && prediction == reference, prediction == "" && reference != ""
}
