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
		case *node:
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
			diags.Merge(node.n.diags)
			if node.n.args != nil {
				diags.Merge(typecheck.Block(node.n.block, node.n.args))
			}

			diags.Merge(validateGraph(node.state))
		}
	}

	return diags
}

type blockNode interface {
	Block() *ast.BlockStmt
}

func newNode(block *ast.BlockStmt) *node {
	return &node{
		id:    blockID(block),
		block: block,
	}
}

var (
	_ dag.Node  = (*node)(nil)
	_ blockNode = (*node)(nil)
)

// node is a generic node that can be added to the graph.
// We only perform type checking if args are not nil.
// We also store any diagnostics that are not related to type cheking on
// the node so we can render them in correct order.
type node struct {
	id    string
	args  any
	block *ast.BlockStmt
	diags diag.Diagnostics
}

// Block implements blockNode.
func (n *node) Block() *ast.BlockStmt {
	return n.block
}

func (n *node) NodeID() string {
	return n.id
}

func newComponentNode(block *ast.BlockStmt) *componentNode {
	return &componentNode{
		id:    blockID(block),
		block: block,
	}
}

var (
	_ dag.Node  = (*componentNode)(nil)
	_ blockNode = (*componentNode)(nil)
)

// componentNode is a node used for components where we need to delay
// certain checks until we have performed other ones.
type componentNode struct {
	id    string
	block *ast.BlockStmt
	diags diag.Diagnostics
}

func (c *componentNode) Block() *ast.BlockStmt {
	return c.block
}

func (c *componentNode) NodeID() string {
	return c.id
}

func newSubNode(n *node, s *state) *subNode {
	return &subNode{
		n:     n,
		state: s,
	}
}

var (
	_ dag.Node  = (*subNode)(nil)
	_ blockNode = (*subNode)(nil)
)

// subNode is used to delay certain checks of a sub graph until we have
// performed other ones.
type subNode struct {
	n *node
	*state
}

// Block implements blockNode.
func (s *subNode) Block() *ast.BlockStmt {
	return s.n.Block()
}

// NodeID implements dag.Node.
func (s *subNode) NodeID() string {
	return s.n.NodeID()
}
