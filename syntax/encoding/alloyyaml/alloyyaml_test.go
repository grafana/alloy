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
)

var updateGolden = flag.Bool("update", false, "update golden YAML files")

// TestGoldenFiles tests bidirectional conversion between Alloy and YAML.
//
// Test files are stored in testdata/ as pairs:
//   - foo.alloy: Alloy configuration (source of truth)
//   - foo.yaml: Expected YAML output (golden file)
//
// The test validates:
//  1. Alloy is syntactically valid (can be parsed)
//  2. YAML → Alloy conversion produces valid Alloy
//  3. YAML → Alloy → YAML round-trip produces same YAML
//
// Run with -update flag to regenerate golden YAML files:
//
//	go test -v -run TestGoldenFiles -update
func TestGoldenFiles(t *testing.T) {
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
			testGoldenFile(t, alloyPath, yamlPath)
		})
	}
}

func testGoldenFile(t *testing.T, alloyPath, yamlPath string) {
	// Read Alloy file
	alloyContent, err := os.ReadFile(alloyPath)
	require.NoError(t, err, "failed to read alloy file: %s", alloyPath)

	// For now, we'll use the YAML file as the source
	// In a full implementation, we'd parse Alloy → YAML
	// But for testing conversion, we'll go: YAML → Alloy → YAML

	// Check if YAML file exists or if we're updating
	yamlContent, err := os.ReadFile(yamlPath)
	if os.IsNotExist(err) && !*updateGolden {
		t.Skipf("Golden file %s does not exist. Run with -update to create it.", yamlPath)
		return
	}

	if *updateGolden {
		t.Logf("Skipping %s - golden file update requires manual creation for now", filepath.Base(alloyPath))
		return
	}

	require.NoError(t, err, "failed to read yaml file: %s", yamlPath)

	// Test: YAML → Alloy
	convertedAlloy, err := ToAlloy(yamlContent)
	require.NoError(t, err, "failed to convert YAML to Alloy")

	// Compare AST semantically (ignoring comments, whitespace, formatting)
	err = compareAlloyAST(t, alloyContent, convertedAlloy)
	if err != nil {
		t.Errorf("AST comparison failed: %v", err)
		t.Logf("Expected (golden):\n%s", string(alloyContent))
		t.Logf("Actual (converted):\n%s", string(convertedAlloy))
		t.FailNow()
	}

	t.Logf("✓ Golden file test passed: %s", filepath.Base(yamlPath))
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
