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

// TestYAMLToAlloy tests YAML → Alloy conversion.
// YAML files are hand-written inputs, Alloy files are the expected outputs.
func TestYAMLToAlloy(t *testing.T) {
	runGoldenTests(t, func(t *testing.T, alloyPath, yamlPath string) {
		expectedAlloy, err := os.ReadFile(alloyPath)
		require.NoError(t, err)

		yamlContent, err := os.ReadFile(yamlPath)
		require.NoError(t, err)

		actualAlloy, err := ToAlloy(yamlContent)
		require.NoError(t, err, "YAML → Alloy conversion failed")

		assertAlloyEqual(t, expectedAlloy, actualAlloy, "YAML → Alloy")
	})
}

// TestAlloyToYAML tests Alloy → YAML → Alloy round-trip conversion.
// Uses round-trip testing: Alloy → YAML → Alloy, comparing original with result.
// Run with -args -update to regenerate YAML golden files from Alloy.
func TestAlloyToYAML(t *testing.T) {
	runGoldenTests(t, func(t *testing.T, alloyPath, yamlPath string) {
		alloyContent, err := os.ReadFile(alloyPath)
		require.NoError(t, err)

		actualYAML, err := ToYAML(alloyContent)
		require.NoError(t, err, "Alloy → YAML conversion failed")

		if *updateGolden {
			err := os.WriteFile(yamlPath, actualYAML, 0644)
			require.NoError(t, err)
			t.Logf("✓ Updated golden file: %s", filepath.Base(yamlPath))
			return
		}

		// Round-trip: convert YAML back to Alloy and compare
		roundTripAlloy, err := ToAlloy(actualYAML)
		require.NoError(t, err, "YAML → Alloy conversion failed")

		assertAlloyEqualWithYAML(t, alloyContent, roundTripAlloy, actualYAML)
	})
}

// runGoldenTests discovers and runs golden file tests in testdata/.
func runGoldenTests(t *testing.T, testFn func(t *testing.T, alloyPath, yamlPath string)) {
	alloyFiles, err := filepath.Glob(filepath.Join("testdata", "*.alloy"))
	require.NoError(t, err)
	if len(alloyFiles) == 0 {
		t.Skip("no testdata files found")
	}

	for _, alloyPath := range alloyFiles {
		baseName := strings.TrimSuffix(filepath.Base(alloyPath), ".alloy")
		yamlPath := filepath.Join("testdata", baseName+".yaml")
		t.Run(baseName, func(t *testing.T) {
			testFn(t, alloyPath, yamlPath)
		})
	}
}

// assertAlloyEqual compares two Alloy configurations using AST comparison.
func assertAlloyEqual(t *testing.T, expected, actual []byte, label string) {
	if err := compareAlloyAST(expected, actual); err != nil {
		t.Errorf("%s: AST comparison failed: %v", label, err)
		t.Logf("Expected:\n%s", string(expected))
		t.Logf("Actual:\n%s", string(actual))
		t.FailNow()
	}
	t.Logf("✓ %s test passed", label)
}

// assertAlloyEqualWithYAML is like assertAlloyEqual but also logs the intermediate YAML.
func assertAlloyEqualWithYAML(t *testing.T, expected, actual, yaml []byte) {
	if err := compareAlloyAST(expected, actual); err != nil {
		t.Errorf("Round-trip AST comparison failed: %v", err)
		t.Logf("Original Alloy:\n%s", string(expected))
		t.Logf("YAML (intermediate):\n%s", string(yaml))
		t.Logf("Round-trip Alloy:\n%s", string(actual))
		t.FailNow()
	}
	t.Logf("✓ Alloy → YAML → Alloy round-trip test passed")
}

// compareAlloyAST parses and compares two Alloy configurations semantically.
// Ignores comments, whitespace, formatting, and ordering.
func compareAlloyAST(expected, actual []byte) error {
	expectedAST, err := parser.ParseFile("expected.alloy", expected)
	if err != nil {
		return fmt.Errorf("parse expected: %w", err)
	}

	actualAST, err := parser.ParseFile("actual.alloy", actual)
	if err != nil {
		return fmt.Errorf("parse actual: %w", err)
	}

	normalizeAST(expectedAST)
	normalizeAST(actualAST)

	if !reflect.DeepEqual(expectedAST.Body, actualAST.Body) {
		return fmt.Errorf("AST structures differ")
	}
	return nil
}

// normalizeAST removes non-semantic information (positions, comments) and sorts elements.
func normalizeAST(file *ast.File) {
	file.Comments = nil
	normalizeBody(file.Body)
}

// normalizeBody sorts and sanitizes statements recursively.
func normalizeBody(body ast.Body) {
	sort.Slice(body, func(i, j int) bool {
		return sortKey(body[i]) < sortKey(body[j])
	})
	for _, stmt := range body {
		normalizeStmt(stmt)
	}
}

// sortKey returns a sort key for ordering statements.
func sortKey(stmt ast.Stmt) string {
	switch s := stmt.(type) {
	case *ast.AttributeStmt:
		return "attr:" + s.Name.Name
	case *ast.BlockStmt:
		return "block:" + strings.Join(s.Name, ".") + ":" + s.Label
	default:
		return ""
	}
}

func normalizeStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.AttributeStmt:
		clearPos(s.Name)
		normalizeExpr(s.Value)
	case *ast.BlockStmt:
		s.NamePos, s.LabelPos, s.LCurlyPos, s.RCurlyPos = token.Pos{}, token.Pos{}, token.Pos{}, token.Pos{}
		normalizeBody(s.Body)
	}
}

func normalizeExpr(expr ast.Expr) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.LiteralExpr:
		e.ValuePos = token.Pos{}
	case *ast.IdentifierExpr:
		clearPos(e.Ident)
	case *ast.ArrayExpr:
		e.LBrackPos, e.RBrackPos = token.Pos{}, token.Pos{}
		for _, elem := range e.Elements {
			normalizeExpr(elem)
		}
	case *ast.ObjectExpr:
		e.LCurlyPos, e.RCurlyPos = token.Pos{}, token.Pos{}
		sort.Slice(e.Fields, func(i, j int) bool {
			return e.Fields[i].Name.Name < e.Fields[j].Name.Name
		})
		for _, field := range e.Fields {
			clearPos(field.Name)
			normalizeExpr(field.Value)
		}
	case *ast.AccessExpr:
		normalizeExpr(e.Value)
		clearPos(e.Name)
	case *ast.IndexExpr:
		e.LBrackPos, e.RBrackPos = token.Pos{}, token.Pos{}
		normalizeExpr(e.Value)
		normalizeExpr(e.Index)
	case *ast.CallExpr:
		e.LParenPos, e.RParenPos = token.Pos{}, token.Pos{}
		normalizeExpr(e.Value)
		for _, arg := range e.Args {
			normalizeExpr(arg)
		}
	case *ast.UnaryExpr:
		e.KindPos = token.Pos{}
		normalizeExpr(e.Value)
	case *ast.BinaryExpr:
		e.KindPos = token.Pos{}
		normalizeExpr(e.Left)
		normalizeExpr(e.Right)
	case *ast.ParenExpr:
		e.LParenPos, e.RParenPos = token.Pos{}, token.Pos{}
		normalizeExpr(e.Inner)
	}
}

func clearPos(ident *ast.Ident) {
	if ident != nil {
		ident.NamePos = token.Pos{}
	}
}
