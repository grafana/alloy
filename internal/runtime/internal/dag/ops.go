package dag

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
)

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
