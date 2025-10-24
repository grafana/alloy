package internal

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type API struct {
	Exports   *Struct
	Arguments *Struct
}

type Struct struct {
	Fields []StructField
}

type StructField struct {
	Name string
	Type string
	Tag  []string
}

func Parse(path string) (*API, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read files at %s: %w", path, err)
	}

	var files []*ast.File
	fset := token.NewFileSet()
	for _, e := range entries {
		// We skip directories, non go files and test files.
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, filepath.Join(path, e.Name()), nil, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to parse file: %w", err)
		}

		files = append(files, file)
	}

	var (
		exports   *ast.TypeSpec
		arguments *ast.TypeSpec
	)
	// Find Exports and Arguments
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			typ, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if typ.Name.String() == "Arguments" {
				arguments = typ
			}

			if typ.Name.String() == "Exports" {
				exports = typ
			}

			return arguments == nil || exports == nil
		})
	}

	return &API{
		Exports:   buildStruct(exports, fset),
		Arguments: buildStruct(arguments, fset),
	}, nil

}

func buildStruct(spec *ast.TypeSpec, fset *token.FileSet) *Struct {
	if spec == nil {
		return nil
	}

	structSpec := spec.Type.(*ast.StructType)
	s := Struct{Fields: make([]StructField, 0, len(structSpec.Fields.List))}
	for _, f := range structSpec.Fields.List {
		s.Fields = append(s.Fields, StructField{
			Name: f.Names[0].String(),
			Type: exprToString(f.Type, fset),
			Tag:  parseTag(f.Tag.Value),
		})
	}
	return &s
}

func parseTag(str string) []string {
	if str == "" {
		return nil
	}
	str = strings.TrimPrefix(str, "`alloy:\"")
	str = strings.TrimSuffix(str, "\"`")
	parts := strings.Split(str, ",")

	var out []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func exprToString(expr ast.Expr, fset *token.FileSet) string {
	var sb strings.Builder
	_ = printer.Fprint(&sb, fset, expr)
	return sb.String()
}
