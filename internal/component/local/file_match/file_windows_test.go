//go:build windows

package file_match

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/local/file_match/testutil"
)

// TestCaseInsensitiveGlobMatching verifies that glob patterns are case-insensitive on Windows.
// A pattern with lowercase extension SHOULD match files with uppercase extension.
func TestCaseInsensitiveGlobMatching(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "case_insensitive_glob")
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	// Create a file with uppercase extension
	testutil.WriteFile(t, dir, "test.LOG")

	// Search with lowercase glob pattern - SHOULD match on Windows (case-insensitive)
	c := testCreateComponent(t, dir, []string{path.Join(dir, "*.log")}, nil)
	c.args.SyncPeriod = 10 * time.Millisecond
	err = c.Update(c.args)
	require.NoError(t, err)

	foundFiles := c.getWatchedFiles()
	require.Len(t, foundFiles, 1, "Windows should be case-insensitive: *.log should match test.LOG")
	require.True(t, testutil.PathEndsWith(foundFiles, "test.log"))
}

// TestCaseInsensitiveGlobMatchingUppercasePattern verifies uppercase patterns match lowercase files.
func TestCaseInsensitiveGlobMatchingUppercasePattern(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "case_insensitive_glob_upper")
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	// Create a file with lowercase extension
	testutil.WriteFile(t, dir, "test.log")

	// Search with uppercase glob pattern - SHOULD match on Windows (case-insensitive)
	c := testCreateComponent(t, dir, []string{path.Join(dir, "*.LOG")}, nil)
	c.args.SyncPeriod = 10 * time.Millisecond
	err = c.Update(c.args)
	require.NoError(t, err)

	foundFiles := c.getWatchedFiles()
	require.Len(t, foundFiles, 1, "Windows should be case-insensitive: *.LOG should match test.log")
	require.True(t, testutil.PathEndsWith(foundFiles, "test.log"))
}

func TestGlobMatchingThroughJunction(t *testing.T) {
	root := t.TempDir()

	// The junction target lives outside of the directory being globbed, so the
	// only way to reach it is through the junction.
	target := filepath.Join(root, "target")
	require.NoError(t, os.MkdirAll(filepath.Join(target, "Documents"), 0755))
	testutil.WriteFile(t, filepath.Join(target, "Documents"), "junction.log")

	dir := filepath.Join(root, "users")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ordinary", "Documents"), 0755))
	testutil.WriteFile(t, filepath.Join(dir, "ordinary", "Documents"), "ordinary.log")

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

	tt := []struct {
		name    string
		pattern string
	}{
		{"wildcard", filepath.Join(dir, "*", "Documents", "*.log")},
		{"recursive", filepath.Join(dir, "**", "*.log")},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			c := testCreateComponent(t, dir, []string{tc.pattern}, nil)
			c.args.SyncPeriod = 10 * time.Millisecond
			require.NoError(t, c.Update(c.args))

			foundFiles := c.getWatchedFiles()
			require.Len(t, foundFiles, 2, "expected files from both the ordinary directory and the junction: %v", foundFiles)
			require.True(t, testutil.PathEndsWith(foundFiles, filepath.Join("users", "ordinary", "Documents", "ordinary.log")))
			// The file must be reported via the junction path, not the resolved target path.
			require.True(t, testutil.PathEndsWith(foundFiles, filepath.Join("users", "junction", "Documents", "junction.log")))
			require.False(t, testutil.PathEndsWith(foundFiles, filepath.Join("target", "Documents", "junction.log")))
		})
	}
}
