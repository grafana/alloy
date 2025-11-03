// Package alloyyaml provides conversion between YAML and Alloy configuration syntax.
//
// This package converts YAML to Alloy text format, which can then be parsed by
// the standard Alloy parser. This enables using YAML as an alternative configuration
// format for Alloy while maintaining full compatibility with all Alloy features.
package alloyyaml

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/grafana/alloy/syntax/scanner"
	"gopkg.in/yaml.v3"
)

// ToAlloy converts YAML to Alloy configuration syntax.
// The YAML should follow a natural structure where:
//   - Maps represent blocks
//   - Key-value pairs with simple values are attributes
//   - Arrays of maps represent multiple blocks with the same name
//   - The expr() wrapper can be used for complex Alloy expressions
//
// Example YAML:
//
//	logging:
//	  level: debug
//	  format: json
//
// Converts to Alloy:
//
//	logging {
//	  format = "json"
//	  level = "debug"
//	}
func ToAlloy(yamlData []byte) ([]byte, error) {
	var data interface{}
	if err := yaml.Unmarshal(yamlData, &data); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	var buf bytes.Buffer
	if err := writeValue(&buf, data, 0, true); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// writeValue writes any YAML value in the appropriate Alloy format.
// The isTopLevel flag indicates whether this is a top-level body (affects formatting).
func writeValue(w io.Writer, value interface{}, indent int, isTopLevel bool) error {
	switch v := value.(type) {
	case nil:
		if isTopLevel {
			// Top-level null doesn't make sense
			return nil
		}
		fmt.Fprint(w, "null")
		return nil

	case map[string]interface{}:
		return writeMap(w, v, indent, isTopLevel)

	case []interface{}:
		if isTopLevel {
			// Top-level array should be treated as multiple statements
			return writeTopLevelArray(w, v, indent)
		}
		return writeArray(w, v)

	case string:
		return writeString(w, v)

	case int:
		fmt.Fprintf(w, "%d", v)
	case int64:
		fmt.Fprintf(w, "%d", v)
	case float32:
		writeFloat(w, float64(v))
	case float64:
		writeFloat(w, v)

	case bool:
		fmt.Fprintf(w, "%v", v)

	default:
		return fmt.Errorf("unsupported value type: %T", v)
	}

	return nil
}

// writeMap writes a YAML map as either a block body (top-level) or object literal (nested).
func writeMap(w io.Writer, m map[string]interface{}, indent int, isTopLevel bool) error {
	if len(m) == 0 {
		if isTopLevel {
			return nil
		}
		fmt.Fprint(w, "{}")
		return nil
	}

	if isTopLevel {
		// Top-level map represents a body with blocks and attributes
		return writeBody(w, m, indent)
	}

	// Nested maps (when used as attribute values) are always object literals
	return writeObjectLiteral(w, m)
}

// writeBody writes a map as Alloy statements (blocks and attributes).
func writeBody(w io.Writer, body map[string]interface{}, indent int) error {
	indentStr := strings.Repeat("  ", indent)

	// Sort keys for deterministic output
	keys := make([]string, 0, len(body))
	for k := range body {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	firstItem := true
	for _, key := range keys {
		value := body[key]

		// Add blank line between top-level blocks (but not first item)
		if !firstItem && indent == 0 && isStructural(value) {
			fmt.Fprintln(w)
		}
		firstItem = false

		// Determine if this is a block or attribute based on value type
		switch v := value.(type) {
		case map[string]interface{}:
			// Check for $object marker - convert to object literal attribute
			if objValue, hasObject := v["$object"]; hasObject {
				if err := writeAttribute(w, key, objValue, indentStr); err != nil {
					return fmt.Errorf("attribute %s: %w", key, err)
				}
				continue
			}

			// Check for $label marker - convert to labeled block
			if labelValue, hasLabel := v["$label"]; hasLabel {
				if labelStr, ok := labelValue.(string); ok {
					// Write block with label
					fmt.Fprintf(w, "%s%s %q {\n", indentStr, key, labelStr)
					// Write remaining keys as body (excluding $label)
					bodyMap := make(map[string]interface{})
					for k, val := range v {
						if k != "$label" {
							bodyMap[k] = val
						}
					}
					if err := writeBody(w, bodyMap, indent+1); err != nil {
						return err
					}
					fmt.Fprintf(w, "%s}\n", indentStr)
					continue
				}
			}

			// Regular map values are blocks
			if err := writeBlock(w, key, value, indent); err != nil {
				return fmt.Errorf("block %s: %w", key, err)
			}

		case []interface{}:
			// Arrays of maps are multiple blocks
			if len(v) > 0 {
				if _, ok := v[0].(map[string]interface{}); ok {
					if err := writeBlock(w, key, value, indent); err != nil {
						return fmt.Errorf("block %s: %w", key, err)
					}
					continue
				}
			}
			// Otherwise it's a simple array attribute
			if err := writeAttribute(w, key, value, indentStr); err != nil {
				return fmt.Errorf("attribute %s: %w", key, err)
			}

		default:
			// Simple values are attributes
			if err := writeAttribute(w, key, value, indentStr); err != nil {
				return fmt.Errorf("attribute %s: %w", key, err)
			}
		}
	}

	return nil
}

// isStructural returns true if the value is a structural element (nested map).
// Arrays are not considered structural because they can be inline attribute values.
func isStructural(value interface{}) bool {
	switch value.(type) {
	case map[string]interface{}:
		return true
	}
	return false
}

// writeBlock writes a block in Alloy syntax.
func writeBlock(w io.Writer, name string, value interface{}, indent int) error {
	indentStr := strings.Repeat("  ", indent)

	switch v := value.(type) {
	case map[string]interface{}:
		// Single block
		fmt.Fprintf(w, "%s%s {\n", indentStr, name)
		if err := writeBody(w, v, indent+1); err != nil {
			return err
		}
		fmt.Fprintf(w, "%s}\n", indentStr)

	case []interface{}:
		// Multiple blocks with same name
		for i, elem := range v {
			if i > 0 && indent == 0 {
				fmt.Fprintln(w) // Blank line between top-level blocks
			}

			if elemMap, ok := elem.(map[string]interface{}); ok {
				fmt.Fprintf(w, "%s%s {\n", indentStr, name)
				if err := writeBody(w, elemMap, indent+1); err != nil {
					return err
				}
				fmt.Fprintf(w, "%s}\n", indentStr)
			} else {
				return fmt.Errorf("array elements for block %s must be maps, got %T", name, elem)
			}
		}

	default:
		return fmt.Errorf("invalid block value type: %T", value)
	}

	return nil
}

// writeAttribute writes an attribute assignment.
func writeAttribute(w io.Writer, name string, value interface{}, indentStr string) error {
	fmt.Fprintf(w, "%s%s = ", indentStr, name)

	if err := writeValue(w, value, 0, false); err != nil {
		return err
	}

	fmt.Fprintln(w)
	return nil
}

// writeString writes a string value, handling expr() wrapper.
func writeString(w io.Writer, s string) error {
	// Check for expr() wrapper
	if strings.HasPrefix(s, "expr(") && strings.HasSuffix(s, ")") {
		// Unwrap and write expression as-is (unquoted)
		expr := s[5 : len(s)-1]
		fmt.Fprint(w, expr)
		return nil
	}

	// Regular string - quote it
	// TODO: Handle escaping properly (newlines, quotes, etc.)
	fmt.Fprintf(w, "%q", s)
	return nil
}

// writeFloat writes a float, avoiding unnecessary decimal points for whole numbers.
func writeFloat(w io.Writer, f float64) {
	if f == float64(int64(f)) {
		// Whole number
		fmt.Fprintf(w, "%d", int64(f))
	} else {
		fmt.Fprintf(w, "%v", f)
	}
}

// writeArray writes an array in Alloy syntax.
func writeArray(w io.Writer, arr []interface{}) error {
	if len(arr) == 0 {
		fmt.Fprint(w, "[]")
		return nil
	}

	fmt.Fprint(w, "[")
	for i, elem := range arr {
		if i > 0 {
			fmt.Fprint(w, ", ")
		}
		if err := writeValue(w, elem, 0, false); err != nil {
			return err
		}
	}
	fmt.Fprint(w, "]")
	return nil
}

// writeTopLevelArray handles an array at the top level (rare, but possible).
func writeTopLevelArray(w io.Writer, arr []interface{}, indent int) error {
	for _, elem := range arr {
		if err := writeValue(w, elem, indent, true); err != nil {
			return err
		}
	}
	return nil
}

// writeObjectLiteral writes an object literal in Alloy syntax.
func writeObjectLiteral(w io.Writer, obj map[string]interface{}) error {
	if len(obj) == 0 {
		fmt.Fprint(w, "{}")
		return nil
	}

	fmt.Fprint(w, "{ ")

	// Sort keys for deterministic output
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, key := range keys {
		if i > 0 {
			fmt.Fprint(w, ", ")
		}

		// Write key (quote if needed)
		if needsQuoting(key) {
			fmt.Fprintf(w, "%q = ", key)
		} else {
			fmt.Fprintf(w, "%s = ", key)
		}

		// Write value
		if err := writeValue(w, obj[key], 0, false); err != nil {
			return err
		}
	}

	fmt.Fprint(w, " }")
	return nil
}

// needsQuoting returns true if a string needs to be quoted as an identifier.
func needsQuoting(s string) bool {
	return !scanner.IsValidIdentifier(s)
}
