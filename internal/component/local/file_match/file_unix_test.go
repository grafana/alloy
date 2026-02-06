//go:build !windows

package file_match

import (
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/local/file_match/testutil"
)

// TestCaseSensitiveGlobMatching verifies that glob patterns are case-sensitive on Unix.
// A pattern with lowercase extension should NOT match files with uppercase extension.
func TestCaseSensitiveGlobMatching(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "case_sensitive_glob")
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	// Create a file with lowercase extension
	testutil.WriteFile(t, dir, "test.log")

	// Search with uppercase glob pattern - should NOT match on Unix (case-sensitive)
	c := testCreateComponent(t, dir, []string{path.Join(dir, "*.LOG")}, nil)
	c.args.SyncPeriod = 10 * time.Millisecond
	err = c.Update(c.args)
	require.NoError(t, err)

	foundFiles := c.getWatchedFiles()
	require.Len(t, foundFiles, 0, "Unix should be case-sensitive: *.LOG should not match test.log")
}

// TestCaseSensitiveGlobMatchingWithCorrectCase verifies the pattern matches when case is correct.
func TestCaseSensitiveGlobMatchingWithCorrectCase(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "case_sensitive_glob_correct")
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	// Create a file with lowercase extension
	testutil.WriteFile(t, dir, "test.log")

	// Search with matching case - should find the file
	c := testCreateComponent(t, dir, []string{path.Join(dir, "*.log")}, nil)
	c.args.SyncPeriod = 10 * time.Millisecond
	err = c.Update(c.args)
	require.NoError(t, err)

	foundFiles := c.getWatchedFiles()
	require.Len(t, foundFiles, 1, "Pattern with matching case should find the file")
	require.True(t, testutil.PathEndsWith(foundFiles, "test.log"))
}
