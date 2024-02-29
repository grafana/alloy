package dag

import "testing"

func TestValidateWithoutCycle(t *testing.T) {
	var g Graph
	var (
		nodeA = stringNode("a")
		nodeB = stringNode("b")
		nodeC = stringNode("c")
	)
	g.Add(nodeA)
	g.Add(nodeB)
	g.Add(nodeC)
	g.AddEdge(Edge{nodeC, nodeA})
	g.AddEdge(Edge{nodeC, nodeB})

	if err := Validate(&g); err != nil {
		t.Fatalf("non errors expected, got: %s", err)
	}
}

func TestValidateWithCycle(t *testing.T) {
	var g Graph
	var (
		nodeA = stringNode("a")
		nodeB = stringNode("b")
		nodeC = stringNode("c")
	)
	g.Add(nodeA)
	g.Add(nodeB)
	g.Add(nodeC)
	g.AddEdge(Edge{nodeC, nodeB})
	g.AddEdge(Edge{nodeC, nodeA})
	g.AddEdge(Edge{nodeA, nodeB})
	g.AddEdge(Edge{nodeB, nodeA})

	if err := Validate(&g); err == nil {
		t.Fatal("graph with cycles")
	}
}

func TestValidateSelfReference(t *testing.T) {
	var g Graph
	var (
		nodeA = stringNode("a")
	)
	g.Add(nodeA)
	g.AddEdge(Edge{nodeA, nodeA})

	if err := Validate(&g); err == nil {
		t.Fatal("graph with self reference")
	}
}
