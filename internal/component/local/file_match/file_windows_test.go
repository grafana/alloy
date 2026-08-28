//go:build windows

package file_match

import (
	"os"
	"path"
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
