package dag

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
)

// Reduce performs a transitive reduction on g. A transitive reduction removes
// as many edges as possible while maintaining the same "reachability" as the
// original graph: any node N reachable from node S will still be reachable
// after a reduction.
func Reduce(g *Graph) {
	// A direct edge between two vertices can be removed if that same target
	// vertex is indirectly reachable through another edge.
	//
	// To detect this, we iterate through all vertices in the graph, performing a
	// depth-first search at its dependencies. If the target vertex is reachable
	// from the source vertex, the edge is removed.
	for source := range g.nodes {
		_ = Walk(g, g.Dependencies(source), func(direct Node) error {
			// Iterate over (direct, indirect) edges and remove (source, indirect)
			// edges if they exist. This is a safe operation because other is still
			// reachable by source via its (source, direct) edge.
			for indirect := range g.outEdges[direct] {
				g.RemoveEdge(Edge{From: source, To: indirect})
			}
			return nil
		})
	}
}

// Validate checks that the graph doesn't contain cycles
func Validate(g *Graph) error {
	var err error

	// Check cycles using strongly connected components algorithm
	for _, cycle := range StronglyConnectedComponents(g) {
		if len(cycle) > 1 {
			cycleStr := make([]string, len(cycle))
			for i, node := range cycle {
				cycleStr[i] = node.NodeID()
			}
			err = multierror.Append(err, fmt.Errorf("cycle: %s", strings.Join(cycleStr, ", ")))
		}
	}

	// Check self references
	for _, e := range g.Edges() {
		if e.From == e.To {
			err = multierror.Append(err, fmt.Errorf("self reference: %s", e.From.NodeID()))
		}
	}

	return err
}
