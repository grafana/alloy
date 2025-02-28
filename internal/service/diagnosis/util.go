package diagnosis

import (
	"strings"

	"github.com/grafana/alloy/internal/component"
)

type node struct {
	info        *component.Info
	connections []*node
}

type edge struct {
	from *node
	to   *node
}

func (d *diagnosis) containsNode(componentName string) bool {
	_, ok := d.tree[componentName]
	return ok
}

func (d *diagnosis) containsEdge(componentName1 string, componentName2 string) bool {
	nodes, ok := d.tree[componentName1]
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

func (d *diagnosis) getEdges(componentName1 string, componentName2 string) []*edge {
	nodes, ok := d.tree[componentName1]
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

func (d *diagnosis) containsNamespace(namespace string) bool {
	for _, node := range d.nodes {
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
