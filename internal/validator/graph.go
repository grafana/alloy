package validator

import (
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/grafana/alloy/internal/dag"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/typecheck"
)

func newGraph() *graph {
	return &graph{
		Graph: &dag.Graph{},
	}
}

func newGraphWithParent(p *graph) *graph {
	return &graph{
		Graph:  &dag.Graph{},
		parent: p,
	}
}

// graph wrapps dag.Graph and keep track of insert order of nodes.
type graph struct {
	ids []string
	*dag.Graph
	parent *graph
}

func (g *graph) Add(n dag.Node) {
	g.ids = append(g.ids, n.NodeID())
	g.Graph.Add(n)
}

func (g *graph) GetByID(id string) dag.Node {
	if n := g.Graph.GetByID(id); n != nil {
		return n
	}

	if g.parent != nil {
		return g.parent.GetByID(id)
	}

	return nil
}

func (g *graph) IsFromParent(n dag.Node) bool {
	return g.Graph.GetByID(n.NodeID()) == nil && g.parent != nil && g.parent.GetByID(n.NodeID()) != nil
}

func (g *graph) Iter() iter.Seq[dag.Node] {
	return func(yield func(dag.Node) bool) {
		for _, id := range g.ids {
			if !yield(g.Graph.GetByID(id)) {
				return
			}
		}
	}
}

func validateGraph(s *state, minStability featuregate.Stability) diag.Diagnostics {
	var diags diag.Diagnostics
	for n := range s.graph.Iter() {
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
		case *moduleNode:
			diags.Merge(node.n.diags)
			if node.n.args != nil {
				diags.Merge(typecheck.Block(node.n.block, node.n.args))
			}
			diags.Merge(validateGraph(node.state, minStability))
		case *foreachNode:
			diags.Merge(node.n.diags)
			if node.n.args != nil {
				diags.Merge(typecheck.Block(node.n.block, node.n.args))
			}
			diags.Merge(validateGraph(node.state, minStability))
		}

		refs, refDiags := findReferences(n, s.graph, s.scope, minStability)
		diags.Merge(refDiags)
		for _, ref := range refs {
			// Nodes within a foreach block can reference nodes from outside of the block
			// but the other way around is not possible. So we use a "sub" graph of the parent so
			// we can find references from either it's own block or from the outside block.
			// Because of this we only have to validate edges within the foreach block.
			if !s.graph.IsFromParent(ref.Target) {
				s.graph.AddEdge(dag.Edge{From: n, To: ref.Target})
			}
		}
	}

	for _, cycle := range dag.StronglyConnectedComponentsOrdered(s.graph.Graph, s.graph.ids) {
		if len(cycle) > 1 {
			cycleStr := make([]string, len(cycle))

			// We need to sort the nodes to have consitent order in output.
			slices.SortFunc(cycle, func(a dag.Node, b dag.Node) int {
				if a.NodeID() > b.NodeID() {
					return 1
				}
				return -1
			})

			for i, node := range cycle {
				cycleStr[i] = node.NodeID()
			}

			for _, node := range cycle {
				n := node.(blockNode)
				diags.Add(diag.Diagnostic{
					Severity: diag.SeverityLevelError,
					StartPos: ast.StartPos(n.Block()).Position(),
					EndPos:   n.Block().LCurlyPos.Position(),
					Message:  fmt.Sprintf("cycle detected: %s", strings.Join(cycleStr, ", ")),
				})
			}
		}
	}

	for _, e := range s.graph.Edges() {
		if e.From == e.To {
			n := e.From.(blockNode)
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: ast.StartPos(n.Block()).Position(),
				EndPos:   n.Block().LCurlyPos.Position(),
				Message:  "cannot reference self",
			})
		}
	}

	return diags
}

type blockNode interface {
	dag.Node
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

func newModuleNode(n *node, s *state) *moduleNode {
	return &moduleNode{
		n:     n,
		state: s,
	}
}

var (
	_ dag.Node  = (*moduleNode)(nil)
	_ blockNode = (*moduleNode)(nil)
)

// moduleNode is used to delay certain checks of a sub graph until we have
// performed other ones.
type moduleNode struct {
	n *node
	*state
}

func (m *moduleNode) Block() *ast.BlockStmt {
	return m.n.Block()
}

func (m *moduleNode) NodeID() string {
	return m.n.NodeID()
}

func newForeachNode(n *node, s *state) *foreachNode {
	return &foreachNode{
		n:     n,
		state: s,
	}
}

var (
	_ dag.Node  = (*foreachNode)(nil)
	_ blockNode = (*foreachNode)(nil)
)

type foreachNode struct {
	n *node
	*state
}

func (f *foreachNode) Block() *ast.BlockStmt {
	return f.n.Block()
}

func (f *foreachNode) NodeID() string {
	return f.n.NodeID()
}
