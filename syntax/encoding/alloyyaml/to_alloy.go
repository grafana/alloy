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
// See tests for example YAML and Alloy input files.
func ToAlloy(yamlData []byte) ([]byte, error) {
	var data interface{}
	if err := yaml.Unmarshal(yamlData, &data); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	var buf bytes.Buffer
	if err := writeValue(&buf, data, true); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// writeValue writes any YAML value in the appropriate Alloy format.
// The isTopLevel flag indicates whether this is a top-level body (affects formatting).
func writeValue(w io.Writer, value interface{}, isTopLevel bool) error {
	switch v := value.(type) {
	case nil:
		if isTopLevel {
			// Top-level null doesn't make sense
			return nil
		}
		_, err := fmt.Fprint(w, "null")
		return err

	case map[string]interface{}:
		return writeMap(w, v, 0, isTopLevel)

	case []interface{}:
		if isTopLevel {
			// Top-level array should be treated as multiple statements
			return writeTopLevelArray(w, v, 0)
		}
		return writeArray(w, v)

	case string:
		return writeString(w, v)

	case int:
		_, err := fmt.Fprintf(w, "%d", v)
		return err
	case int64:
		_, err := fmt.Fprintf(w, "%d", v)
		return err
	case float32:
		return writeFloat(w, float64(v))
	case float64:
		return writeFloat(w, v)

	case bool:
		_, err := fmt.Fprintf(w, "%v", v)
		return err

	default:
		return fmt.Errorf("unsupported value type: %T", v)
	}
}

// writeMap writes a YAML map as either a block body (top-level) or object literal (nested).
func writeMap(w io.Writer, m map[string]interface{}, indent int, isTopLevel bool) error {
	if len(m) == 0 {
		if isTopLevel {
			return nil
		}
		_, err := fmt.Fprint(w, "{}")
		return err
	}

	if isTopLevel {
		// Top-level map represents a body with blocks and attributes
		return writeBody(w, m, indent)
	}

	// Nested maps (when used as attribute values) are always object literals
	return writeObjectLiteral(w, m)
}

// writeBody is a compatibility wrapper that handles both old map format and new array format.
func writeBody(w io.Writer, body interface{}, indent int) error {
	switch b := body.(type) {
	case []interface{}:
		return writeBodyArray(w, b, indent)
	case map[string]interface{}:
		// Old format compatibility - should not happen in new design
		return writeBodyMap(w, b, indent)
	default:
		return fmt.Errorf("invalid body type: %T", body)
	}
}

// writeBodyArray writes an array of single-key maps as Alloy statements.
// Each element in the array is a map with exactly one key (the statement name).
func writeBodyArray(w io.Writer, body []interface{}, indent int) error {
	indentStr := strings.Repeat("  ", indent)

	for i, item := range body {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("body element %d must be a map, got %T", i, item)
		}

		if len(itemMap) != 1 {
			return fmt.Errorf("body element %d must have exactly one key, got %d keys", i, len(itemMap))
		}

		// Extract the single key-value pair
		var key string
		var value interface{}
		for k, v := range itemMap {
			key = k
			value = v
			break
		}

		// Add blank line between top-level blocks (but not before first item)
		if i > 0 && indent == 0 && isStructuralValue(value) {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}

		// Check if key contains "/" separator for block/label syntax
		blockName, label := splitBlockLabel(key)

		// Determine if this is a block or attribute based on value type
		if err := writeStatement(w, blockName, label, value, indentStr, indent); err != nil {
			return err
		}
	}

	return nil
}

// writeBodyMap writes the old map-based body format (for compatibility).
func writeBodyMap(w io.Writer, body map[string]interface{}, indent int) error {
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
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		firstItem = false

		// Check if key contains "/" separator for block/label syntax
		blockName, label := splitBlockLabel(key)

		if err := writeStatement(w, blockName, label, value, indentStr, indent); err != nil {
			return err
		}
	}

	return nil
}

// writeStatement writes a single statement (attribute or block).
func writeStatement(w io.Writer, blockName, label string, value interface{}, indentStr string, indent int) error {
	switch v := value.(type) {
	case map[string]interface{}:
		// Check for $array marker - convert to array literal attribute
		if arrValue, hasArray := v["$array"]; hasArray {
			if err := writeAttribute(w, blockName, arrValue, indentStr); err != nil {
				return fmt.Errorf("attribute %s: %w", blockName, err)
			}
			return nil
		}

		// In array-based format, plain maps are object literals (not blocks)
		// Blocks always have array bodies in the new format
		if err := writeAttribute(w, blockName, value, indentStr); err != nil {
			return fmt.Errorf("attribute %s: %w", blockName, err)
		}

	case []interface{}:
		// In new format, arrays are block bodies
		// Empty arrays or arrays of single-key maps are blocks
		if len(v) == 0 {
			// Empty block
			if label != "" {
				_, err := fmt.Fprintf(w, "%s%s %q { }\n", indentStr, blockName, label)
				return err
			}
			_, err := fmt.Fprintf(w, "%s%s { }\n", indentStr, blockName)
			return err
		}

		if firstElem, ok := v[0].(map[string]interface{}); ok && len(firstElem) == 1 {
			// This is a block body
			if label != "" {
				if _, err := fmt.Fprintf(w, "%s%s %q {\n", indentStr, blockName, label); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(w, "%s%s {\n", indentStr, blockName); err != nil {
					return err
				}
			}
			if err := writeBodyArray(w, v, indent+1); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "%s}\n", indentStr); err != nil {
				return err
			}
			return nil
		}

		// Otherwise it's an array attribute
		if err := writeAttribute(w, blockName, value, indentStr); err != nil {
			return fmt.Errorf("attribute %s: %w", blockName, err)
		}

	default:
		// Simple values are attributes
		if err := writeAttribute(w, blockName, value, indentStr); err != nil {
			return fmt.Errorf("attribute %s: %w", blockName, err)
		}
	}

	return nil
}

// splitBlockLabel splits a key into block name and label if it contains "/".
// Returns (blockName, label) where label is empty if no "/" is present.
func splitBlockLabel(key string) (string, string) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return key, ""
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

// isStructuralValue returns true if the value represents a block (not an attribute).
// In the new format, blocks are represented as arrays of single-key maps.
func isStructuralValue(value interface{}) bool {
	switch v := value.(type) {
	case []interface{}:
		// Check if it's an array of single-key maps (block body)
		if len(v) > 0 {
			if firstElem, ok := v[0].(map[string]interface{}); ok && len(firstElem) == 1 {
				return true
			}
		}
		return false
	case map[string]interface{}:
		// In array-based format, maps are object literals (not blocks)
		// Only $array marker indicates it's not structural
		if _, hasArray := v["$array"]; hasArray {
			return false
		}
		// Maps without markers are object literals, not structural
		return false
	}
	return false
}

// writeAttribute writes an attribute assignment.
func writeAttribute(w io.Writer, name string, value interface{}, indentStr string) error {
	if _, err := fmt.Fprintf(w, "%s%s = ", indentStr, name); err != nil {
		return err
	}

	if err := writeValue(w, value, false); err != nil {
		return err
	}

	_, err := fmt.Fprintln(w)
	return err
}

// writeString writes a string value, handling expr() wrapper.
func writeString(w io.Writer, s string) error {
	// Check for expr() wrapper
	if strings.HasPrefix(s, "expr(") && strings.HasSuffix(s, ")") {
		// Unwrap and write expression as-is (unquoted)
		expr := s[5 : len(s)-1]
		_, err := fmt.Fprint(w, expr)
		return err
	}

	// Regular string - quote it
	// TODO: Handle escaping properly (newlines, quotes, etc.)
	_, err := fmt.Fprintf(w, "%q", s)
	return err
}

// writeFloat writes a float, avoiding unnecessary decimal points for whole numbers.
func writeFloat(w io.Writer, f float64) error {
	if f == float64(int64(f)) {
		// Whole number
		_, err := fmt.Fprintf(w, "%d", int64(f))
		return err
	}
	_, err := fmt.Fprintf(w, "%v", f)
	return err
}

// writeArray writes an array in Alloy syntax.
func writeArray(w io.Writer, arr []interface{}) error {
	if len(arr) == 0 {
		_, err := fmt.Fprint(w, "[]")
		return err
	}

	if _, err := fmt.Fprint(w, "["); err != nil {
		return err
	}
	for i, elem := range arr {
		if i > 0 {
			if _, err := fmt.Fprint(w, ", "); err != nil {
				return err
			}
		}
		if err := writeValue(w, elem, false); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(w, "]")
	return err
}

// writeTopLevelArray handles an array at the top level.
// In the new format, this is an array of single-key maps representing statements.
func writeTopLevelArray(w io.Writer, arr []interface{}, indent int) error {
	return writeBodyArray(w, arr, indent)
}

// writeObjectLiteral writes an object literal in Alloy syntax.
func writeObjectLiteral(w io.Writer, obj map[string]interface{}) error {
	if len(obj) == 0 {
		_, err := fmt.Fprint(w, "{}")
		return err
	}

	if _, err := fmt.Fprint(w, "{ "); err != nil {
		return err
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, key := range keys {
		if i > 0 {
			if _, err := fmt.Fprint(w, ", "); err != nil {
				return err
			}
		}

		// Write key (quote if needed)
		if needsQuoting(key) {
			if _, err := fmt.Fprintf(w, "%q = ", key); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, "%s = ", key); err != nil {
				return err
			}
		}

		// Write value
		if err := writeValue(w, obj[key], false); err != nil {
			return err
		}
	}

	_, err := fmt.Fprint(w, " }")
	return err
}

// needsQuoting returns true if a string needs to be quoted as an identifier.
func needsQuoting(s string) bool {
	return !scanner.IsValidIdentifier(s)
}
