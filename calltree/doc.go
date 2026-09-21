// Calltree renders and narrows call graphs as indented trees.
//
// Calltree reads a directed graph in the line-oriented format used by
// golang.org/x/tools/cmd/digraph — the first word on each line is a node, and
// each subsequent word is a node it has an edge to — and writes an indented
// tree rooted at each node named on the command line, or at every node with no
// predecessors if none are named.
//
// Its intended input is the output of golang.org/x/tools/cmd/callgraph:
//
//	callgraph -algo=cha -format='{{.Caller}} {{.Callee}}' ./pkg |
//		calltree -within=github.com/you/mod -trim=github.com/you/mod/
//
// The -within flag is what makes such output readable. A call graph over even a
// small package is mostly standard-library edges, and an approximate algorithm
// such as cha adds an edge for every possible dynamic target, so filtering to
// the edges whose caller and callee are both yours typically removes more than
// nine tenths of the graph.
//
// Usage:
//
//	calltree [-f file] [-depth n] [-within prefix] [-trim prefix] [-tag name] [-edges] [root...]
//
// It reads standard input unless -f names a file. The flags are:
//
//	-depth n
//		Print at most n levels below each root. The default, 0, is unlimited.
//	-within prefix
//		Keep only edges whose caller and callee both begin with prefix.
//	-trim prefix
//		Remove prefix from the start of every node name, after filtering.
//	-tag name
//		Begin every output line with "name: ", to label one graph of a set.
//	-edges
//		Write the filtered edges instead of a tree, in the input format, so
//		that the result can be piped to digraph or back to calltree.
//	-f file
//		Read the graph from file rather than from standard input.
//
// Calltree reports an error if no edge survives -within. An empty result is
// almost always a partial load rather than a package that makes no calls:
// callgraph prints the edges it did resolve even when it fails to load some
// packages, so a stale binary or one unbuildable directory yields a large graph
// with none of your own code in it.
//
// A cycle prints as "...cycle" at the node that closes it. A node whose subtree
// has already been printed in full prints as "...", so that the output stays
// proportional to the size of the graph rather than to the number of paths
// through it.
//
// # Examples
//
// The production and test call paths of a package, as two comparable files:
//
//	callgraph -algo=cha -format='{{.Caller}} {{.Callee}}' ./catalog |
//		calltree -within=$(go list -m) -trim=$(go list -m)/ -edges -tag=prod >call-graph.prod.txt
//	callgraph -algo=cha -test -format='{{.Caller}} {{.Callee}}' ./catalog |
//		calltree -within=$(go list -m) -trim=$(go list -m)/ -edges -tag=test >call-graph.test.txt
//
// The difference between them is the set of seams where a test substitutes an
// implementation.
//
// Narrowing with digraph, then rendering, using -edges to stay in one format:
//
//	callgraph -algo=cha -format='{{.Caller}} {{.Callee}}' ./... |
//		calltree -within=$(go list -m) -edges |
//		digraph focus github.com/you/mod/catalog.Lookup |
//		calltree
package main
