package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tmc/misc/xq/xml"
)

var version = "0.1.0"

func main() {
	os.Exit(run(os.Args, os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// Parse flags
	flags := flag.NewFlagSet(args[0], flag.ExitOnError)
	flags.SetOutput(stderr)

	// Re-declare all flags here
	compactOutput := flags.Bool("c", false, "compact instead of pretty-printed output")
	rawOutput := flags.Bool("r", false, "output raw strings, not JSON texts")
	colorOutput := flags.Bool("C", false, "colorize JSON")
	nullInput := flags.Bool("n", false, "use `null` as the single input value")
	slurpInput := flags.Bool("s", false, "read (slurp) all inputs into an array")
	fromJSON := flags.Bool("f", false, "input is JSON, not XML")
	toJSON := flags.Bool("j", false, "output as JSON")
	htmlInput := flags.Bool("h", false, "treat input as HTML")
	streamInput := flags.Bool("S", false, "stream large XML files")
	versionInfo := flags.Bool("v", false, "output version information and exit")
	xpathQuery := flags.String("x", "", "XPath query to select nodes")

	if err := flags.Parse(args[1:]); err != nil {
		fmt.Fprintf(stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	if *versionInfo {
		fmt.Fprintf(stdout, "xq version %s\n", version)
		return 0
	}

	var inputs []io.Reader
	if flags.NArg() == 0 {
		inputs = append(inputs, stdin)
	} else {
		for _, filename := range flags.Args() {
			file, err := os.Open(filename)
			if err != nil {
				fmt.Fprintf(stderr, "Error opening file %s: %v\n", filename, err)
				return 1
			}
			defer file.Close()
			inputs = append(inputs, file)
		}
	}

	output, err := processInputs(inputs, *compactOutput, *rawOutput, *colorOutput, *nullInput, *slurpInput, *fromJSON, *toJSON, *htmlInput, *streamInput, *xpathQuery)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, output)
	return 0
}

func processInputs(inputs []io.Reader, compactOutput, rawOutput, colorOutput, nullInput, slurpInput, fromJSON, toJSON, htmlInput, streamInput bool, xpathQuery string) (string, error) {
	var docs []interface{}
	for _, input := range inputs {
		var doc interface{}
		var err error

		if nullInput {
			doc = nil
		} else if fromJSON {
			err = json.NewDecoder(input).Decode(&doc)
		} else if streamInput {
			doc, err = xml.Parse(input)
		} else if htmlInput {
			doc, err = xml.ParseHTML(input)
		} else {
			doc, err = xml.Parse(input)
		}

		if err != nil {
			return "", fmt.Errorf("error parsing input: %v", err)
		}

		if xpathQuery != "" {
			doc, err = xml.XPathQuery(doc, xpathQuery)
			if err != nil {
				return "", fmt.Errorf("error executing XPath query: %v", err)
			}
		}

		docs = append(docs, doc)
	}

	var result interface{}
	if slurpInput {
		result = docs
	} else if len(docs) == 1 {
		result = docs[0]
	} else {
		result = docs
	}

	var output string
	indent := "  "
	if compactOutput {
		indent = ""
	}

	if toJSON {
		jsonData, err := json.MarshalIndent(xml.ToJSON(result), "", indent)
		if err != nil {
			return "", fmt.Errorf("error converting to JSON: %v", err)
		}
		output = string(jsonData)
	} else {
		output = xml.Format(result, indent)
	}

	if rawOutput {
		output = strings.Trim(output, "\"")
		output = strings.ReplaceAll(output, "\\\"", "\"")
	}

	if colorOutput {
		output = xml.Colorize(output)
	}

	return output, nil
}
