package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/tmc/misc/xq/xml"
	"golang.org/x/term"
)

var (
	compactOutput = flag.Bool("c", false, "compact instead of pretty-printed output")
	rawOutput     = flag.Bool("r", false, "output raw strings, not JSON texts")
	colorOutput   = flag.Bool("C", false, "colorize JSON")
	nullInput     = flag.Bool("n", false, "use `null` as the single input value")
	slurpInput    = flag.Bool("s", false, "read (slurp) all inputs into an array")
	fromJSON      = flag.Bool("f", false, "input is JSON, not XML")
	toJSON        = flag.Bool("j", false, "output as JSON")
	htmlInput     = flag.Bool("h", false, "treat input as HTML")
	streamInput   = flag.Bool("S", false, "stream large XML files")
	versionInfo   = flag.Bool("v", false, "output version information and exit")
	xpathQuery    = flag.String("x", "", "XPath query to select nodes")
)

const version = "0.2.0"

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [file...]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *versionInfo {
		fmt.Printf("xq version %s\n", version)
		return
	}

	var inputs []io.Reader
	if flag.NArg() == 0 {
		if term.IsTerminal(int(os.Stdin.Fd())) {
			flag.Usage()
			return
		}
		inputs = append(inputs, os.Stdin)
	} else {
		for _, filename := range flag.Args() {
			file, err := os.Open(filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening file %s: %v\n", filename, err)
				os.Exit(1)
			}
			defer file.Close()
			inputs = append(inputs, file)
		}
	}

	output, err := processInputs(inputs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(output)
}

func processInputs(inputs []io.Reader) (string, error) {
	var docs []interface{}
	for _, input := range inputs {
		var doc interface{}
		var err error

		if *nullInput {
			doc = nil
		} else if *fromJSON {
			err = json.NewDecoder(input).Decode(&doc)
		} else if *streamInput {
			doc, err = xml.Parse(input)
		} else if *htmlInput {
			doc, err = xml.ParseHTML(input)
		} else {
			doc, err = xml.Parse(input)
		}

		if err != nil {
			return "", fmt.Errorf("error parsing input: %v", err)
		}

		log.Printf("Parsed document: %+v", doc)

		if *xpathQuery != "" {
			doc, err = xml.XPathQuery(doc, *xpathQuery)
			if err != nil {
				return "", fmt.Errorf("error executing XPath query: %v", err)
			}
		}

		docs = append(docs, doc)
	}

	var result interface{}
	if *slurpInput {
		result = docs
	} else if len(docs) == 1 {
		result = docs[0]
	} else {
		result = docs
	}

	var output string
	indent := "  "
	if *compactOutput {
		indent = ""
	}

	if *toJSON {
		jsonData, err := json.MarshalIndent(xml.ToJSON(result), "", indent)
		if err != nil {
			return "", fmt.Errorf("error converting to JSON: %v", err)
		}
		output = string(jsonData)
	} else {
		output = xml.Format(result, indent)
	}

	log.Printf("Formatted output: %s", output)

	if *rawOutput {
		output = strings.Trim(output, "\"")
		output = strings.ReplaceAll(output, "\\\"", "\"")
	}

	if *colorOutput {
		output = xml.Colorize(output)
	}

	return output, nil
}
