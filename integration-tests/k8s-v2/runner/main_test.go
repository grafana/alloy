package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/planner"
)

func TestResolveSelection_ByNameAndPath(t *testing.T) {
	base := t.TempDir()
	logsDir := filepath.Join(base, "logs-loki")
	metricsDir := filepath.Join(base, "metrics-mimir")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatalf("mkdir logs dir: %v", err)
	}
	if err := os.MkdirAll(metricsDir, 0o755); err != nil {
		t.Fatalf("mkdir metrics dir: %v", err)
	}
	all := []planner.TestCase{
		{Name: "logs-loki", Dir: logsDir},
		{Name: "metrics-mimir", Dir: metricsDir},
	}

	resolved, err := resolveSelection(options{tests: []string{"logs-loki", metricsDir}}, all)
	if err != nil {
		t.Fatalf("resolve selection failed: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("expected two selected tests, got %d", len(resolved))
	}
	if resolved[0].Name != "logs-loki" || resolved[1].Name != "metrics-mimir" {
		t.Fatalf("unexpected resolved tests: %#v", resolved)
	}
}

func TestResolveSelection_RejectsAllAndTest(t *testing.T) {
	_, err := resolveSelection(options{all: true, tests: []string{"x"}}, []planner.TestCase{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestBuildGoTestArgs(t *testing.T) {
	opts := options{
		verbose:          true,
		debug:            true,
		timeout:          30 * time.Minute,
		setupTimeout:     10 * time.Minute,
		readinessTimeout: 90 * time.Second,
		keepCluster:      true,
		keepDeps:         true,
		reuseCluster:     "cluster-a",
		reuseDeps:        true,
		alloyImage:       "alloy-ci:test-sha",
		alloyPullPolicy:  "IfNotPresent",
	}
	selected := []selectedTest{
		{Name: "logs-loki"},
		{Name: "metrics-mimir"},
	}
	got := buildGoTestArgs(opts, selected, []string{"--count=1"})
	wantFragments := []string{
		"test", "-v", "-tags", integrationGoTags, "-timeout", "30m0s", packageDir,
		"--count=1", "-args", "-k8s.v2.tests=logs-loki,metrics-mimir",
		"-k8s.v2.setup-timeout=10m0s", "-k8s.v2.readiness-timeout=1m30s",
		"-k8s.v2.keep-cluster=true", "-k8s.v2.keep-deps=true",
		"-k8s.v2.reuse-cluster=cluster-a", "-k8s.v2.reuse-deps=true", "-k8s.v2.debug=true",
		"-k8s.v2.alloy-image=alloy-ci:test-sha", "-k8s.v2.alloy-image-pull-policy=IfNotPresent",
	}
	for _, fragment := range wantFragments {
		if !slices.Contains(got, fragment) {
			t.Fatalf("missing argument %q in %v", fragment, got)
		}
	}
}
