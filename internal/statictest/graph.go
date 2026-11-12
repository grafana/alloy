package statictest

import (
	"fmt"
	"maps"
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

// dataFlow returns, for each node, the nodes it sends data to. A node is a
// component ID, or a module's argument or export, such as
// cluster_metrics/argument.metrics_destinations. Data that a module sends to
// an argument goes on to what the blocks that use the module pass in.
func dataFlow(c *config) map[string][]string {
	// holder is something that holds references: a component, the value of
	// an argument passed to a module, or a module's export.
	type holder struct {
		node string
		refs []reference
	}
	var holders []holder
	find := func(node, scope string, n ast.Node) {
		v := &refFinder{config: c, scope: scope}
		ast.Walk(v, n)
		holders = append(holders, holder{node: node, refs: v.refs})
	}
	for _, comp := range c.components {
		if comp.instanceOf == "" {
			find(comp.id, comp.module, comp.block.Body)
			continue
		}
		for _, stmt := range comp.block.Body {
			if attr, ok := stmt.(*ast.AttributeStmt); ok {
				find(argumentNode(comp.instanceOf, attr.Name.Name), comp.module, attr.Value)
			}
		}
	}
	for _, path := range slices.Sorted(maps.Keys(c.modules)) {
		m := c.modules[path]
		for _, name := range slices.Sorted(maps.Keys(m.exports)) {
			find(exportNode(path, name), path, m.exports[name].Body)
		}
	}

	// An argument or export pushes data when what it holds does, such as
	// [prometheus.remote_write.default.receiver]. Arguments and exports can
	// hold each other, so repeat until nothing changes.
	pushNodes := map[string]bool{}
	pushes := func(r reference) bool {
		if r.pseudo {
			return pushNodes[r.id]
		}
		return pushExports[r.export]
	}
	for changed := true; changed; {
		changed = false
		for _, h := range holders {
			if pushNodes[h.node] || !slices.ContainsFunc(h.refs, pushes) {
				continue
			}
			if _, isComp := c.byID[h.node]; !isComp {
				pushNodes[h.node] = true
				changed = true
			}
		}
	}

	edges := map[string][]string{}
	add := func(from, to string) {
		if from != to && !slices.Contains(edges[from], to) {
			edges[from] = append(edges[from], to)
		}
	}
	for _, h := range holders {
		for _, ref := range h.refs {
			if pushes(ref) {
				add(h.node, ref.id)
			} else {
				add(ref.id, h.node)
			}
		}
	}
	return edges
}

func argumentNode(module, name string) string { return module + "/argument." + name }
func exportNode(module, name string) string   { return module + "/export." + name }

type reference struct {
	id     string // the component, argument or export referenced
	export string // the first part after a component's ID
	// pseudo is true for a module's argument or export.
	pseudo bool
}

// refFinder collects references to components, arguments and module exports
// in one scope.
type refFinder struct {
	config *config
	scope  string
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
	if parts[0] == "argument" && v.scope != "" && len(parts) >= 2 {
		v.refs = append(v.refs, reference{id: argumentNode(v.scope, parts[1]), pseudo: true})
		return nil
	}
	// The longest prefix that names a component is the reference.
	for n := len(parts) - 1; n >= 2; n-- {
		comp, ok := v.config.byID[joinModule(v.scope, strings.Join(parts[:n], "."))]
		if !ok {
			continue
		}
		if comp.instanceOf != "" {
			v.refs = append(v.refs, reference{id: exportNode(comp.instanceOf, parts[n]), pseudo: true})
		} else {
			v.refs = append(v.refs, reference{id: comp.id, export: parts[n]})
		}
		return nil
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
