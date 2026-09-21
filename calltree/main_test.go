package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{"space", "a b c", []string{"a", "b", "c"}},
		{"tab", "a\tb", []string{"a", "b"}},
		{"literal backslash t", `a\tb`, []string{"a", "b"}},
		{"quoted space", `a "New[func(i int) error]"`, []string{"a", "New[func(i int) error]"}},
		{"tab keeps unquoted space", "main\tNew[func(i int) error]", []string{"main", "New[func(i int) error]"}},
		{"comment", "# a b", nil},
		{"blank", "   ", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := split(tt.line)
			if err != nil {
				t.Fatalf("split(%q) = error %v", tt.line, err)
			}
			if len(got) != len(tt.want) || (len(got) > 0 && !reflect.DeepEqual(got, tt.want)) {
				t.Errorf("split(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestSplitUnterminated(t *testing.T) {
	if _, err := split(`a "b`); err == nil {
		t.Error(`split("a \"b") = nil error, want one`)
	}
}

func TestIn(t *testing.T) {
	const pkg = "example.com/m"
	tests := []struct {
		node string
		want bool
	}{
		{"example.com/m.F", true},
		{"(*example.com/m.T).M", true},
		{"(example.com/m.I).M", true},
		{"example.com/m.F$1", true},
		{"example.com/other.F", false},
		{"fmt.Println", false},
		{"(*os.File).Read", false},
	}
	for _, tt := range tests {
		if got := in(tt.node, pkg); got != tt.want {
			t.Errorf("in(%q, %q) = %v, want %v", tt.node, pkg, got, tt.want)
		}
	}
}

func TestRemove(t *testing.T) {
	const pkg = "example.com/m."
	tests := []struct{ node, want string }{
		{"example.com/m.F", "F"},
		{"(*example.com/m.T).M", "(*T).M"},
		{"fmt.Println", "fmt.Println"},
	}
	for _, tt := range tests {
		if got := remove(tt.node, pkg); got != tt.want {
			t.Errorf("remove(%q, %q) = %q, want %q", tt.node, pkg, got, tt.want)
		}
	}
}

func TestRead(t *testing.T) {
	const in = `example.com/m.main	example.com/m.run
example.com/m.main	fmt.Println
example.com/m.run	(*example.com/m.T).M
runtime.main	example.com/m.main
example.com/m.run	example.com/m.run
`
	g, err := read(strings.NewReader(in), "example.com/m", "example.com/m.")
	if err != nil {
		t.Fatal(err)
	}
	if g.edges != 3 {
		t.Errorf("edges = %d, want 3 (stdlib caller and callee dropped)", g.edges)
	}
	if want := []string{"main"}; !reflect.DeepEqual(g.roots(), want) {
		t.Errorf("roots = %q, want %q", g.roots(), want)
	}
	if want := []string{"(*T).M", "run"}; !reflect.DeepEqual(g.succs["run"], want) {
		t.Errorf("succs[run] = %q, want %q", g.succs["run"], want)
	}
}

func TestReadDeduplicates(t *testing.T) {
	g, err := read(strings.NewReader("a b\na b\na b\n"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"b"}; !reflect.DeepEqual(g.succs["a"], want) {
		t.Errorf("succs[a] = %q, want %q", g.succs["a"], want)
	}
}
