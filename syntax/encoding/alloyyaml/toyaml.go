package alloyyaml

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/printer"
	"github.com/grafana/alloy/syntax/token"
	"gopkg.in/yaml.v3"
)

// ToYAML converts Alloy configuration syntax to YAML.
// The conversion uses an array-based structure to preserve order and allow duplicates:
//   - Root level is a YAML array
//   - Block bodies are YAML arrays of single-key maps
//   - Each statement (attribute or block) is a single-key map
//   - Labeled blocks use "/" separator (e.g., "block_name/label")
//   - Array literals are marked with $array
//   - Object literals are marked with $object
//   - Complex expressions are wrapped in expr(...)
//
// Example Alloy:
//
//	logging {
//	  level = "debug"
//	  format = "json"
//	}
//
// Converts to YAML:
//
//	- logging:
//	    - level: debug
//	    - format: json
func ToYAML(alloyData []byte) ([]byte, error) {
	// Parse the Alloy file
	file, err := parser.ParseFile("config.alloy", alloyData)
	if err != nil {
		return nil, fmt.Errorf("invalid Alloy syntax: %w", err)
	}

	// Convert the AST body to YAML-compatible data structure
	data, err := convertBody(file.Body)
	if err != nil {
		return nil, err
	}

	// Marshal to YAML
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal YAML: %w", err)
	}

	return yamlData, nil
}

// convertBody converts an AST body to a YAML-compatible array of single-key maps.
// Each statement (attribute or block) becomes a single-key map in the array.
// This preserves order and allows duplicate block names.
func convertBody(body ast.Body) (interface{}, error) {
	if len(body) == 0 {
		return []interface{}{}, nil
	}

	result := make([]interface{}, 0, len(body))

	for _, stmt := range body {
		switch s := stmt.(type) {
		case *ast.AttributeStmt:
			// Convert attribute to single-key map
			name := s.Name.Name
			value, err := convertExpr(s.Value)
			if err != nil {
				return nil, fmt.Errorf("attribute %s: %w", name, err)
			}
			result = append(result, map[string]interface{}{
				name: value,
			})

		case *ast.BlockStmt:
			// Convert block to single-key map
			name := strings.Join(s.Name, ".")
			key := name
			if s.Label != "" {
				key = name + "/" + s.Label
			}

			blockData, err := convertBody(s.Body)
			if err != nil {
				return nil, fmt.Errorf("block %s: %w", key, err)
			}

			result = append(result, map[string]interface{}{
				key: blockData,
			})
		}
	}

	return result, nil
}

// convertExpr converts an AST expression to a YAML-compatible value.
func convertExpr(expr ast.Expr) (interface{}, error) {
	switch e := expr.(type) {
	case *ast.LiteralExpr:
		return convertLiteral(e)

	case *ast.ArrayExpr:
		return convertArray(e)

	case *ast.ObjectExpr:
		return convertObject(e)

	case *ast.IdentifierExpr, *ast.AccessExpr, *ast.IndexExpr, *ast.CallExpr,
		*ast.UnaryExpr, *ast.BinaryExpr, *ast.ParenExpr:
		// Complex expressions: render as Alloy syntax and wrap in expr()
		return renderExprAsString(expr)

	default:
		return nil, fmt.Errorf("unsupported expression type: %T", expr)
	}
}

// convertLiteral converts a literal expression to its Go value.
func convertLiteral(lit *ast.LiteralExpr) (interface{}, error) {
	switch lit.Kind {
	case token.NULL:
		return nil, nil

	case token.BOOL:
		return lit.Value == "true", nil

	case token.NUMBER:
		// Try parsing as int first, then as uint, then as float
		if intVal, err := strconv.ParseInt(lit.Value, 0, 64); err == nil {
			return intVal, nil
		}
		if uintVal, err := strconv.ParseUint(lit.Value, 0, 64); err == nil {
			// Check if it fits in int64
			if uintVal <= 9223372036854775807 {
				return int64(uintVal), nil
			}
			return uintVal, nil
		}
		if floatVal, err := strconv.ParseFloat(lit.Value, 64); err == nil {
			return floatVal, nil
		}
		return nil, fmt.Errorf("invalid number literal: %s", lit.Value)

	case token.FLOAT:
		floatVal, err := strconv.ParseFloat(lit.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float literal: %s", lit.Value)
		}
		return floatVal, nil

	case token.STRING:
		strVal, err := strconv.Unquote(lit.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid string literal: %s", lit.Value)
		}
		return strVal, nil

	default:
		return nil, fmt.Errorf("unsupported literal kind: %v", lit.Kind)
	}
}

// convertArray converts an array expression to a slice.
func convertArray(arr *ast.ArrayExpr) (interface{}, error) {
	// Check if all elements are object literals
	allObjects := true
	for _, elem := range arr.Elements {
		if _, ok := elem.(*ast.ObjectExpr); !ok {
			allObjects = false
			break
		}
	}

	result := make([]interface{}, len(arr.Elements))
	for i, elem := range arr.Elements {
		// For object literals in arrays, convert directly without $object marker
		if objExpr, ok := elem.(*ast.ObjectExpr); ok && allObjects {
			objData := make(map[string]interface{})
			for _, field := range objExpr.Fields {
				val, err := convertExpr(field.Value)
				if err != nil {
					return nil, fmt.Errorf("array element %d, field %s: %w", i, field.Name.Name, err)
				}
				objData[field.Name.Name] = val
			}
			result[i] = objData
		} else {
			val, err := convertExpr(elem)
			if err != nil {
				return nil, fmt.Errorf("array element %d: %w", i, err)
			}
			result[i] = val
		}
	}

	// If all elements were objects, wrap in $array marker to distinguish from multiple blocks
	if allObjects {
		return map[string]interface{}{
			"$array": result,
		}, nil
	}

	return result, nil
}

// convertObject converts an object expression to a map with $object marker.
func convertObject(obj *ast.ObjectExpr) (interface{}, error) {
	objData := make(map[string]interface{})
	for _, field := range obj.Fields {
		val, err := convertExpr(field.Value)
		if err != nil {
			return nil, fmt.Errorf("object field %s: %w", field.Name.Name, err)
		}
		objData[field.Name.Name] = val
	}

	// Wrap in $object marker to distinguish from blocks
	return map[string]interface{}{
		"$object": objData,
	}, nil
}

// renderExprAsString renders an expression as Alloy syntax and wraps it in expr().
func renderExprAsString(expr ast.Expr) (string, error) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, expr); err != nil {
		return "", fmt.Errorf("failed to render expression: %w", err)
	}

	// Clean up the output: remove excessive whitespace and newlines
	output := buf.String()
	output = strings.TrimSpace(output)

	// Wrap in expr()
	return "expr(" + output + ")", nil
}
