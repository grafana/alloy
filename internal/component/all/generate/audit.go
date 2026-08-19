package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	alloyModulePrefix     = "github.com/grafana/alloy/"
	componentPackagePath  = "github.com/grafana/alloy/internal/component"
	registrationFieldName = "Name"
)

type registrationScan struct {
	Names      []string
	Unresolved []string
}

func auditCatalogSources(cat catalog, repoRoot string) error {
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
		return fmt.Errorf("locate repository root %q: %w", repoRoot, err)
	}

	var problems []string
	for _, pkg := range cat.Packages {
		relative := strings.TrimPrefix(pkg.ImportPath, alloyModulePrefix)
		if relative == pkg.ImportPath {
			problems = append(problems, fmt.Sprintf("%s: cannot resolve package within Alloy module", pkg.ImportPath))
			continue
		}
		directory := filepath.Join(repoRoot, filepath.FromSlash(relative))
		scan, err := scanRegistrationNames(directory, repoRoot)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", pkg.ImportPath, err))
			continue
		}
		if len(scan.Unresolved) > 0 {
			problems = append(problems, fmt.Sprintf("%s: unresolved component registrations at %s", pkg.ImportPath, strings.Join(scan.Unresolved, ", ")))
		}
		if !equalStrings(scan.Names, pkg.Components) {
			problems = append(problems, fmt.Sprintf("%s: catalog names [%s], source names [%s]", pkg.ImportPath, strings.Join(pkg.Components, ", "), strings.Join(scan.Names, ", ")))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

func scanRegistrationNames(directory, repoRoot string) (registrationScan, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return registrationScan{}, fmt.Errorf("read package directory: %w", err)
	}

	type parsedFile struct {
		file             *ast.File
		componentAliases map[string]struct{}
	}

	fset := token.NewFileSet()
	var files []parsedFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		filename := filepath.Join(directory, entry.Name())
		file, err := parser.ParseFile(fset, filename, nil, parser.SkipObjectResolution)
		if err != nil {
			return registrationScan{}, fmt.Errorf("parse %s: %w", filename, err)
		}
		aliases, err := componentImportAliases(file)
		if err != nil {
			return registrationScan{}, fmt.Errorf("inspect imports in %s: %w", filename, err)
		}
		files = append(files, parsedFile{
			file:             file,
			componentAliases: aliases,
		})
	}
	if len(files) == 0 {
		return registrationScan{}, errors.New("package has no non-test Go source files")
	}

	astFiles := make([]*ast.File, 0, len(files))
	for _, parsed := range files {
		astFiles = append(astFiles, parsed.file)
	}
	values := collectValues(astFiles)
	stringsByIdentifier := resolvePackageStrings(values)
	names := make(map[string]struct{})
	var unresolved []string
	for _, parsed := range files {
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 || !isComponentRegister(call.Fun, parsed.componentAliases) {
				return true
			}
			name, ok := registrationName(call.Args[0], values, stringsByIdentifier)
			if !ok {
				position := fset.Position(call.Pos())
				filename := position.Filename
				if repoRoot != "" {
					if relative, err := filepath.Rel(repoRoot, filename); err == nil {
						filename = filepath.ToSlash(relative)
					}
				}
				unresolved = append(unresolved, fmt.Sprintf("%s:%d", filename, position.Line))
				return true
			}
			names[name] = struct{}{}
			return true
		})
	}

	return registrationScan{
		Names:      sortedSet(names),
		Unresolved: uniqueSortedStrings(unresolved),
	}, nil
}

func componentImportAliases(file *ast.File) (map[string]struct{}, error) {
	aliases := make(map[string]struct{})
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		if importPath != componentPackagePath {
			continue
		}
		alias := "component"
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		if alias != "_" && alias != "." {
			aliases[alias] = struct{}{}
		}
	}
	return aliases, nil
}

func collectValues(files []*ast.File) map[string]ast.Expr {
	values := make(map[string]ast.Expr)
	for _, file := range files {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || (general.Tok != token.CONST && general.Tok != token.VAR) {
				continue
			}
			var previous []ast.Expr
			for _, spec := range general.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				expressions := valueSpec.Values
				if general.Tok == token.CONST && len(expressions) == 0 {
					expressions = previous
				}
				if len(valueSpec.Values) > 0 {
					previous = valueSpec.Values
				}
				for index, identifier := range valueSpec.Names {
					if index < len(expressions) {
						values[identifier.Name] = expressions[index]
					} else if len(expressions) == 1 {
						values[identifier.Name] = expressions[0]
					}
				}
			}
		}
	}
	return values
}

func resolvePackageStrings(values map[string]ast.Expr) map[string]string {
	resolved := make(map[string]string)
	for changed := true; changed; {
		changed = false
		for identifier, expression := range values {
			if _, exists := resolved[identifier]; exists {
				continue
			}
			if value, ok := evaluateString(expression, resolved); ok {
				resolved[identifier] = value
				changed = true
			}
		}
	}
	return resolved
}

func isComponentRegister(expression ast.Expr, aliases map[string]struct{}) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Register" {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	_, ok = aliases[identifier.Name]
	return ok
}

func registrationName(expression ast.Expr, values map[string]ast.Expr, resolvedStrings map[string]string) (string, bool) {
	visited := make(map[string]struct{})
	for {
		switch value := expression.(type) {
		case *ast.ParenExpr:
			expression = value.X
		case *ast.UnaryExpr:
			if value.Op != token.AND {
				return "", false
			}
			expression = value.X
		case *ast.Ident:
			if _, exists := visited[value.Name]; exists {
				return "", false
			}
			visited[value.Name] = struct{}{}
			next, exists := values[value.Name]
			if !exists {
				return "", false
			}
			expression = next
		case *ast.CompositeLit:
			for _, element := range value.Elts {
				keyValue, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := keyValue.Key.(*ast.Ident)
				if ok && key.Name == registrationFieldName {
					return evaluateString(keyValue.Value, resolvedStrings)
				}
			}
			return "", false
		default:
			return "", false
		}
	}
}

func evaluateString(expression ast.Expr, resolved map[string]string) (string, bool) {
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(value.Value)
		return text, err == nil
	case *ast.Ident:
		text, ok := resolved[value.Name]
		return text, ok
	case *ast.ParenExpr:
		return evaluateString(value.X, resolved)
	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return "", false
		}
		left, leftOK := evaluateString(value.X, resolved)
		right, rightOK := evaluateString(value.Y, resolved)
		return left + right, leftOK && rightOK
	default:
		return "", false
	}
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func uniqueSortedStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return sortedSet(set)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
