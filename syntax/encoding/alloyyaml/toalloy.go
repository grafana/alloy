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
// The YAML uses an array-based structure to preserve order:
//   - Root level is a YAML array
//   - Block bodies are YAML arrays of single-key maps
//   - Each statement (attribute or block) is a single-key map
//   - Block labels can be specified using "/" separator (e.g., "block_name/label")
//   - Special keys: $object (for object literals), $array (for array literals)
//   - The expr() wrapper can be used for complex Alloy expressions
//
// Example YAML:
//
//	- logging:
//	    - level: debug
//	    - format: json
//
// Converts to Alloy:
//
//	logging {
//	  level = "debug"
//	  format = "json"
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
			fmt.Fprintln(w)
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
			fmt.Fprintln(w)
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

		// Check for $object marker - convert to object literal attribute
		if objValue, hasObject := v["$object"]; hasObject {
			if err := writeAttribute(w, blockName, objValue, indentStr); err != nil {
				return fmt.Errorf("attribute %s: %w", blockName, err)
			}
			return nil
		}

		// If label is present from "/" syntax, write labeled block
		if label != "" {
			fmt.Fprintf(w, "%s%s %q {\n", indentStr, blockName, label)
			if err := writeBody(w, v, indent+1); err != nil {
				return err
			}
			fmt.Fprintf(w, "%s}\n", indentStr)
			return nil
		}

		// Regular map values could be blocks (old format) or need to check for array (new format)
		// In new format, blocks have array bodies
		if err := writeBlock(w, blockName, value, indent); err != nil {
			return fmt.Errorf("block %s: %w", blockName, err)
		}

	case []interface{}:
		// In new format, arrays are block bodies
		// Empty arrays or arrays of single-key maps are blocks
		if len(v) == 0 {
			// Empty block
			if label != "" {
				fmt.Fprintf(w, "%s%s %q { }\n", indentStr, blockName, label)
			} else {
				fmt.Fprintf(w, "%s%s { }\n", indentStr, blockName)
			}
			return nil
		}
		
		if firstElem, ok := v[0].(map[string]interface{}); ok && len(firstElem) == 1 {
			// This is a block body
			if label != "" {
				fmt.Fprintf(w, "%s%s %q {\n", indentStr, blockName, label)
			} else {
				fmt.Fprintf(w, "%s%s {\n", indentStr, blockName)
			}
			if err := writeBodyArray(w, v, indent+1); err != nil {
				return err
			}
			fmt.Fprintf(w, "%s}\n", indentStr)
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
		// Check if it's not a special marker
		if _, hasArray := v["$array"]; hasArray {
			return false
		}
		if _, hasObject := v["$object"]; hasObject {
			return false
		}
		return true
	}
	return false
}

// writeBlock writes a block in Alloy syntax.
// In the new format, blocks have array bodies (arrays of single-key maps).
func writeBlock(w io.Writer, name string, value interface{}, indent int) error {
	indentStr := strings.Repeat("  ", indent)

	switch v := value.(type) {
	case []interface{}:
		// New format: array body
		fmt.Fprintf(w, "%s%s {\n", indentStr, name)
		if err := writeBodyArray(w, v, indent+1); err != nil {
			return err
		}
		fmt.Fprintf(w, "%s}\n", indentStr)

	case map[string]interface{}:
		// Old format compatibility: map body
		fmt.Fprintf(w, "%s%s {\n", indentStr, name)
		if err := writeBody(w, v, indent+1); err != nil {
			return err
		}
		fmt.Fprintf(w, "%s}\n", indentStr)

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

// writeTopLevelArray handles an array at the top level.
// In the new format, this is an array of single-key maps representing statements.
func writeTopLevelArray(w io.Writer, arr []interface{}, indent int) error {
	return writeBodyArray(w, arr, indent)
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

