// Command webidl2plan9 reads Web IDL and writes a Plan 9 / Wanix service(4)
// manpage skeleton — the deterministic part of a synthetic-filesystem API
// design. It extracts IDL from a Bikeshed (.bs) <xmp class="idl"> block or
// reads raw .idl, then renders the tree, per-file semantics, feature-mapping
// table, and fails-closed boundary that the IDL fully determines.
//
// Usage:
//
//	webidl2plan9 -name "Chrome Prompt API" -root /llm input.idl
//	webidl2plan9 -name "WebNN" -root /nn spec.bs > nn(4).md
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/tmc/misc/webidl2plan9"
)

var reIDLBlock = regexp.MustCompile(`(?s)<xmp[^>]*class="?idl"?[^>]*>(.*?)</xmp>|<pre[^>]*class="?idl"?[^>]*>(.*?)</pre>`)

func main() {
	name := flag.String("name", "", "human API name for the manpage title (required)")
	root := flag.String("root", "", "service root, e.g. /llm (required)")
	flag.Parse()

	if *name == "" || *root == "" {
		fmt.Fprintln(os.Stderr, "usage: webidl2plan9 -name <api name> -root /<service> [file.idl|file.bs]")
		os.Exit(2)
	}
	if !strings.HasPrefix(*root, "/") {
		fmt.Fprintf(os.Stderr, "root must start with '/': %s\n", *root)
		os.Exit(2)
	}

	var raw []byte
	var err error
	if flag.NArg() > 0 {
		raw, err = os.ReadFile(flag.Arg(0))
	} else {
		raw, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "read input: %v\n", err)
		os.Exit(1)
	}

	idlSrc := extractIDL(string(raw))
	idl, err := webidl2plan9.Parse(idlSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(webidl2plan9.Render(idl, *name, *root))
}

// extractIDL pulls IDL out of a Bikeshed/HTML <xmp class=idl>/<pre class=idl>
// block when present; otherwise it returns the input unchanged (raw .idl).
func extractIDL(s string) string {
	ms := reIDLBlock.FindAllStringSubmatch(s, -1)
	if len(ms) == 0 {
		return s
	}
	var b strings.Builder
	for _, m := range ms {
		block := m[1]
		if block == "" {
			block = m[2]
		}
		b.WriteString(block)
		b.WriteString("\n")
	}
	return b.String()
}
