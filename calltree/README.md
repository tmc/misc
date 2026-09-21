# calltree

Calltree renders and narrows call graphs as indented trees.

    go install github.com/tmc/misc/calltree@latest

It reads the line-oriented graph format used by
[digraph](https://pkg.go.dev/golang.org/x/tools/cmd/digraph) — the first word on
each line is a node, each later word a node it has an edge to — so it composes
with [callgraph](https://pkg.go.dev/golang.org/x/tools/cmd/callgraph) on one
side and `digraph` on the other.

Run over its own source:

    $ callgraph -algo=cha -format=$'{{.Caller}}\t{{.Callee}}' . |
        calltree -within=github.com/tmc/misc/calltree -trim=github.com/tmc/misc/calltree. main
    main
      -> run
        -> (*graph).roots
        -> (*graph).writeEdges
          -> label
          -> quote
        -> (*graph).writeTree
          -> (*tree).print
            -> (*tree).print ...cycle
            -> label
        -> read
          -> (*graph).add
          -> (*graph).addEdge
            -> (*graph).add
          -> compact
          -> in
          -> remove
          -> split
            -> fields

`-within` is what makes the output readable: a call graph over even a small
package is mostly standard-library edges, and an approximate algorithm such as
`cha` adds an edge for every possible dynamic target. Keeping only the edges
whose caller and callee are both yours typically removes more than nine tenths
of the graph. A method is named `(*path/to/pkg.T).M`, with the package path
inside the parentheses, so `-within` and `-trim` look past a leading `(*`.

Both flags take several prefixes, separated by commas or white space, so the
output of `go list -m` can be passed unedited — in a workspace it names every
module in the workspace, one per line:

    calltree -within="$(go list -m)"

A cycle prints as `...cycle` where it closes; a subtree already printed in full
prints as `...`. Output stays proportional to the graph rather than to the
number of paths through it.

## Composing with digraph

`-edges` writes the filtered graph back in the input format, so a query can sit
in the middle of a pipeline:

    callgraph -algo=cha -format=$'{{.Caller}}\t{{.Callee}}' ./... |
        calltree -within=$(go list -m) -edges |
        digraph focus github.com/you/mod/catalog.Lookup |
        calltree

## Comparing production and test call paths

The two graphs `callgraph` produces with and without `-test` differ exactly
where a test substitutes an implementation, which makes their difference a map
of a package's seams:

    mod=$(go list -m)
    for mode in prod test; do
        [ "$mode" = test ] && t=-test || t=
        callgraph -algo=cha $t -format=$'{{.Caller}}\t{{.Callee}}' ./catalog |
            calltree -within=$mod -trim=$mod/ -edges -tag=$mode >call-graph.$mode.txt
    done
    diff call-graph.prod.txt call-graph.test.txt

## Two traps in the input

Both are silent, and `calltree` handles both.

Generic instantiations contain spaces — `New[func(i int) error]` is one
function — so splitting `callgraph` output on whitespace corrupts the graph, as
`digraph` itself does. Separate the two columns with a tab (`calltree` splits on
it when present) and `calltree` quotes such names on the way back out.

`callgraph -format='{{.Caller}}\t{{.Callee}}'` writes a literal backslash-t: the
format is a Go template, not a shell string. Use `$'...'` to get a real tab.
Calltree also accepts the literal form rather than reporting one enormous node.

Finally, `callgraph` writes the edges it *did* resolve even when it fails to
load some packages, so a stale binary or one unbuildable directory yields a
large, plausible graph holding none of your own code. Calltree treats an empty
result after `-within` as an error for that reason.
