//go:build windows

package file

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/logging"
)

// TestGlobResolverCaseInsensitive verifies that glob patterns are case-insensitive on Windows.
// An uppercase pattern SHOULD match lowercase files.
func TestGlobResolverCaseInsensitive(t *testing.T) {
	resolver := newGlobResolver(logging.NewSlogNop())

	// Use uppercase pattern - SHOULD match the lowercase .log files on Windows
	targets := []discovery.Target{
		discovery.NewTargetFromLabelSet(model.LabelSet{
			"__path__": "./testdata/*.LOG",
			"label":    "test",
		}),
	}

	var results []resolvedTarget
	for target := range resolver.Resolve(targets) {
		results = append(results, target)
	}

	require.Len(t, results, 2, "Windows should be case-insensitive: *.LOG should match *.log files")
}

// TestGlobResolverCaseInsensitiveLowercase verifies that lowercase patterns also work.
func TestGlobResolverCaseInsensitiveLowercase(t *testing.T) {
	resolver := newGlobResolver(logging.NewSlogNop())

	// Use lowercase pattern - should also match
	targets := []discovery.Target{
		discovery.NewTargetFromLabelSet(model.LabelSet{
			"__path__": "./testdata/*.log",
			"label":    "test",
		}),
	}

	var results []resolvedTarget
	for target := range resolver.Resolve(targets) {
		results = append(results, target)
	}

	require.Len(t, results, 2, "Lowercase pattern should also find the files")
}

func TestGlobResolverThroughJunction(t *testing.T) {
	root := t.TempDir()

	// The junction target lives outside of the directory being globbed, so the
	// only way to reach it is through the junction.
	target := filepath.Join(root, "target")
	require.NoError(t, os.MkdirAll(filepath.Join(target, "Documents"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "Documents", "junction.log"), nil, 0644))

	dir := filepath.Join(root, "users")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ordinary", "Documents"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ordinary", "Documents", "ordinary.log"), nil, 0644))

	junction := filepath.Join(dir, "junction")
	out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, target).CombinedOutput()
	require.NoError(t, err, "failed to create junction: %s", out)

	// Sanity check that Go sees the junction the way the issue describes.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		if e.Name() == "junction" {
			require.False(t, e.IsDir(), "expected ReadDir to not report the junction as a directory")
		}
	}

	// Files must be reported via the junction path, not the resolved target path.
	expected := []string{
		filepath.Join(dir, "ordinary", "Documents", "ordinary.log"),
		filepath.Join(dir, "junction", "Documents", "junction.log"),
	}

	tt := []struct {
		name    string
		pattern string
	}{
		{"wildcard", filepath.Join(dir, "*", "Documents", "*.log")},
		{"recursive", filepath.Join(dir, "**", "*.log")},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			resolver := newGlobResolver(logging.NewSlogNop())
			targets := []discovery.Target{
				discovery.NewTargetFromLabelSet(model.LabelSet{"__path__": model.LabelValue(tc.pattern)}),
			}

			var paths []string
			for r := range resolver.Resolve(targets) {
				paths = append(paths, r.Path)
			}

			require.ElementsMatch(t, expected, paths)
		})
	}
}
