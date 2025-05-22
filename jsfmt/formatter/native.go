package formatter

import (
	"regexp"
	"strings"
)

// FormatNative formats JavaScript using a Go native implementation.
// This is used as a fallback when Node.js is not available.
func FormatNative(src []byte, tabSize int) ([]byte, error) {
	// Create tab string based on tab size
	tab := strings.Repeat(" ", tabSize)
	
	// Convert input to string for easier manipulation
	code := string(src)
	
	// Normalize line endings
	code = strings.ReplaceAll(code, "\r\n", "\n")
	
	// Remove trailing whitespace from lines
	code = removeTrailingWhitespace(code)
	
	// Add space after keywords
	code = addSpaceAfterKeywords(code)
	
	// Format braces
	code = formatBraces(code, tab)
	
	// Format operators
	code = formatOperators(code)
	
	// Format semicolons
	code = formatSemicolons(code)
	
	// Format indentation
	code = formatIndentation(code, tab)
	
	// Ensure single newline at EOF
	code = ensureSingleNewlineAtEOF(code)
	
	return []byte(code), nil
}

func removeTrailingWhitespace(code string) string {
	re := regexp.MustCompile(`[ \t]+\n`)
	return re.ReplaceAllString(code, "\n")
}

func addSpaceAfterKeywords(code string) string {
	keywords := []string{
		"if", "else", "for", "while", "do", "switch", "catch", "try", "with", "return",
	}
	
	for _, keyword := range keywords {
		re := regexp.MustCompile(`\b` + keyword + `\(`)
		code = re.ReplaceAllString(code, keyword+" (")
	}
	
	return code
}

func formatBraces(code string, tab string) string {
	// Add space before opening braces
	re := regexp.MustCompile(`(\w|\)|\])\{`)
	code = re.ReplaceAllString(code, "$1 {")
	
	// Add newline after opening braces
	re = regexp.MustCompile(`\{(\S)`)
	code = re.ReplaceAllString(code, "{\n"+tab+"$1")
	
	// Add newline before closing braces
	re = regexp.MustCompile(`(\S)\}`)
	code = re.ReplaceAllString(code, "$1\n}")
	
	// Handle else statements
	re = regexp.MustCompile(`\}\s*else\s*\{`)
	code = re.ReplaceAllString(code, "} else {")
	
	return code
}

func formatOperators(code string) string {
	operators := []string{
		"=", "\\+", "-", "\\*", "/", "%", "==", "===", "!=", "!==", 
		"<", ">", "<=", ">=", "\\?", ":", "\\+=", "-=", "\\*=", "/=", "%=",
	}
	
	for _, op := range operators {
		re := regexp.MustCompile(`(\S)` + op + `(\S)`)
		code = re.ReplaceAllString(code, "$1 "+strings.ReplaceAll(op, "\\", "")+" $2")
	}
	
	return code
}

func formatSemicolons(code string) string {
	// Ensure space after semicolons
	re := regexp.MustCompile(`;(\S)`)
	code = re.ReplaceAllString(code, "; $1")
	
	return code
}

func formatIndentation(code string, tab string) string {
	lines := strings.Split(code, "\n")
	indentLevel := 0
	
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		
		// Decrease indent level for lines with just closing braces/brackets
		if trimmedLine == "}" || trimmedLine == ")" || trimmedLine == "]" {
			indentLevel--
		}
		
		// Apply indentation if the line is not empty
		if trimmedLine != "" {
			lines[i] = strings.Repeat(tab, indentLevel) + trimmedLine
		} else {
			lines[i] = ""
		}
		
		// Increase indent level after opening braces/brackets
		if strings.HasSuffix(trimmedLine, "{") || 
		   strings.HasSuffix(trimmedLine, "(") || 
		   strings.HasSuffix(trimmedLine, "[") {
			indentLevel++
		}
	}
	
	return strings.Join(lines, "\n")
}

func ensureSingleNewlineAtEOF(code string) string {
	// Trim trailing newlines and add exactly one
	return strings.TrimRight(code, "\n") + "\n"
}