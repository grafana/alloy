package validator

import (
	"iter"

	"github.com/grafana/alloy/internal/dag"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
)

func newGraph() *orderedGraph {
	return &orderedGraph{
		Graph: &dag.Graph{},
	}
}

// orderedGraph wrapps dag.Graph and keep track of insert order of nodes.
type orderedGraph struct {
	ids []string
	*dag.Graph
}

func (g *orderedGraph) Add(n dag.Node) {
	g.ids = append(g.ids, n.NodeID())
	g.Graph.Add(n)
}

func (g *orderedGraph) Nodes() iter.Seq[*blockNode] {
	return func(yield func(*blockNode) bool) {
		for _, id := range g.ids {
			if !yield(g.Graph.GetByID(id).(*blockNode)) {
				return
			}
		}
	}
}

func newBlockNode(block *ast.BlockStmt) *blockNode {
	return &blockNode{
		id:    blockID(block),
		block: block,
	}
}

var _ dag.Node = (*blockNode)(nil)

// blockNode is used to insert any block into the graph.
// We only perform type checking if args are not nil.
// We also store any diagnostics that are not related to type cheking on
// the node so we can render them in correct order.
type blockNode struct {
	id    string
	args  any
	diags diag.Diagnostics
	block *ast.BlockStmt
}

func (n *blockNode) NodeID() string {
	return n.id
}
