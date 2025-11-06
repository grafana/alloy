package alloyyaml

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/token"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var updateGolden = flag.Bool("update", false, "update golden YAML files")

// TestYAMLToAlloy tests YAML → Alloy conversion using golden files.
//
// Test files are stored in testdata/ as pairs:
//   - foo.yaml: YAML input (hand-written golden file)
//   - foo.alloy: Expected Alloy output (golden file)
//
// The test validates:
//  1. YAML → Alloy conversion produces valid Alloy
//  2. The generated Alloy matches the golden file (semantically via AST comparison)
//
// Run with -update flag to skip tests (golden files must be created manually):
//
//	go test -v -run TestYAMLToAlloy -update
func TestYAMLToAlloy(t *testing.T) {
	testdataDir := "testdata"

	// Find all .alloy files in testdata
	alloyFiles, err := filepath.Glob(filepath.Join(testdataDir, "*.alloy"))
	require.NoError(t, err, "failed to list testdata files")

	if len(alloyFiles) == 0 {
		t.Skip("no testdata files found")
	}

	for _, alloyPath := range alloyFiles {
		// Get base name without extension
		baseName := strings.TrimSuffix(filepath.Base(alloyPath), ".alloy")
		yamlPath := filepath.Join(testdataDir, baseName+".yaml")

		t.Run(baseName, func(t *testing.T) {
			testYAMLToAlloy(t, alloyPath, yamlPath)
		})
	}
}

func testYAMLToAlloy(t *testing.T, alloyPath, yamlPath string) {
	// Read golden Alloy file
	expectedAlloy, err := os.ReadFile(alloyPath)
	require.NoError(t, err, "failed to read alloy file: %s", alloyPath)

	// Check if YAML file exists or if we're updating
	yamlContent, err := os.ReadFile(yamlPath)
	if os.IsNotExist(err) && !*updateGolden {
		t.Skipf("Golden file %s does not exist. Run with -update to create it.", yamlPath)
		return
	}

	if *updateGolden {
		t.Logf("Skipping %s - golden file update requires manual creation", filepath.Base(alloyPath))
		return
	}

	require.NoError(t, err, "failed to read yaml file: %s", yamlPath)

	// Convert YAML → Alloy
	actualAlloy, err := ToAlloy(yamlContent)
	require.NoError(t, err, "failed to convert YAML to Alloy")

	// Compare AST semantically (ignoring comments, whitespace, formatting)
	err = compareAlloyAST(t, expectedAlloy, actualAlloy)
	if err != nil {
		t.Errorf("AST comparison failed: %v", err)
		t.Logf("Expected Alloy (golden):\n%s", string(expectedAlloy))
		t.Logf("Actual Alloy (converted):\n%s", string(actualAlloy))
		t.FailNow()
	}

	t.Logf("✓ YAML → Alloy test passed: %s", filepath.Base(yamlPath))
}

// compareAlloyAST parses both Alloy strings and compares their AST structures semantically.
// It ignores comments, whitespace, formatting differences, and ordering.
// Returns an error with a detailed message if the ASTs differ.
func compareAlloyAST(t *testing.T, expected, actual []byte) error {
	// Parse expected Alloy
	expectedAST, err := parser.ParseFile("expected.alloy", expected)
	if err != nil {
		return fmt.Errorf("failed to parse expected Alloy: %w", err)
	}

	// Parse actual Alloy
	actualAST, err := parser.ParseFile("actual.alloy", actual)
	if err != nil {
		return fmt.Errorf("failed to parse actual Alloy: %w", err)
	}

	// Normalize ASTs: remove position info, clear comments, and sort everything
	normalizeAST(expectedAST)
	normalizeAST(actualAST)

	// Use reflect.DeepEqual for comparison
	if !reflect.DeepEqual(expectedAST.Body, actualAST.Body) {
		return fmt.Errorf("AST mismatch: structures differ")
	}

	return nil
}

// normalizeAST removes position information, comments, and sorts all elements
// to enable order-independent comparison using reflect.DeepEqual.
func normalizeAST(file *ast.File) {
	// Clear comments (they're not semantic)
	file.Comments = nil

	// Sort and sanitize body
	sortAndSanitizeBody(file.Body)
}

// sortAndSanitizeBody sorts statements and recursively normalizes them.
func sortAndSanitizeBody(body ast.Body) {
	// Sort statements: attributes first (by name), then blocks (by name+label)
	sort.Slice(body, func(i, j int) bool {
		return stmtSortKey(body[i]) < stmtSortKey(body[j])
	})

	// Sanitize each statement
	for _, stmt := range body {
		sanitizeStmt(stmt)
	}
}

// stmtSortKey returns a sort key for a statement.
func stmtSortKey(stmt ast.Stmt) string {
	switch s := stmt.(type) {
	case *ast.AttributeStmt:
		return "attr:" + s.Name.Name
	case *ast.BlockStmt:
		return "block:" + strings.Join(s.Name, ".") + ":" + s.Label
	default:
		return ""
	}
}

func sanitizeStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.AttributeStmt:
		sanitizeIdent(s.Name)
		sanitizeExpr(s.Value)
	case *ast.BlockStmt:
		s.NamePos = token.Pos{}
		s.LabelPos = token.Pos{}
		s.LCurlyPos = token.Pos{}
		s.RCurlyPos = token.Pos{}
		// Recursively sort and sanitize block body
		sortAndSanitizeBody(s.Body)
	}
}

func sanitizeIdent(ident *ast.Ident) {
	if ident != nil {
		ident.NamePos = token.Pos{}
	}
}

func sanitizeExpr(expr ast.Expr) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.LiteralExpr:
		e.ValuePos = token.Pos{}
	case *ast.IdentifierExpr:
		sanitizeIdent(e.Ident)
	case *ast.ArrayExpr:
		e.LBrackPos = token.Pos{}
		e.RBrackPos = token.Pos{}
		for _, elem := range e.Elements {
			sanitizeExpr(elem)
		}
	case *ast.ObjectExpr:
		e.LCurlyPos = token.Pos{}
		e.RCurlyPos = token.Pos{}
		// Sort object fields by name
		sort.Slice(e.Fields, func(i, j int) bool {
			return e.Fields[i].Name.Name < e.Fields[j].Name.Name
		})
		for _, field := range e.Fields {
			sanitizeIdent(field.Name)
			sanitizeExpr(field.Value)
		}
	case *ast.AccessExpr:
		sanitizeExpr(e.Value)
		sanitizeIdent(e.Name)
	case *ast.IndexExpr:
		e.LBrackPos = token.Pos{}
		e.RBrackPos = token.Pos{}
		sanitizeExpr(e.Value)
		sanitizeExpr(e.Index)
	case *ast.CallExpr:
		e.LParenPos = token.Pos{}
		e.RParenPos = token.Pos{}
		sanitizeExpr(e.Value)
		for _, arg := range e.Args {
			sanitizeExpr(arg)
		}
	case *ast.UnaryExpr:
		e.KindPos = token.Pos{}
		sanitizeExpr(e.Value)
	case *ast.BinaryExpr:
		e.KindPos = token.Pos{}
		sanitizeExpr(e.Left)
		sanitizeExpr(e.Right)
	case *ast.ParenExpr:
		e.LParenPos = token.Pos{}
		e.RParenPos = token.Pos{}
		sanitizeExpr(e.Inner)
	}
}

// TestAlloyToYAML tests Alloy → YAML conversion using the same golden files.
//
// Test files are stored in testdata/ as pairs:
//   - foo.alloy: Alloy configuration (input)
//   - foo.yaml: Expected YAML output (golden file)
//
// The test validates:
//  1. Alloy → YAML conversion produces valid YAML
//  2. The generated YAML matches the golden file (semantically)
//
// Run with -args -update to regenerate golden YAML files from Alloy:
//
//	go test -v -run TestAlloyToYAML -args -update
//	go test -v -run TestAlloyToYAML/13_traces -args -update  # Update specific test
func TestAlloyToYAML(t *testing.T) {
	testdataDir := "testdata"

	// Find all .alloy files in testdata
	alloyFiles, err := filepath.Glob(filepath.Join(testdataDir, "*.alloy"))
	require.NoError(t, err, "failed to list testdata files")

	if len(alloyFiles) == 0 {
		t.Skip("no testdata files found")
	}

	for _, alloyPath := range alloyFiles {
		// Get base name without extension
		baseName := strings.TrimSuffix(filepath.Base(alloyPath), ".alloy")
		yamlPath := filepath.Join(testdataDir, baseName+".yaml")

		t.Run(baseName, func(t *testing.T) {
			testAlloyToYAML(t, alloyPath, yamlPath)
		})
	}
}

func testAlloyToYAML(t *testing.T, alloyPath, yamlPath string) {
	// Read Alloy file
	alloyContent, err := os.ReadFile(alloyPath)
	require.NoError(t, err, "failed to read alloy file: %s", alloyPath)

	// Convert Alloy → YAML
	actualYAML, err := ToYAML(alloyContent)
	require.NoError(t, err, "failed to convert Alloy to YAML")

	// If update flag is set, write the generated YAML and skip comparison
	if *updateGolden {
		err := os.WriteFile(yamlPath, actualYAML, 0644)
		require.NoError(t, err, "failed to write golden file: %s", yamlPath)
		t.Logf("✓ Updated golden file: %s", filepath.Base(yamlPath))
		return
	}

	// Read expected YAML file
	expectedYAML, err := os.ReadFile(yamlPath)
	if os.IsNotExist(err) {
		t.Skipf("Golden file %s does not exist. Run with -args -update to create it.", yamlPath)
		return
	}
	require.NoError(t, err, "failed to read yaml file: %s", yamlPath)

	// Compare YAML data structures (ignoring comments, ordering, formatting)
	err = compareYAMLData(t, expectedYAML, actualYAML)
	if err != nil {
		t.Errorf("YAML comparison failed: %v", err)
		t.Logf("Expected YAML:\n%s", string(expectedYAML))
		t.Logf("Actual YAML:\n%s", string(actualYAML))
		t.FailNow()
	}

	t.Logf("✓ Alloy → YAML test passed: %s", filepath.Base(alloyPath))
}

// compareYAMLData parses both YAML strings and compares their data structures.
// It ignores comments, whitespace, and key ordering.
func compareYAMLData(t *testing.T, expected, actual []byte) error {
	var expectedData, actualData interface{}

	if err := yaml.Unmarshal(expected, &expectedData); err != nil {
		return fmt.Errorf("failed to parse expected YAML: %w", err)
	}

	if err := yaml.Unmarshal(actual, &actualData); err != nil {
		return fmt.Errorf("failed to parse actual YAML: %w", err)
	}

	// Normalize both data structures
	expectedData = normalizeYAMLData(expectedData)
	actualData = normalizeYAMLData(actualData)

	// Use reflect.DeepEqual for comparison
	if !reflect.DeepEqual(expectedData, actualData) {
		return fmt.Errorf("YAML data structures differ")
	}

	return nil
}

// normalizeYAMLData recursively normalizes YAML data for comparison.
// This ensures consistent types and ordering for comparison.
func normalizeYAMLData(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		// Unwrap $array marker if present
		if arrValue, hasArray := v["$array"]; hasArray && len(v) == 1 {
			return normalizeYAMLData(arrValue)
		}

		result := make(map[string]interface{})
		for key, val := range v {
			result[key] = normalizeYAMLData(val)
		}
		return result

	case map[interface{}]interface{}:
		// yaml.v3 sometimes parses maps with interface{} keys
		result := make(map[string]interface{})
		for key, val := range v {
			keyStr := fmt.Sprintf("%v", key)
			result[keyStr] = normalizeYAMLData(val)
		}
		// Unwrap $array marker if present
		if arrValue, hasArray := result["$array"]; hasArray && len(result) == 1 {
			return normalizeYAMLData(arrValue)
		}
		return result

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, elem := range v {
			result[i] = normalizeYAMLData(elem)
		}
		return result

	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		if v <= 9223372036854775807 {
			return int64(v)
		}
		return v
	case float32:
		return float64(v)
	case float64:
		return v
	case string:
		return v
	case bool:
		return v
	case nil:
		return nil

	default:
		// Unknown type, return as-is
		return v
	}
}
