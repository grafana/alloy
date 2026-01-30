// Package findcomponents exposes an Analyzer which ensures that created
// components are imported by a registry package.
package findcomponents

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

var Analyzer = &analysis.Analyzer{
	Name: "findcomponents",
	Doc:  "ensure components are imported",
	Run:  run,
}

var (
	componentPattern = "./internal/component/..."
	checkPackage     = "github.com/grafana/alloy/internal/component/all"
)

func init() {
	Analyzer.Flags.StringVar(&componentPattern, "components", componentPattern, "Pattern where components are defined")
	Analyzer.Flags.StringVar(&checkPackage, "import-package", checkPackage, "Package that should import components")
}

func run(p *analysis.Pass) (any, error) {
	// Our linter works as follows:
	//
	// 1. Retrieve the list of direct imports of the package we are performing
	//    analysis on.
	// 2. Find component packages across the module as defined by the -components
	//    flag.
	// 3. Report a diagnostic for any component package which is not being
	//    imported.
	//
	// This linter should only be run against a single package to check for
	// imports. The import-package flag is checked and all other packages are
	// ignored.

	if p.Pkg.Path() != checkPackage {
		return nil, nil
	}

	imports := make(map[string]struct{})
	for _, dep := range p.Pkg.Imports() {
		imports[dep.Path()] = struct{}{}
	}

	componentPackages, err := findComponentPackages(componentPattern)
	if err != nil {
		return nil, err
	}
	for componentPackage := range componentPackages {
		if _, imported := imports[componentPackage]; !imported {
			p.Report(analysis.Diagnostic{
				Pos:     p.Files[0].Pos(),
				Message: fmt.Sprintf("package does not import component %s", componentPackage),
			})
		}
	}

	return nil, nil
}

// findComponentPackages returns a map of discovered packages which declare
// components. The pattern argument controls the full list of patterns which
// are searched (e.g., "./..." or "./component/...").
func findComponentPackages(pattern string) (map[string]struct{}, error) {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
	}, "pattern="+pattern)
	if err != nil {
		return nil, err
	}

	componentPackages := map[string]struct{}{}

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			if declaresComponent(pkg, file) {
				componentPackages[pkg.ID] = struct{}{}
			}
		}
	}

	return componentPackages, nil
}

// declaresComponent inspects a file to see if it has something matching the
// following:
//
//	func init() {
//		component.Register(component.Registration{ ... })
//	}
func declaresComponent(pkg *packages.Package, file *ast.File) bool {
	// Look for an init function in the file.
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if funcDecl.Name.Name != "init" || funcDecl.Recv != nil {
			continue
		}

		var foundComponentDecl bool

		// Given an init function, check to see if there's a function call to
		// component.Register.
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}

			// Check to see if the ident refers to
			// github.com/grafana/alloy/internal/component.
			if pkgName, ok := pkg.TypesInfo.Uses[ident].(*types.PkgName); ok {
				if pkgName.Imported().Path() == "github.com/grafana/alloy/internal/component" &&
					sel.Sel.Name == "Register" {

					foundComponentDecl = true
					return false
				}
			}

			return true
		})

		if foundComponentDecl {
			return true
		}
	}

	return false
}
