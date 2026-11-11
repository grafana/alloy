package statictest

import (
	"fmt"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/vm"
)

// component is a top-level block of the config, such as
// prometheus.scrape "default".
type component struct {
	name  string // prometheus.scrape
	id    string // prometheus.scrape.default
	block *ast.BlockStmt
	line  int
}

type config struct {
	file       string
	components []component
	byID       map[string]*component
}

func parseConfig(file string, src []byte) (*config, error) {
	f, err := parser.ParseFile(file, src)
	if err != nil {
		return nil, err
	}
	c := &config{file: file, byID: map[string]*component{}}
	for _, stmt := range f.Body {
		block, ok := stmt.(*ast.BlockStmt)
		if !ok {
			continue
		}
		comp := component{
			name:  block.GetBlockName(),
			id:    block.GetBlockName(),
			block: block,
			line:  block.NamePos.Position().Line,
		}
		if block.Label != "" {
			comp.id += "." + block.Label
		}
		c.components = append(c.components, comp)
	}
	for i := range c.components {
		c.byID[c.components[i].id] = &c.components[i]
	}
	return c, nil
}

// selectComponents returns the components that match any pattern, by name or
// by ID. In a pattern, * matches any characters, including dots.
func (c *config) selectComponents(sel Selector) []*component {
	var out []*component
	for i := range c.components {
		comp := &c.components[i]
		for _, p := range sel {
			if matchName(p, comp.name) || matchName(p, comp.id) {
				out = append(out, comp)
				break
			}
		}
	}
	return out
}

func matchName(pattern, name string) bool {
	ok, _ := path.Match(pattern, name)
	return ok
}

func (c *config) where(comp *component) string {
	return fmt.Sprintf("%s (%s:%d)", comp.id, c.file, comp.line)
}

// value is what a path points at: an attribute or a block.
type value struct {
	attr  *ast.AttributeStmt
	block *ast.BlockStmt
}

// resolve returns what a dot-separated path points at inside a block. A
// block segment matches every block with that name, so endpoint.url and
// endpoint[*].url are the same. [n] picks the nth block, from 0. A block name
// can have dots, such as stage.json, so the longest match wins.
func resolve(block *ast.BlockStmt, p string) ([]value, error) {
	if p == "" {
		return []value{{block: block}}, nil
	}
	return resolveParts(block, strings.Split(p, "."))
}

func resolveParts(b *ast.BlockStmt, parts []string) ([]value, error) {
	if len(parts) == 0 {
		return []value{{block: b}}, nil
	}
	if len(parts) == 1 {
		for _, stmt := range b.Body {
			if attr, ok := stmt.(*ast.AttributeStmt); ok && attr.Name.Name == parts[0] {
				return []value{{attr: attr}}, nil
			}
		}
	}
	for n := len(parts); n >= 1; n-- {
		name, index, err := splitIndex(strings.Join(parts[:n], "."))
		if err != nil {
			return nil, err
		}
		var blocks []*ast.BlockStmt
		for _, stmt := range b.Body {
			if child, ok := stmt.(*ast.BlockStmt); ok && child.GetBlockName() == name {
				blocks = append(blocks, child)
			}
		}
		if len(blocks) == 0 {
			continue
		}
		if index >= 0 {
			if index >= len(blocks) {
				return nil, nil
			}
			blocks = blocks[index : index+1]
		}
		var out []value
		for _, child := range blocks {
			vals, err := resolveParts(child, parts[n:])
			if err != nil {
				return nil, err
			}
			out = append(out, vals...)
		}
		return out, nil
	}
	return nil, nil
}

var indexSuffix = regexp.MustCompile(`^(.*)\[(\*|\d+)\]$`)

// splitIndex splits endpoint[2] into endpoint and 2. The index is -1 for no
// index or [*].
func splitIndex(s string) (string, int, error) {
	m := indexSuffix.FindStringSubmatch(s)
	if m == nil {
		return s, -1, nil
	}
	if m[2] == "*" {
		return m[1], -1, nil
	}
	i, err := strconv.Atoi(m[2])
	return m[1], i, err
}

// condition is the equals or matches part of a check.
type condition struct {
	equals  *any
	matches *regexp.Regexp
}

func newCondition(c Check) condition {
	cond := condition{equals: c.Equals}
	if c.Matches != "" {
		cond.matches = regexp.MustCompile("^(?:" + c.Matches + ")$")
	}
	return cond
}

func (c condition) String() string {
	if c.matches != nil {
		return "matches " + strings.TrimSuffix(strings.TrimPrefix(c.matches.String(), "^(?:"), ")$")
	}
	return fmt.Sprintf("equals %v", *c.equals)
}

// holds evaluates an attribute and checks it. It returns a reason when the
// check fails. An attribute that isn't a constant, such as one that reads
// another component's export, fails with that reason.
func (c condition) holds(attr *ast.AttributeStmt) (bool, string) {
	var v any
	if !isConstant(attr.Value) || vm.New(attr.Value).Evaluate(nil, &v) != nil {
		return false, "is not a constant, so it can't be checked"
	}
	if c.matches != nil {
		s, ok := v.(string)
		if !ok {
			return false, fmt.Sprintf("is %v, not a string", v)
		}
		if !c.matches.MatchString(s) {
			return false, fmt.Sprintf("is %q", s)
		}
		return true, ""
	}
	if !equal(v, *c.equals) {
		return false, fmt.Sprintf("is %v", format(v))
	}
	return true, ""
}

// isConstant reports whether an expression has no references and no function
// calls. A call such as sys.env would read the environment of the machine
// that runs the test, not the one that runs Alloy.
func isConstant(expr ast.Expr) bool {
	v := &constantVisitor{constant: true}
	ast.Walk(v, expr)
	return v.constant
}

type constantVisitor struct{ constant bool }

func (v *constantVisitor) Visit(node ast.Node) ast.Visitor {
	switch node.(type) {
	case *ast.IdentifierExpr, *ast.AccessExpr, *ast.CallExpr:
		v.constant = false
		return nil
	}
	return v
}

func equal(a, b any) bool {
	if fa, ok := toFloat(a); ok {
		fb, ok := toFloat(b)
		return ok && fa == fb
	}
	return reflect.DeepEqual(a, b)
}

func toFloat(v any) (float64, bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rv.Uint()), true
	case reflect.Float32, reflect.Float64:
		return rv.Float(), true
	}
	return 0, false
}

func format(v any) string {
	if s, ok := v.(string); ok {
		return strconv.Quote(s)
	}
	return fmt.Sprint(v)
}
