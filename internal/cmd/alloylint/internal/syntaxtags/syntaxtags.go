// Package syntaxtags exposes an Analyzer which lints Alloy syntax tags.
package syntaxtags

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "syntaxtags",
	Doc:  "perform validation checks on Alloy syntax tags",
	Run:  run,
}

var noLintRegex = regexp.MustCompile(`//\s*nolint:(\S+)`)

var (
	syntaxTagRegex = regexp.MustCompile(`alloy:"([^"]*)"`)
	jsonTagRegex   = regexp.MustCompile(`json:"([^"]*)"`)
	yamlTagRegex   = regexp.MustCompile(`yaml:"([^"]*)"`)
)

// Rules for alloy tag linting:
//
// - No alloy tags on anonymous fields.
// - No alloy tags on unexported fields.
// - No empty tags (alloy:"").
// - Tags must have options (alloy:"NAME,OPTIONS").
// - Options must be one of the following:
//   - attr
//   - attr,optional
//   - block
//   - block,optional
//   - enum
//   - enum,optional
//   - label
//   - squash
// - Attribute and block tags must have a non-empty value NAME.
// - Fields marked as blocks must be the appropriate type.
// - Label tags must have an empty value for NAME.
// - Non-empty values for NAME must be snake_case.
// - Non-empty NAME values must be valid Alloy identifiers.
// - Attributes may not have a NAME with a `.` in it.

func run(p *analysis.Pass) (any, error) {
	structs := getStructs(p.TypesInfo)
	for _, sInfo := range structs {
		sNode := sInfo.Node
		s := sInfo.Type

		var hasSyntaxTags bool

		for i := 0; i < s.NumFields(); i++ {
			matches := syntaxTagRegex.FindAllStringSubmatch(s.Tag(i), -1)
			if len(matches) > 0 {
				hasSyntaxTags = true
				break
			}
		}

	NextField:
		for i := 0; i < s.NumFields(); i++ {
			field := s.Field(i)
			nodeField := lookupField(sNode, i)

			// Ignore fields with //nolint:syntaxtags in them.
			if comments := nodeField.Comment; comments != nil {
				for _, comment := range comments.List {
					if lintingDisabled(comment.Text) {
						continue NextField
					}
				}
			}

			matches := syntaxTagRegex.FindAllStringSubmatch(s.Tag(i), -1)
			if len(matches) == 0 && hasSyntaxTags {
				// If this struct has alloy tags, but this field only has json/yaml
				// tags, emit an error.
				jsonMatches := jsonTagRegex.FindAllStringSubmatch(s.Tag(i), -1)
				yamlMatches := yamlTagRegex.FindAllStringSubmatch(s.Tag(i), -1)

				if len(jsonMatches) > 0 || len(yamlMatches) > 0 {
					p.Report(analysis.Diagnostic{
						Pos:      field.Pos(),
						Category: "syntaxtags",
						Message:  "field has yaml or json tags, but no alloy tags",
					})
				}

				continue
			} else if len(matches) == 0 {
				continue
			} else if len(matches) > 1 {
				p.Report(analysis.Diagnostic{
					Pos:      field.Pos(),
					Category: "syntaxtags",
					Message:  "field should not have more than one alloy tag",
				})
			}

			// Before checking the tag, do general validations first.
			if field.Anonymous() {
				p.Report(analysis.Diagnostic{
					Pos:      field.Pos(),
					Category: "syntaxtags",
					Message:  "alloy tags may not be given to anonymous fields",
				})
			}
			if !field.Exported() {
				p.Report(analysis.Diagnostic{
					Pos:      field.Pos(),
					Category: "syntaxtags",
					Message:  "alloy tags may only be given to exported fields",
				})
			}
			if len(nodeField.Names) > 1 {
				// Report "a, b, c int `alloy:"name,attr"`" as invalid usage.
				p.Report(analysis.Diagnostic{
					Pos:      field.Pos(),
					Category: "syntaxtags",
					Message:  "alloy tags should not be inserted on field names separated by commas",
				})
			}

			for _, match := range matches {
				diagnostics := lintSyntaxTag(field, match[1])
				for _, diag := range diagnostics {
					p.Report(analysis.Diagnostic{
						Pos:      field.Pos(),
						Category: "syntaxtags",
						Message:  diag,
					})
				}
			}
		}
	}

	return nil, nil
}

func lintingDisabled(comment string) bool {
	// Extract //nolint:A,B,C into A,B,C
	matches := noLintRegex.FindAllStringSubmatch(comment, -1)
	for _, match := range matches {
		// Iterate over A,B,C by comma and see if our linter is included.
		for _, disabledLinter := range strings.Split(match[1], ",") {
			if disabledLinter == "syntaxtags" {
				return true
			}
		}
	}

	return false
}

func getStructs(ti *types.Info) []*structInfo {
	var res []*structInfo

	for ty, def := range ti.Defs {
		def, ok := def.(*types.TypeName)
		if !ok {
			continue
		}

		structTy, ok := def.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}

		switch node := ty.Obj.Decl.(*ast.TypeSpec).Type.(type) {
		case *ast.StructType:
			res = append(res, &structInfo{
				Node: node,
				Type: structTy,
			})
		default:
		}
	}

	return res
}

// lookupField gets a field given an index. If a field has multiple names, each
// name is counted as one index. For example,
//
//	Field1, Field2, Field3 int
//
// is one *ast.Field, but covers index 0 through 2.
func lookupField(node *ast.StructType, index int) *ast.Field {
	startIndex := 0

	for _, f := range node.Fields.List {
		length := len(f.Names)
		if length == 0 { // Embedded field
			length = 1
		}

		endIndex := startIndex + length
		if index >= startIndex && index < endIndex {
			return f
		}

		startIndex += length
	}

	panic(fmt.Sprintf("index %d out of range %d", index, node.Fields.NumFields()))
}

type structInfo struct {
	Node *ast.StructType
	Type *types.Struct
}

func lintSyntaxTag(ty *types.Var, tag string) (diagnostics []string) {
	if tag == "" {
		diagnostics = append(diagnostics, "alloy tag should not be empty")
		return
	}

	parts := strings.SplitN(tag, ",", 2)
	if len(parts) != 2 {
		diagnostics = append(diagnostics, "alloy tag is missing options")
		return
	}

	var (
		name    = parts[0]
		options = parts[1]

		nameParts = splitName(name)
	)

	switch options {
	case "attr", "attr,optional":
		if len(nameParts) == 0 {
			diagnostics = append(diagnostics, "attr field must have a name")
		} else if len(nameParts) > 1 {
			diagnostics = append(diagnostics, "attr field names must not contain `.`")
		}
		for _, name := range nameParts {
			diagnostics = append(diagnostics, validateFieldName(name)...)
		}

	case "block", "block,optional":
		if len(nameParts) == 0 {
			diagnostics = append(diagnostics, "block field must have a name")
		}
		for _, name := range nameParts {
			diagnostics = append(diagnostics, validateFieldName(name)...)
		}

		innerTy := getInnermostType(ty.Type())
		if !isStructType(innerTy) && !isStringMap(innerTy) && !isEmptyInterface(innerTy) {
			diagnostics = append(diagnostics, "block fields must be an interface{}, map[string]T, a struct, or a slice of structs")
		}

	case "enum", "enum,optional":
		if len(nameParts) == 0 {
			diagnostics = append(diagnostics, "block field must have a name")
		}
		for _, name := range nameParts {
			diagnostics = append(diagnostics, validateFieldName(name)...)
		}

		_, isArray := ty.Type().(*types.Array)
		_, isSlice := ty.Type().(*types.Slice)

		if !isArray && !isSlice {
			diagnostics = append(diagnostics, "enum fields must be a slice or array of structs")
		} else {
			innerTy := getInnermostType(ty.Type())
			if _, ok := innerTy.(*types.Struct); !ok {
				diagnostics = append(diagnostics, "enum fields must be a slice or array of structs")
			}
		}

	case "label":
		if name != "" {
			diagnostics = append(diagnostics, "label field must have an empty value for name")
		}

	case "squash":
		if name != "" {
			diagnostics = append(diagnostics, "squash field must have an empty value for name")
		}

	default:
		diagnostics = append(diagnostics, fmt.Sprintf("unrecognized options %s", options))
	}

	return
}

func getInnermostType(ty types.Type) types.Type {
	ty = ty.Underlying()

	switch ty := ty.(type) {
	case *types.Pointer:
		return getInnermostType(ty.Elem())
	case *types.Array:
		return getInnermostType(ty.Elem())
	case *types.Slice:
		return getInnermostType(ty.Elem())
	}

	return ty
}

func splitName(in string) []string {
	return strings.Split(in, ".")
}

var fieldNameRegex = regexp.MustCompile("^[a-z][a-z0-9_]*$")

func validateFieldName(name string) (diagnostics []string) {
	if !fieldNameRegex.MatchString(name) {
		msg := fmt.Sprintf("%q must be a valid syntax snake_case identifier", name)
		diagnostics = append(diagnostics, msg)
	}

	return
}

func isStructType(ty types.Type) bool {
	_, ok := ty.(*types.Struct)
	return ok
}

func isStringMap(ty types.Type) bool {
	mapType, ok := ty.(*types.Map)
	if !ok {
		return false
	}
	if basic, ok := mapType.Key().(*types.Basic); ok {
		return basic.Kind() == types.String
	}
	return false
}

func isEmptyInterface(ty types.Type) bool {
	ifaceType, ok := ty.(*types.Interface)
	if !ok {
		return false
	}
	return ifaceType.Empty()
}
