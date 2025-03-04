package diagnosis

import (
	"strings"

	"github.com/grafana/alloy/internal/component"
)

type graph struct {
	module string
	tree   map[string]map[string]*node // key is the component name, second key is the component id
	nodes  []*node
	roots  []*node

	clusteringEnabled bool
}

type node struct {
	info        *component.Info
	connections []*node
}

type edge struct {
	from *node
	to   *node
}

func newGraph(module string, clusteringEnabled bool) *graph {
	return &graph{
		module: module,
		tree:   make(map[string]map[string]*node, 0),
		nodes:  make([]*node, 0),
		roots:  make([]*node, 0),

		clusteringEnabled: clusteringEnabled,
	}
}

func (g *graph) build(components []*component.Info) {
	g.tree = make(map[string]map[string]*node, 0)
	g.nodes = make([]*node, 0)
	g.roots = make([]*node, 0)
	for _, c := range components {
		if _, ok := g.tree[c.ComponentName]; !ok {
			g.tree[c.ComponentName] = make(map[string]*node, 0)
		}
		node := &node{
			info:        c,
			connections: make([]*node, 0),
		}
		g.tree[c.ComponentName][c.ID.LocalID] = node
		g.nodes = append(g.nodes, node)
	}

	destNode := make(map[string]struct{})
	for _, c := range components {
		if strings.HasPrefix(c.ID.LocalID, "prometheus.exporter") || strings.HasPrefix(c.ID.LocalID, "discovery") {
			for _, ref := range c.ReferencedBy {
				refCpName := getNameFromID(ref)
				g.tree[c.ComponentName][c.ID.LocalID].connections = append(g.tree[c.ComponentName][c.ID.LocalID].connections, g.tree[refCpName][ref])
				destNode[ref] = struct{}{}
			}
		} else {
			for _, ref := range c.References {
				if strings.HasPrefix(ref, "prometheus.exporter") || strings.HasPrefix(ref, "discovery") {
					continue
				}
				refCpName := getNameFromID(ref)
				g.tree[c.ComponentName][c.ID.LocalID].connections = append(g.tree[c.ComponentName][c.ID.LocalID].connections, g.tree[refCpName][ref])
				destNode[ref] = struct{}{}
			}
		}
	}

	for _, node := range g.nodes {
		if _, ok := destNode[node.info.ID.LocalID]; !ok {
			g.roots = append(g.roots, node)
		}
	}
}

func (g *graph) containsNode(componentName string) bool {
	_, ok := g.tree[componentName]
	return ok
}

func (g *graph) getNodes(componentNames ...string) []*node {
	nodes := make([]*node, 0)
	for _, componentName := range componentNames {
		ids := g.tree[componentName]
		for _, id := range ids {
			nodes = append(nodes, id)
		}
	}
	return nodes
}

func (g *graph) containsEdge(componentName1 string, componentName2 string) bool {
	nodes, ok := g.tree[componentName1]
	if !ok {
		return false
	}
	for _, node := range nodes {
		result := searchNode(node, componentName2)
		if result != nil {
			return true
		}
	}
	return false
}

func (g *graph) getEdges(componentName1 string, componentName2 string) []*edge {
	nodes, ok := g.tree[componentName1]
	if !ok {
		return nil
	}
	edges := make([]*edge, 0)
	for _, node := range nodes {
		result := searchNode(node, componentName2)
		if result != nil {
			edges = append(edges, &edge{
				from: node,
				to:   result,
			})
		}
	}
	return edges
}

func (g *graph) containsNamespace(namespace string) bool {
	for _, node := range g.nodes {
		if strings.HasPrefix(node.info.ComponentName, namespace) {
			return true
		}
	}
	return false
}

func searchNode(root *node, componentName string) *node {
	if root.info.ComponentName == componentName {
		return root
	}
	for _, node := range root.connections {
		result := searchNode(node, componentName)
		if result != nil {
			return result
		}
	}
	return nil
}

func getNameFromID(s string) string {
	lastDotIndex := strings.LastIndex(s, ".")
	if lastDotIndex == -1 {
		return s
	}
	return s[:lastDotIndex]
}
