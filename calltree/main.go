package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

var (
	depth  = flag.Int("depth", 0, "print at most `n` levels below each root; 0 is unlimited")
	within = flag.String("within", "", "keep only edges whose caller and callee are both in `prefix`")
	trim   = flag.String("trim", "", "remove `prefix` wherever it appears in a node name")
	tag    = flag.String("tag", "", "begin every output line with \"`name`: \"")
	edges  = flag.Bool("edges", false, "write the filtered edges instead of a tree")
	file   = flag.String("f", "", "read the graph from `file` instead of standard input")
)

func main() {
	log.SetPrefix("calltree: ")
	log.SetFlags(0)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: calltree [-f file] [-depth n] [-within prefix] [-trim prefix] [-tag name] [-edges] [root...]")
		flag.PrintDefaults()
	}
	flag.Parse()
	in := io.Reader(os.Stdin)
	if *file != "" {
		f, err := os.Open(*file)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		in = f
	}
	if err := run(in, os.Stdout, flag.Args()); err != nil {
		log.Fatal(err)
	}
}

func run(r io.Reader, w io.Writer, roots []string) error {
	g, err := read(r, *within, *trim)
	if err != nil {
		return err
	}
	if g.edges == 0 {
		if *within != "" {
			return fmt.Errorf("no edge has both ends within %q: callgraph writes the edges it did "+
				"resolve even when it fails to load packages, so check its exit status and build tags", *within)
		}
		return fmt.Errorf("no edges in input")
	}
	bw := bufio.NewWriter(w)
	if *edges {
		g.writeEdges(bw, *tag)
		return bw.Flush()
	}
	if len(roots) == 0 {
		roots = g.roots()
		if len(roots) == 0 {
			return fmt.Errorf("every node has a predecessor, so there is no root to start from; name one")
		}
	}
	for _, root := range roots {
		if _, ok := g.succs[root]; !ok {
			return fmt.Errorf("no node named %q (use -trim if the names carry a module prefix)", root)
		}
	}
	g.writeTree(bw, roots, *depth, *tag)
	return bw.Flush()
}

// A graph is a call graph in adjacency-list form.
type graph struct {
	succs   map[string][]string // successors of each node, sorted and deduplicated
	hasPred map[string]bool     // nodes with at least one predecessor
	edges   int                 // edges retained
}

// read parses edges from r, keeping only those whose endpoints are both in the
// package within, then removing trim from each node name.
func read(r io.Reader, within, trim string) (*graph, error) {
	g := &graph{succs: make(map[string][]string), hasPred: make(map[string]bool)}
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 16<<20)
	for line := 1; s.Scan(); line++ {
		words, err := split(s.Text())
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", line, err)
		}
		if len(words) == 0 {
			continue
		}
		from := words[0]
		if !in(from, within) {
			continue
		}
		from = remove(from, trim)
		g.add(from)
		for _, to := range words[1:] {
			if !in(to, within) {
				continue
			}
			g.addEdge(from, remove(to, trim))
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	for n, succs := range g.succs {
		sort.Strings(succs)
		g.succs[n] = compact(succs)
	}
	return g, nil
}

// in reports whether the function named n belongs to the package prefix. A
// method is named "(*path/to/pkg.T).M" and an interface method "(path/to/pkg.I).M",
// so the name is considered after any leading parenthesis and star.
func in(n, prefix string) bool {
	return strings.HasPrefix(strings.TrimLeft(n, "(*"), prefix)
}

// remove deletes prefix from n wherever it appears, since a method embeds its
// package path inside parentheses rather than at the start of its name.
func remove(n, prefix string) string {
	if prefix == "" {
		return n
	}
	return strings.ReplaceAll(n, prefix, "")
}

func (g *graph) add(n string) {
	if _, ok := g.succs[n]; !ok {
		g.succs[n] = nil
	}
}

func (g *graph) addEdge(from, to string) {
	g.succs[from] = append(g.succs[from], to)
	g.add(to)
	g.hasPred[to] = true
	g.edges++
}

// roots returns the nodes no edge points to, sorted.
func (g *graph) roots() []string {
	var roots []string
	for n := range g.succs {
		if !g.hasPred[n] {
			roots = append(roots, n)
		}
	}
	sort.Strings(roots)
	return roots
}

func (g *graph) writeEdges(w io.Writer, tag string) {
	from := make([]string, 0, len(g.succs))
	for n := range g.succs {
		from = append(from, n)
	}
	sort.Strings(from)
	for _, n := range from {
		for _, to := range g.succs[n] {
			fmt.Fprintf(w, "%s%s %s\n", label(tag), quote(n), quote(to))
		}
	}
}

func (g *graph) writeTree(w io.Writer, roots []string, depth int, tag string) {
	t := &tree{w: w, g: g, depth: depth, tag: tag, path: make(map[string]bool), done: make(map[string]bool)}
	for _, root := range roots {
		t.print(root, 0)
	}
}

type tree struct {
	w     io.Writer
	g     *graph
	depth int
	tag   string
	path  map[string]bool // ancestors on the current branch, to cut cycles
	done  map[string]bool // nodes already printed in full, to elide repeats
}

func (t *tree) print(n string, level int) {
	arrow := ""
	if level > 0 {
		arrow = "-> "
	}
	indent := label(t.tag) + strings.Repeat("  ", level) + arrow
	switch {
	case t.path[n]:
		fmt.Fprintf(t.w, "%s%s ...cycle\n", indent, n)
		return
	case t.done[n] && len(t.g.succs[n]) > 0:
		fmt.Fprintf(t.w, "%s%s ...\n", indent, n)
		return
	}
	fmt.Fprintf(t.w, "%s%s\n", indent, n)
	if t.depth > 0 && level >= t.depth {
		return
	}
	t.path[n] = true
	for _, s := range t.g.succs[n] {
		t.print(s, level+1)
	}
	delete(t.path, n)
	t.done[n] = true
}

func label(tag string) string {
	if tag == "" {
		return ""
	}
	return tag + ": "
}

// quote renders a node name as one word of the input format.
func quote(n string) string {
	if strings.ContainsAny(n, " \t\"") {
		return strconv.Quote(n)
	}
	return n
}

// split divides a line into words. A line holding a tab is split on tabs, and
// one holding the two characters \t is split on those, because a callgraph
// format string written with '\t' rather than $'\t' emits them literally. Every
// other line is split on unquoted whitespace, as digraph does, so that a name
// holding a space — a generic instantiation such as New[func(i int) error] —
// survives if it is quoted.
func split(line string) ([]string, error) {
	if i := strings.IndexByte(line, '#'); i == 0 {
		return nil, nil
	}
	if strings.ContainsRune(line, '\t') {
		return fields(strings.Split(line, "\t")), nil
	}
	if strings.Contains(line, `\t`) {
		return fields(strings.Split(line, `\t`)), nil
	}
	var words []string
	for len(line) > 0 {
		line = strings.TrimLeft(line, " ")
		if len(line) == 0 {
			break
		}
		if line[0] == '"' {
			end := 1
			for end < len(line) && (line[end] != '"' || line[end-1] == '\\') {
				end++
			}
			if end >= len(line) {
				return nil, fmt.Errorf("unterminated quotation")
			}
			word, err := strconv.Unquote(line[:end+1])
			if err != nil {
				return nil, err
			}
			words, line = append(words, word), line[end+1:]
			continue
		}
		i := strings.IndexByte(line, ' ')
		if i < 0 {
			return append(words, line), nil
		}
		words, line = append(words, line[:i]), line[i:]
	}
	return words, nil
}

func fields(words []string) []string {
	out := words[:0]
	for _, w := range words {
		if w = strings.TrimSpace(w); w != "" {
			out = append(out, w)
		}
	}
	return out
}

func compact(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}
