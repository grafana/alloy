package dag

type FilterFunc func(g *Graph, n Node) bool

func FilterAllFunc(_ *Graph, _ Node) bool {
	return true
}

func FilterLeavesFunc(g *Graph, n Node) bool {
	return len(g.outEdges[n]) == 0
}

func FilterRootsFunc(g *Graph, n Node) bool {
	return len(g.inEdges[n]) == 0
}

// WeaklyConnectedComponents returns the graph split into weakly connected
// components. Two nodes are in the same component if there is a path between them in either direction.
// Each node appears in exactly one component. The graph is unchanged.
func WeaklyConnectedComponents(g *Graph, f FilterFunc) [][]Node {
	visited := make(nodeSet)
	var components [][]Node

	for _, n := range g.Nodes() {
		if visited.Has(n) {
			continue
		}

		var (
			queue     = []Node{n}
			component = make([]Node, 0)
		)

		// BFS from n following both outgoing and incoming edges.
		visited.Add(n)
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]

			if f(g, curr) {
				component = append(component, curr)
			}

			for _, neighbor := range neighbors(g, curr) {
				if visited.Has(neighbor) {
					continue
				}
				visited.Add(neighbor)
				queue = append(queue, neighbor)
			}
		}

		components = append(components, component)
	}

	return components
}

// neighbors returns all nodes adjacent to n ignoring edge direction.
func neighbors(g *Graph, n Node) []Node {
	out := make([]Node, 0, len(g.outEdges[n])+len(g.inEdges[n]))
	for neighbor := range g.outEdges[n] {
		out = append(out, neighbor)
	}
	for neighbor := range g.inEdges[n] {
		out = append(out, neighbor)
	}
	return out
}
