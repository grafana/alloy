package validator

import (
	"iter"

	"github.com/grafana/alloy/internal/dag"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/typecheck"
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

func (g *orderedGraph) Nodes() iter.Seq[dag.Node] {
	return func(yield func(dag.Node) bool) {
		for _, id := range g.ids {
			if !yield(g.Graph.GetByID(id)) {
				return
			}
		}
	}
}

func validateGraph(s *state) diag.Diagnostics {
	var diags diag.Diagnostics
	for n := range s.graph.Nodes() {
		switch node := n.(type) {
		case *blockNode:
			// Add any diagnostic for node that should be before type check.
			diags.Merge(node.diags)
			if node.args != nil {
				diags.Merge(typecheck.Block(node.block, node.args))
			}
		case *componentNode:
			name := node.block.GetBlockName()
			reg, err := s.cr.Get(name)
			if err != nil {
				node.diags.Add(diag.Diagnostic{
					Severity: diag.SeverityLevelError,
					StartPos: node.block.NamePos.Position(),
					EndPos:   node.block.NamePos.Add(len(name) - 1).Position(),
					Message:  err.Error(),
				})

				diags.Merge(node.diags)
				continue
			}

			diags.Merge(node.diags)
			if reg.Args == nil {
				continue
			}
			diags.Merge(typecheck.Block(node.block, reg.CloneArguments()))

		case *subNode:
			diags.Merge(validateGraph(node.state))
		}
	}

	return diags
}

func newBlockNode(block *ast.BlockStmt) *blockNode {
	return &blockNode{
		id:    blockID(block),
		block: block,
	}
}

var _ dag.Node = (*blockNode)(nil)

// blockNode is a generic node that can be added to the graph.
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

func newComponentNode(block *ast.BlockStmt) *componentNode {
	return &componentNode{
		id:    blockID(block),
		block: block,
	}
}

var _ dag.Node = (*componentNode)(nil)

// componentNode is a node used for components where we need to delay
// certain checks until we have performed other ones.
type componentNode struct {
	id    string
	diags diag.Diagnostics
	block *ast.BlockStmt
}

func (c *componentNode) NodeID() string {
	return c.id
}

func newSubNode(node dag.Node, s *state) *subNode {
	return &subNode{
		id:    node.NodeID() + "-sub",
		state: s,
	}
}

var _ dag.Node = (*subNode)(nil)

// subNode is used to delay certain checks of a sub graph until we have
// performed other ones.
type subNode struct {
	id string
	*state
}

// NodeID implements dag.Node.
func (s *subNode) NodeID() string {
	return s.id
}
