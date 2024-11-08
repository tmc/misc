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

	var docs []interface{}
	for _, input := range inputs {
		var doc interface{}
		var err error

		if *nullInput {
			doc = nil
		} else if *fromJSON {
			err = json.NewDecoder(input).Decode(&doc)
		} else if *streamInput {
			doc, err = xml.StreamParse(input)
		} else if *htmlInput {
			doc, err = xml.ParseHTML(input)
		} else {
			doc, err = xml.Parse(input)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing input: %v\n", err)
			os.Exit(1)
		}

		if *xpathQuery != "" {
			doc, err = xml.XPathQuery(doc, *xpathQuery)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error executing XPath query: %v\n", err)
				os.Exit(1)
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
			fmt.Fprintf(os.Stderr, "Error converting to JSON: %v\n", err)
			os.Exit(1)
		}
		output = string(jsonData)
	} else {
		output = xml.Format(result, indent)
	}

	if *rawOutput {
		output = strings.Trim(output, "\"")
		output = strings.ReplaceAll(output, "\\\"", "\"")
	}

	if *colorOutput {
		output = xml.Colorize(output)
	}

	fmt.Println(output)
}
