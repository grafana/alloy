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
// The conversion preserves the structure:
//   - Blocks become nested maps
//   - Labeled blocks use "/" separator (e.g., "block_name/label")
//   - Object literals are marked with $object
//   - Complex expressions are wrapped in expr(...)
//   - Multiple blocks with the same name become YAML arrays
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
//	logging:
//	  format: json
//	  level: debug
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

// convertBody converts an AST body to a YAML-compatible map.
// It handles merging multiple blocks with the same name into arrays.
//
// Note: Single blocks are represented as objects, while multiple blocks with the same
// name are represented as arrays. Without schema information, we cannot determine if
// a single block COULD have multiple instances, so the YAML representation may differ
// from hand-written YAML that uses arrays for potentially-repeatable blocks.
func convertBody(body ast.Body) (interface{}, error) {
	if len(body) == 0 {
		return map[string]interface{}{}, nil
	}

	result := make(map[string]interface{})
	blockCounts := make(map[string]int) // Track how many times each block name appears

	// First pass: count block occurrences
	for _, stmt := range body {
		if block, ok := stmt.(*ast.BlockStmt); ok {
			name := strings.Join(block.Name, ".")
			if block.Label != "" {
				name = name + "/" + block.Label
			}
			blockCounts[name]++
		}
	}

	// Second pass: convert statements
	blockArrays := make(map[string][]interface{}) // Collect multiple blocks with same name

	for _, stmt := range body {
		switch s := stmt.(type) {
		case *ast.AttributeStmt:
			// Convert attribute
			name := s.Name.Name
			value, err := convertExpr(s.Value)
			if err != nil {
				return nil, fmt.Errorf("attribute %s: %w", name, err)
			}
			result[name] = value

		case *ast.BlockStmt:
			// Convert block
			name := strings.Join(s.Name, ".")
			key := name
			if s.Label != "" {
				key = name + "/" + s.Label
			}

			blockData, err := convertBody(s.Body)
			if err != nil {
				return nil, fmt.Errorf("block %s: %w", key, err)
			}

			// If this block name appears multiple times, collect in array
			if blockCounts[key] > 1 {
				blockArrays[key] = append(blockArrays[key], blockData)
			} else {
				result[key] = blockData
			}
		}
	}

	// Add arrays for multiple blocks
	for key, blocks := range blockArrays {
		result[key] = blocks
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
