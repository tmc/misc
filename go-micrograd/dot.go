package micrograd

import (
	"fmt"

	"github.com/tmc/dot"
)

func trace(root *Value) (map[*Value]struct{}, map[*Value][]*Value) {
	nodes := make(map[*Value]struct{})
	edges := make(map[*Value][]*Value)
	var build func(*Value)
	build = func(v *Value) {
		if _, exists := nodes[v]; !exists {
			nodes[v] = struct{}{}
			for _, child := range v._prev {
				edges[v] = append(edges[v], child)
				build(child)
			}
		}
	}
	build(root)
	return nodes, edges
}

func DrawDot(root *Value) string {
	nodes, edges := trace(root)
	g := dot.NewGraph("G")
	g.SetType(dot.DIGRAPH)
	g.Set("rankdir", "LR")

	for n := range nodes {
		nodeID := fmt.Sprintf("%p", n)
		node, _ := g.AddNode(dot.NewNode(nodeID))
		node.Set("shape", "record")
		node.Set("label", fmt.Sprintf("{ %v | data %.4f | grad %.4f }", n._label, n.data, n.grad))

		if n._op != "" {
			opNodeID := fmt.Sprintf("%p%s", n, n._op)
			opNode, _ := g.AddNode(dot.NewNode(opNodeID))
			opNode.Set("label", string(n._op))
			g.AddEdge(dot.NewEdge(opNode, node))
		}
	}

	for parent, children := range edges {
		for _, child := range children {
			childOpNodeID := fmt.Sprintf("%p%s", parent, parent._op)
			g.AddEdge(dot.NewEdge(dot.NewNode(fmt.Sprintf("%p", child)), dot.NewNode(childOpNodeID)))
		}
	}

	return g.String()
}
