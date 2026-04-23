package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRender_SubstitutesKnownKeys(t *testing.T) {
	got := Render("name: ${TEST_NAMESPACE}\nlabel: ${TEST_ID}\n", map[string]string{
		"TEST_ID":        "abc",
		"TEST_NAMESPACE": "ns",
	})
	if !strings.Contains(got, "name: ns") || !strings.Contains(got, "label: abc") {
		t.Fatalf("unexpected render output: %q", got)
	}
	if strings.Contains(got, "${") {
		t.Fatalf("render left placeholders unsubstituted: %q", got)
	}
}

// Render is intentionally naive: unknown placeholders pass through. This
// test documents that behavior so a future silent rename of a runtime key
// is caught.
func TestRender_UnknownPlaceholderPassesThrough(t *testing.T) {
	got := Render("x: ${TEST_ID}\ny: ${UNKNOWN}\n", map[string]string{"TEST_ID": "abc"})
	if !strings.Contains(got, "x: abc") {
		t.Fatalf("expected substitution, got %q", got)
	}
	if !strings.Contains(got, "${UNKNOWN}") {
		t.Fatalf("expected unknown placeholder to pass through, got %q", got)
	}
}

func TestRender_ReplacesAllOccurrences(t *testing.T) {
	got := Render("a: ${TEST_ID}\nb: ${TEST_ID}\nc: ${TEST_ID}-suffix\n", map[string]string{
		"TEST_ID": "xyz",
	})
	if strings.Count(got, "xyz") != 3 {
		t.Fatalf("expected 3 substitutions, got %q", got)
	}
}

func TestRender_EmptyVars(t *testing.T) {
	input := "name: ${TEST_ID}\n"
	got := Render(input, nil)
	if got != input {
		t.Fatalf("expected unchanged output with no vars, got %q", got)
	}
}

// A var's value must be used literally even if it happens to contain
// another placeholder expression. This guards against a footgun where, for
// example, ${A} = "xx-${B}-yy" would recursively expand ${B}.
func TestRender_NoRecursiveExpansion(t *testing.T) {
	got := Render("x: ${A}\n", map[string]string{
		"A": "xx-${B}-yy",
		"B": "inner",
	})
	if !strings.Contains(got, "xx-${B}-yy") {
		t.Fatalf("expected literal value, got %q", got)
	}
	if strings.Contains(got, "inner") {
		t.Fatalf("B must not have been recursively expanded, got %q", got)
	}
}

// Bare $VAR should not be substituted because YAML/Helm values files
// frequently contain shell-like strings that are not intended as template
// references.
func TestRender_BareDollarNotSubstituted(t *testing.T) {
	got := Render("cmd: echo $HOME\nx: ${TEST_ID}\n", map[string]string{
		"HOME":    "replaced",
		"TEST_ID": "abc",
	})
	if !strings.Contains(got, "echo $HOME") {
		t.Fatalf("bare $HOME must not be expanded, got %q", got)
	}
	if !strings.Contains(got, "x: abc") {
		t.Fatalf("brace form must still expand, got %q", got)
	}
}

func TestRenderFile_MissingFile(t *testing.T) {
	if _, err := RenderFile("/nonexistent/does-not-exist.yaml", nil); err == nil {
		t.Fatal("expected error for missing input file")
	}
}

func TestRenderFile_WritesSubstitutedContent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "input.yaml")
	if err := os.WriteFile(src, []byte("name: ${TEST_ID}\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	rendered, err := RenderFile(src, map[string]string{"TEST_ID": "xyz"})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	defer os.Remove(rendered)

	raw, err := os.ReadFile(rendered)
	if err != nil {
		t.Fatalf("read rendered: %v", err)
	}
	if !strings.Contains(string(raw), "name: xyz") {
		t.Fatalf("unexpected rendered content: %q", string(raw))
	}
}
