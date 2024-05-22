// Package dag defines a Directed Acyclic Graph.
package dag

import "fmt"

// Node is an individual Vertex in the DAG.
type Node interface {
	// NodeID returns the display name of the Node.
	NodeID() string
}

// Edge is a directed connection between two Nodes.
type Edge struct{ From, To Node }

// Graph is a Directed Acyclic Graph. The zero value is ready for use. Graph
// cannot be modified concurrently.
type Graph struct {
	nodeByID map[string]Node
	nodes    nodeSet
	outEdges map[Node]nodeSet // Outgoing edges for a given Node
	inEdges  map[Node]nodeSet // Incoming edges for a given Node
}

type nodeSet map[Node]struct{}

// Add adds n into ns if it doesn't already exist.
func (ns nodeSet) Add(n Node) { ns[n] = struct{}{} }

// Remove removes n from ns if it exists.
func (ns nodeSet) Remove(n Node) { delete(ns, n) }

// Has returns true if ns includes n.
func (ns nodeSet) Has(n Node) bool {
	_, ok := ns[n]
	return ok
}

// Clone returns a copy of ns.
func (ns nodeSet) Clone() nodeSet {
	newSet := make(nodeSet, len(ns))
	for node := range ns {
		newSet[node] = struct{}{}
	}
	return newSet
}

// init prepares g for writing.
func (g *Graph) init() {
	if g.nodeByID == nil {
		g.nodeByID = make(map[string]Node)
	}
	if g.nodes == nil {
		g.nodes = make(nodeSet)
	}
	if g.outEdges == nil {
		g.outEdges = make(map[Node]nodeSet)
	}
	if g.inEdges == nil {
		g.inEdges = make(map[Node]nodeSet)
	}
}

// Add adds a new Node into g. Add is a no-op if n already exists in g.
//
// Add will panic if there is another node in g with the same NodeID as n.
func (g *Graph) Add(n Node) {
	g.init()

	if other, ok := g.nodeByID[n.NodeID()]; ok && other != n {
		panic(fmt.Sprintf("Graph.Add: Node ID %q is already in use by another Node", n.NodeID()))
	}
	g.nodes.Add(n)
	g.nodeByID[n.NodeID()] = n
}

// GetByID returns a node from an ID. Returns nil if the ID does not exist in
// the graph.
func (g *Graph) GetByID(id string) Node { return g.nodeByID[id] }

// Remove removes a Node from g. Remove is a no-op if n does not exist in g.
//
// Remove also removes any edge to or from n.
func (g *Graph) Remove(n Node) {
	if !g.nodes.Has(n) {
		return
	}

	delete(g.nodeByID, n.NodeID())
	g.nodes.Remove(n)

	// Remove all the outgoing edges from n.
	delete(g.outEdges, n)

	// Remove n from any edge where it is the target.
	for _, ns := range g.inEdges {
		ns.Remove(n)
	}
}

// AddEdge adds a new Edge into g. AddEdge does not prevent cycles from being
// introduced; cycles must be detected separately.
//
// AddEdge will panic if either node in the edge doesn't exist in g.
func (g *Graph) AddEdge(e Edge) {
	g.init()

	if !g.nodes.Has(e.From) || !g.nodes.Has(e.To) {
		panic("AddEdge called with a node that doesn't exist in graph")
	}

	inSet, ok := g.inEdges[e.To]
	if !ok {
		inSet = make(nodeSet)
		g.inEdges[e.To] = inSet
	}
	inSet.Add(e.From)

	outSet, ok := g.outEdges[e.From]
	if !ok {
		outSet = make(nodeSet)
		g.outEdges[e.From] = outSet
	}
	outSet.Add(e.To)
}

// RemoveEdge removes an edge e from g. RemoveEdge is a no-op if e doesn't
// exist in g.
func (g *Graph) RemoveEdge(e Edge) {
	inSet, ok := g.inEdges[e.To]
	if ok {
		delete(inSet, e.From)
	}

	outSet, ok := g.outEdges[e.From]
	if ok {
		delete(outSet, e.To)
	}
}

// Nodes returns the set of Nodes in g.
func (g *Graph) Nodes() []Node {
	nodes := make([]Node, 0, len(g.nodes))
	for n := range g.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// Edges returns the set of all edges in g.
func (g *Graph) Edges() []Edge {
	var edges []Edge
	for from, tos := range g.outEdges {
		for to := range tos {
			edges = append(edges, Edge{From: from, To: to})
		}
	}
	return edges
}

// Dependants returns the list of Nodes that depend on n: all Nodes for which
// an edge to n is defined.
func (g *Graph) Dependants(n Node) []Node {
	sourceDependants := g.inEdges[n]
	dependants := make([]Node, 0, len(sourceDependants))
	for dep := range sourceDependants {
		dependants = append(dependants, dep)
	}
	return dependants
}

// Dependencies returns the list of Nodes that n depends on: all Nodes for
// which an edge from n is defined.
func (g *Graph) Dependencies(n Node) []Node {
	sourceDependencies := g.outEdges[n]
	dependencies := make([]Node, 0, len(sourceDependencies))
	for dep := range sourceDependencies {
		dependencies = append(dependencies, dep)
	}
	return dependencies
}

// Roots returns the set of Nodes in g that have no dependants. This is useful
// for walking g.
func (g *Graph) Roots() []Node {
	var res []Node

	for n := range g.nodes {
		if len(g.inEdges[n]) == 0 {
			res = append(res, n)
		}
	}

	return res
}

// Leaves returns the set of Nodes in g that have no dependencies. This is
// useful for walking g in reverse.
func (g *Graph) Leaves() []Node {
	var res []Node

	for n := range g.nodes {
		if len(g.outEdges[n]) == 0 {
			res = append(res, n)
		}
	}

	return res
}

// Clone returns a copy of g.
func (g *Graph) Clone() *Graph {
	newGraph := &Graph{
		nodes: g.nodes.Clone(),

		nodeByID: make(map[string]Node, len(g.nodeByID)),
		outEdges: make(map[Node]nodeSet, len(g.outEdges)),
		inEdges:  make(map[Node]nodeSet, len(g.outEdges)),
	}

	for key, value := range g.nodeByID {
		newGraph.nodeByID[key] = value
	}
	for node, set := range g.outEdges {
		newGraph.outEdges[node] = set.Clone()
	}
	for node, set := range g.inEdges {
		newGraph.inEdges[node] = set.Clone()
	}
	return newGraph
}
