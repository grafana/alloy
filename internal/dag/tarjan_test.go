package dag

import (
	"reflect"
	"sort"
	"testing"
)

func TestGraphStronglyConnected(t *testing.T) {
	var g Graph
	var (
		nodeA = stringNode("a")
		nodeB = stringNode("b")
	)
	g.Add(nodeA)
	g.Add(nodeB)
	g.AddEdge(Edge{nodeA, nodeB})
	g.AddEdge(Edge{nodeB, nodeA})

	actual := sortSlice(StronglyConnectedComponents(&g))
	expected := [][]Node{{nodeA, nodeB}}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("error calculating strongly connected components: expected %s, got %s", expected, actual)
	}
}

func TestGraphNotStronglyConnected(t *testing.T) {
	var g Graph
	var (
		nodeA = stringNode("a")
		nodeB = stringNode("b")
	)
	g.Add(nodeA)
	g.Add(nodeB)
	g.AddEdge(Edge{nodeA, nodeB})

	actual := sortSlice(StronglyConnectedComponents(&g))
	expected := [][]Node{{nodeA}, {nodeB}}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("error calculating strongly connected components: expected %s, got %s", expected, actual)
	}
}

func TestGraphStronglyConnectedMulti(t *testing.T) {
	var g Graph
	var (
		nodeA = stringNode("a")
		nodeB = stringNode("b")
		nodeC = stringNode("c")
		nodeD = stringNode("d")
		nodeE = stringNode("e")
	)
	g.Add(nodeA)
	g.Add(nodeB)
	g.AddEdge(Edge{nodeA, nodeB})
	g.AddEdge(Edge{nodeB, nodeA})
	g.Add(nodeC)
	g.Add(nodeD)
	g.Add(nodeE)
	g.AddEdge(Edge{nodeC, nodeD})
	g.AddEdge(Edge{nodeD, nodeE})
	g.AddEdge(Edge{nodeE, nodeC})

	actual := sortSlice(StronglyConnectedComponents(&g))
	expected := [][]Node{{nodeA, nodeB}, {nodeC, nodeD, nodeE}}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("error calculating strongly connected components: expected %s, got %s", expected, actual)
	}
}

func sortSlice(nodeSets [][]Node) [][]Node {
	var sorted [][]Node

	// Sort nodes of the detected cycle by id
	for _, ns := range nodeSets {
		sort.Slice(ns, func(i, j int) bool {
			return ns[i].NodeID() < ns[j].NodeID()
		})
		sorted = append(sorted, ns)
	}
	// Sort the final slice by the first element of the cycle
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i][0].NodeID() < sorted[j][0].NodeID()
	})
	return sorted
}

type stringNode string

func (s stringNode) NodeID() string { return string(s) }
