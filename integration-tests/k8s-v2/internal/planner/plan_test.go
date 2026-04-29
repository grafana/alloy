package planner

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSelectTests_All(t *testing.T) {
	all := []TestCase{{Name: "a"}, {Name: "b"}}
	got, err := SelectTests(all, "all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 tests, got %d", len(got))
	}
}

func TestSelectTests_Unknown(t *testing.T) {
	all := []TestCase{{Name: "a"}}
	_, err := SelectTests(all, "missing")
	if err == nil {
		t.Fatal("expected unknown test error")
	}
}

func TestRequirementsSet(t *testing.T) {
	selected := []TestCase{
		{Name: "a", Requires: []string{"loki", "mimir"}},
		{Name: "b", Requires: []string{"loki"}},
	}
	got := RequirementsSet(selected)
	want := []string{"loki", "mimir"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected requirements set: got=%v want=%v", got, want)
	}
}

func TestDiscoverTests(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "metrics"), 0o755); err != nil {
		t.Fatalf("mkdir metrics: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "metrics", "requirements.yaml"), []byte("requires:\n  - mimir\n"), 0o644); err != nil {
		t.Fatalf("write requirements: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "logs"), 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "logs", "requirements.yaml"), []byte("requires:\n  - loki\n"), 0o644); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	tests, err := DiscoverTests(root)
	if err != nil {
		t.Fatalf("discover tests: %v", err)
	}
	if len(tests) != 2 {
		t.Fatalf("expected 2 tests, got %d", len(tests))
	}
	if tests[0].Name != "logs" || tests[1].Name != "metrics" {
		t.Fatalf("tests are not sorted by name: %v", tests)
	}
}
