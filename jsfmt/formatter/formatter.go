// Package formatter provides JavaScript formatting functionality.
package formatter

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
)

// Format formats JavaScript source code according to Chrome's style.
// It returns the formatted source.
func Format(filename string, src []byte, tabSize int) ([]byte, error) {
	// First try using node and prettier to format JavaScript
	formatted, err := formatWithNode(src, tabSize)
	if err == nil {
		return formatted, nil
	}

	// If node fails or isn't available, fall back to native implementation
	return FormatNative(src, tabSize)
}

// formatWithNode attempts to format JavaScript using Node.js and Prettier.
func formatWithNode(src []byte, tabSize int) ([]byte, error) {
	// Check if node is available
	if _, err := exec.LookPath("node"); err != nil {
		return nil, fmt.Errorf("node not found: %v", err)
	}
	
	cmd := exec.Command("node", "-e", prettierScript(tabSize))
	cmd.Stdin = bytes.NewReader(src)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("formatting error: %v\n%s", err, stderr.String())
	}

	return out.Bytes(), nil
}

// IsFormatted reports whether the file is already formatted according to jsfmt's style.
func IsFormatted(src io.Reader, tabSize int) (bool, error) {
	input, err := io.ReadAll(src)
	if err != nil {
		return false, err
	}

	formatted, err := Format("", input, tabSize)
	if err != nil {
		return false, err
	}

	return bytes.Equal(input, formatted), nil
}

// WriteDiff writes a diff of what would be changed to out.
func WriteDiff(out io.Writer, src io.Reader, filename string, tabSize int) error {
	input, err := io.ReadAll(src)
	if err != nil {
		return err
	}

	formatted, err := Format(filename, input, tabSize)
	if err != nil {
		return err
	}

	if bytes.Equal(input, formatted) {
		return nil
	}

	// Create diff
	cmd := exec.Command("diff", "-u", "--label=original", "--label=formatted", "-", "-")
	cmd.Stdin = bytes.NewReader(append(input, '\n'))
	cmd.Stdout = out
	diffIn, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	go func() {
		defer diffIn.Close()
		diffIn.Write(input)
		diffIn.Write([]byte{'\n'})
		diffIn.Write(formatted)
		diffIn.Write([]byte{'\n'})
	}()

	return cmd.Run()
}

// prettierScript returns the JavaScript code to run prettier with the specified tab size.
func prettierScript(tabSize int) string {
	return fmt.Sprintf(`
const prettier = require('prettier');
const fs = require('fs');

// Read from stdin
let code = '';
process.stdin.on('data', chunk => {
  code += chunk;
});

process.stdin.on('end', () => {
  try {
    const result = prettier.format(code, {
      parser: 'babel',
      printWidth: 80,
      tabWidth: %d,
      useTabs: false,
      semi: true,
      singleQuote: true,
      quoteProps: 'as-needed',
      jsxSingleQuote: false,
      trailingComma: 'es5',
      bracketSpacing: true,
      bracketSameLine: false,
      arrowParens: 'always',
      endOfLine: 'lf',
    });
    process.stdout.write(result);
  } catch (error) {
    console.error('Error formatting code:', error);
    process.exit(1);
  }
});
`, tabSize)
}