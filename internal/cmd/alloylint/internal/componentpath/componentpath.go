// Package componentpath exposes an Analyzer which ensures that each component
// is defined in the package which matches its name.
package componentpath

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/grafana/alloy/internal/cmd/alloylint/internal/findcomponents"
)

var Analyzer = &analysis.Analyzer{
	Name: "componentpath",
	Doc:  "ensure components are defined in the package which matches their name",
	Run:  run,
}

const componentPackage = "github.com/grafana/alloy/internal/component"

func run(p *analysis.Pass) (any, error) {
	// The component named "loki.source.file" must be registered by
	// internal/component/loki/source/file. Build tooling relies on this to
	// find the package of a component from its name.
	rel, ok := strings.CutPrefix(p.Pkg.Path(), componentPackage+"/")
	if !ok {
		return nil, nil
	}
	expected := strings.ReplaceAll(rel, "/", ".")

	for _, file := range p.Files {
		if strings.HasSuffix(p.Fset.File(file.Pos()).Name(), "_test.go") {
			continue
		}

		for _, call := range findcomponents.RegisterCalls(p.TypesInfo, file) {
			name, ok := registrationName(p.TypesInfo, call)
			if !ok {
				p.Reportf(call.Pos(), "component name must be a constant string")
				continue
			}
			if name != expected {
				p.Reportf(call.Pos(), "component %q must be defined in package %s/%s", name, componentPackage, strings.ReplaceAll(name, ".", "/"))
			}
		}
	}

	return nil, nil
}

// registrationName returns the value of the Name field of the
// component.Registration passed to a component.Register call, if it's a
// constant string.
func registrationName(info *types.Info, call *ast.CallExpr) (string, bool) {
	if len(call.Args) != 1 {
		return "", false
	}
	lit, ok := call.Args[0].(*ast.CompositeLit)
	if !ok {
		return "", false
	}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := kv.Key.(*ast.Ident); !ok || key.Name != "Name" {
			continue
		}

		value := info.Types[kv.Value].Value
		if value == nil || value.Kind() != constant.String {
			return "", false
		}
		return constant.StringVal(value), true
	}
	return "", false
}
