package statictest

import (
	"fmt"
	"slices"
	"strings"

	"github.com/grafana/alloy/syntax/ast"
)

// pushExports are exports that other components send data to, such as
// loki.write.default.receiver in a forward_to list. A reference to one of
// these sends data from the component that holds the reference. A reference
// to any other export, such as discovery.kubernetes.pods.targets, sends data
// to the component that holds the reference.
var pushExports = map[string]bool{
	"receiver": true,
	"input":    true,
}

// dataFlow returns, for each component ID, the components it sends data to.
func dataFlow(c *config) map[string][]string {
	edges := map[string][]string{}
	add := func(from, to string) {
		if from != to && !slices.Contains(edges[from], to) {
			edges[from] = append(edges[from], to)
		}
	}
	for _, comp := range c.components {
		v := &refFinder{config: c}
		ast.Walk(v, comp.block.Body)
		for _, ref := range v.refs {
			if pushExports[ref.export] {
				add(comp.id, ref.id)
			} else {
				add(ref.id, comp.id)
			}
		}
	}
	return edges
}

type reference struct {
	id     string // the component referenced
	export string // the first part after its ID
}

// refFinder collects references to components of the config.
type refFinder struct {
	config *config
	refs   []reference
}

func (v *refFinder) Visit(node ast.Node) ast.Visitor {
	access, ok := node.(*ast.AccessExpr)
	if !ok {
		return v
	}
	parts, ok := identChain(access)
	if !ok {
		return v
	}
	// The longest prefix that names a component is the reference.
	for n := len(parts) - 1; n >= 2; n-- {
		id := strings.Join(parts[:n], ".")
		if _, ok := v.config.byID[id]; ok {
			v.refs = append(v.refs, reference{id: id, export: parts[n]})
			return nil
		}
	}
	return nil
}

// identChain flattens an access expression made only of identifiers, such as
// loki.write.default.receiver, into its parts.
func identChain(expr ast.Expr) ([]string, bool) {
	switch e := expr.(type) {
	case *ast.IdentifierExpr:
		return []string{e.Ident.Name}, true
	case *ast.AccessExpr:
		parts, ok := identChain(e.Value)
		if !ok {
			return nil, false
		}
		return append(parts, e.Name.Name), true
	default:
		return nil, false
	}
}

// checkFlowsThrough requires every path of data from a From component to a
// To component to pass a Through component. It reports one path for each
// pair that doesn't.
func checkFlowsThrough(c *config, chk Check) []string {
	edges := dataFlow(c)
	through := map[string]bool{}
	for _, comp := range c.selectComponents(chk.Through) {
		through[comp.id] = true
	}
	targets := map[string]bool{}
	for _, comp := range c.selectComponents(chk.To) {
		targets[comp.id] = true
	}

	var problems []string
	for _, src := range c.selectComponents(chk.From) {
		if through[src.id] {
			continue
		}
		// Breadth-first search that doesn't enter Through components, so
		// any target it reaches has a path around them.
		prev := map[string]string{src.id: ""}
		queue := []string{src.id}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			for _, next := range edges[id] {
				if _, seen := prev[next]; seen || through[next] {
					continue
				}
				prev[next] = id
				if targets[next] {
					problems = append(problems, fmt.Sprintf("%s reaches %s without %s: %s",
						c.where(src), next, strings.Join(chk.Through, " or "), pathTo(prev, next)))
				}
				queue = append(queue, next)
			}
		}
	}
	return problems
}

func pathTo(prev map[string]string, id string) string {
	var path []string
	for ; id != ""; id = prev[id] {
		path = append(path, id)
	}
	slices.Reverse(path)
	return strings.Join(path, " -> ")
}
