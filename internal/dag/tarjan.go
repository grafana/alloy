package dag

// StronglyConnectedComponents returns the list of strongly connected components
// of the graph using Tarjan algorithm.
func StronglyConnectedComponents(g *Graph) [][]Node {
	nodes := g.Nodes()
	t := tarjan{
		nodes:   make(map[Node]int, len(nodes)),
		lowLink: make(map[Node]int, len(nodes)),
	}

	for _, n := range g.Nodes() {
		// Calculate strong connect components for non-visited nodes
		if t.nodes[n] == 0 {
			t.tarjan(g, n)
		}
	}
	return t.result
}

// tarjan represents Tarjan algorithm to find
// strongly connected component finding.
//
// https://en.wikipedia.org/wiki/Tarjan%27s_strongly_connected_components_algorithm
type tarjan struct {
	index   int
	nodes   map[Node]int
	lowLink map[Node]int
	stack   []Node

	result [][]Node
}

func (t *tarjan) tarjan(g *Graph, n Node) {
	t.visit(n)
	for succ := range g.outEdges[n] {
		if t.nodes[succ] == 0 {
			// Successor not visited, recurse on it
			t.tarjan(g, succ)
			t.lowLink[n] = min(t.lowLink[n], t.lowLink[succ])
		} else if t.onStack(succ) {
			// Successor is in stack and hence in the current SCC
			t.lowLink[n] = min(t.lowLink[n], t.nodes[succ])
		}
	}

	// If n is a root node, pop the stack and generate an SCC
	if t.lowLink[n] == t.nodes[n] {
		// Start a new strongly connected component
		var scc []Node
		for {
			succ := t.pop()
			// Add w to current strongly connected component.
			scc = append(scc, succ)
			if succ == n {
				break
			}
		}
		// Add current strongly connected component to result
		t.result = append(t.result, scc)
	}
}

// visit marks node as visited and pushes to the stack
func (t *tarjan) visit(n Node) {
	t.index++
	t.nodes[n] = t.index
	t.lowLink[n] = t.index
	t.push(n)
}

func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

// push adds a node to the stack
func (t *tarjan) push(n Node) {
	t.stack = append(t.stack, n)
}

// pop removes a node from the stack
func (t *tarjan) pop() Node {
	n := len(t.stack)
	if n == 0 {
		return nil
	}
	node := t.stack[n-1]
	t.stack = t.stack[:n-1]
	return node
}

// onStack checks if node is in stack
func (t *tarjan) onStack(n Node) bool {
	for _, e := range t.stack {
		if n == e {
			return true
		}
	}
	return false
}
