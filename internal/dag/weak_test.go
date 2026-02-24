package dag

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWeaklyConnectedComponents(t *testing.T) {
	t.Run("two disjoint graphs", func(t *testing.T) {
		var g Graph
		a, b, c, d := stringNode("a"), stringNode("b"), stringNode("c"), stringNode("d")
		g.Add(a)
		g.Add(b)
		g.Add(c)
		g.Add(d)
		g.AddEdge(Edge{From: a, To: b})
		g.AddEdge(Edge{From: c, To: d})

		got := WeaklyConnectedComponents(&g, FilterAllFunc)
		got = sortComponents(got)

		require.Equal(t, [][]Node{{a, b}, {c, d}}, got)
	})

	t.Run("single graph", func(t *testing.T) {
		// One component: a->b->c
		var g Graph
		a, b, c := stringNode("a"), stringNode("b"), stringNode("c")
		g.Add(a)
		g.Add(b)
		g.Add(c)
		g.AddEdge(Edge{From: a, To: b})
		g.AddEdge(Edge{From: b, To: c})

		got := WeaklyConnectedComponents(&g, FilterAllFunc)
		got = sortComponents(got)
		require.Equal(t, [][]Node{{a, b, c}}, got)
	})

	t.Run("isolated graphs", func(t *testing.T) {
		// Three isolated nodes
		var g Graph
		a, b, c := stringNode("a"), stringNode("b"), stringNode("c")
		g.Add(a)
		g.Add(b)
		g.Add(c)

		got := WeaklyConnectedComponents(&g, FilterAllFunc)
		got = sortComponents(got)
		require.Equal(t, [][]Node{{a}, {b}, {c}}, got)
	})

	t.Run("only leaves", func(t *testing.T) {
		var g Graph
		a, b, c, d := stringNode("a"), stringNode("b"), stringNode("c"), stringNode("d")
		g.Add(a)
		g.Add(b)
		g.Add(c)
		g.Add(d)
		g.AddEdge(Edge{From: a, To: b})
		g.AddEdge(Edge{From: c, To: d})

		got := WeaklyConnectedComponents(&g, FilterLeavesFunc)
		got = sortComponents(got)
		require.Equal(t, [][]Node{{b}, {d}}, got)
	})

	t.Run("only roots", func(t *testing.T) {
		var g Graph
		a, b, c, d := stringNode("a"), stringNode("b"), stringNode("c"), stringNode("d")
		g.Add(a)
		g.Add(b)
		g.Add(c)
		g.Add(d)
		g.AddEdge(Edge{From: a, To: b})
		g.AddEdge(Edge{From: c, To: d})

		got := WeaklyConnectedComponents(&g, FilterRootsFunc)
		got = sortComponents(got)
		require.Equal(t, [][]Node{{a}, {c}}, got)
	})
}

func sortComponents(components [][]Node) [][]Node {
	for _, c := range components {
		sort.Slice(c, func(i, j int) bool {
			return c[i].NodeID() < c[j].NodeID()
		})
	}
	sort.Slice(components, func(i, j int) bool {
		return components[i][0].NodeID() < components[j][0].NodeID()
	})
	return components
}
