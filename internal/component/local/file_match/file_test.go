package file_match

import (
	"context"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/local/file_match/testutil"
)

func TestFile(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t1")
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	testutil.WriteFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := testCreateComponent(t, dir, []string{path.Join(dir, "*.txt")}, nil)
	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 5*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 1)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
	}, 10*time.Second, 300*time.Millisecond)
}

func TestDirectoryFile(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t1")
	subdir := path.Join(dir, "subdir")
	err := os.MkdirAll(subdir, 0755)
	require.NoError(t, err)
	testutil.WriteFile(t, subdir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := testCreateComponent(t, dir, []string{path.Join(dir, "**/")}, nil)
	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 5*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 1)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
	}, 10*time.Second, 300*time.Millisecond)
}

func TestFileIgnoreOlder(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t1")
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	testutil.WriteFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := testCreateComponent(t, dir, []string{path.Join(dir, "*.txt")}, nil)
	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 5*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	c.args.IgnoreOlderThan = 100 * time.Millisecond
	c.Update(c.args)
	go c.Run(ct)

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 1)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
	}, 10*time.Second, 300*time.Millisecond)
	time.Sleep(150 * time.Millisecond)

	testutil.WriteFile(t, dir, "t2.txt")
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 1)
		assert.True(collect, testutil.ContainsPath(f, "t2.txt"))
	}, 10*time.Second, 300*time.Millisecond)
}

func TestAddingFile(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t2")
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	testutil.WriteFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := testCreateComponent(t, dir, []string{path.Join(dir, "*.txt")}, nil)

	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 40*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 1)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
	}, 10*time.Second, 300*time.Millisecond)
	testutil.WriteFile(t, dir, "t2.txt")
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 2)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
		assert.True(collect, testutil.ContainsPath(f, "t2.txt"))
	}, 10*time.Second, 300*time.Millisecond)
}

func TestAddingFileInSubDir(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t3")
	os.MkdirAll(dir, 0755)
	testutil.WriteFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := testCreateComponent(t, dir, []string{path.Join(dir, "**", "*.txt")}, nil)
	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 40*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 1)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
	}, 10*time.Second, 300*time.Millisecond)
	testutil.WriteFile(t, dir, "t2.txt")
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 2)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
		assert.True(collect, testutil.ContainsPath(f, "t2.txt"))
	}, 10*time.Second, 300*time.Millisecond)
	subdir := path.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	err := os.WriteFile(path.Join(subdir, "t3.txt"), []byte("asdf"), 0664)
	require.NoError(t, err)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 3)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
		assert.True(collect, testutil.ContainsPath(f, "t2.txt"))
		assert.True(collect, testutil.ContainsPath(f, "t3.txt"))
	}, 10*time.Second, 300*time.Millisecond)
}

func TestAddingFileInAnExcludedSubDir(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t3")
	os.MkdirAll(dir, 0755)
	testutil.WriteFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	included := []string{path.Join(dir, "**", "*.txt")}
	excluded := []string{path.Join(dir, "subdir", "*.txt")}
	c := testCreateComponent(t, dir, included, excluded)
	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 40*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 1)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
	}, 10*time.Second, 300*time.Millisecond)
	testutil.WriteFile(t, dir, "t2.txt")
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 2)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
		assert.True(collect, testutil.ContainsPath(f, "t2.txt"))
	}, 10*time.Second, 300*time.Millisecond)
	subdir := path.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	subdir2 := path.Join(dir, "subdir2")
	os.Mkdir(subdir2, 0755)
	// This file will not be included, since it is in the excluded subdir
	err := os.WriteFile(path.Join(subdir, "exclude_me.txt"), []byte("asdf"), 0664)
	require.NoError(t, err)
	// This file will be included, since it is in another subdir
	err = os.WriteFile(path.Join(subdir2, "another.txt"), []byte("asdf"), 0664)
	require.NoError(t, err)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 3)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
		assert.True(collect, testutil.ContainsPath(f, "t2.txt"))
		assert.True(collect, testutil.ContainsPath(f, "another.txt"))
	}, 10*time.Second, 300*time.Millisecond)
}

func TestAddingRemovingFileInSubDir(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t3")
	os.MkdirAll(dir, 0755)
	testutil.WriteFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := testCreateComponent(t, dir, []string{path.Join(dir, "**", "*.txt")}, nil)

	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 40*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 1)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
	}, 10*time.Second, 300*time.Millisecond)
	testutil.WriteFile(t, dir, "t2.txt")
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 2)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
		assert.True(collect, testutil.ContainsPath(f, "t2.txt"))
	}, 10*time.Second, 300*time.Millisecond)
	subdir := path.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	err := os.WriteFile(path.Join(subdir, "t3.txt"), []byte("asdf"), 0664)
	require.NoError(t, err)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 3)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
		assert.True(collect, testutil.ContainsPath(f, "t2.txt"))
		assert.True(collect, testutil.ContainsPath(f, "t3.txt"))
	}, 10*time.Second, 300*time.Millisecond)

	err = os.RemoveAll(subdir)
	require.NoError(t, err)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 2)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
		assert.True(collect, testutil.ContainsPath(f, "t2.txt"))
	}, 10*time.Second, 300*time.Millisecond)
}

func TestExclude(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t3")
	os.MkdirAll(dir, 0755)
	testutil.WriteFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := testCreateComponent(t, dir, []string{path.Join(dir, "**", "*.txt")}, []string{path.Join(dir, "**", "*.bad")})
	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 40*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 1)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
	}, 10*time.Second, 300*time.Millisecond)
	subdir := path.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	testutil.WriteFile(t, subdir, "t3.txt")
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 2)
		assert.True(collect, testutil.ContainsPath(f, "t1.txt"))
		assert.True(collect, testutil.ContainsPath(f, "t3.txt"))
	}, 10*time.Second, 300*time.Millisecond)
}

// TestMultiplePatterns verifies that the {a,b,c} pattern syntax works for matching
// multiple file extensions, multiple directories, and excluding multiple file types
// in a single glob pattern. This mirrors the documentation example.
// This is a feature of doublestar v4: https://github.com/grafana/alloy/issues/4423
func TestMultiplePatterns(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "multi_pattern")
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	// Create directory structure matching the documentation example:
	// {nginx,apache,caddy}/*.{log,txt,json} excluding *.{gz,zip,bak,old}
	for _, subdir := range []string{"nginx", "apache", "caddy", "other"} {
		subdirPath := path.Join(dir, subdir)
		err := os.MkdirAll(subdirPath, 0755)
		require.NoError(t, err)

		// Create files with various extensions in each directory
		testutil.WriteFile(t, subdirPath, "access.log")
		testutil.WriteFile(t, subdirPath, "error.txt")
		testutil.WriteFile(t, subdirPath, "config.json")
		testutil.WriteFile(t, subdirPath, "debug.yaml")      // Should not match (wrong extension)
		testutil.WriteFile(t, subdirPath, "access.log.gz")   // Should be excluded
		testutil.WriteFile(t, subdirPath, "error.txt.zip")   // Should be excluded
		testutil.WriteFile(t, subdirPath, "config.json.bak") // Should be excluded
		testutil.WriteFile(t, subdirPath, "old.log.old")     // Should be excluded
	}

	// Use pattern matching multiple directories and extensions with exclusions
	// This mirrors: /var/log/{nginx,apache,caddy}/*.{log,txt,json} excluding *.{gz,zip,bak,old}
	includePath := path.Join(dir, "{nginx,apache,caddy}", "*.{log,txt,json}")
	excludePath := path.Join(dir, "{nginx,apache,caddy}", "*.{gz,zip,bak,old}")

	c := testCreateComponent(t, dir, []string{includePath}, []string{excludePath})
	c.args.SyncPeriod = 10 * time.Millisecond
	err = c.Update(c.args)
	require.NoError(t, err)

	foundFiles := c.getWatchedFiles()

	// Expected files: 3 directories × 3 extensions
	expectedFiles := []string{
		path.Join("nginx", "access.log"),
		path.Join("nginx", "error.txt"),
		path.Join("nginx", "config.json"),
		path.Join("apache", "access.log"),
		path.Join("apache", "error.txt"),
		path.Join("apache", "config.json"),
		path.Join("caddy", "access.log"),
		path.Join("caddy", "error.txt"),
		path.Join("caddy", "config.json"),
	}

	require.Len(t, foundFiles, len(expectedFiles),
		"Expected %d files: %v", len(expectedFiles), expectedFiles)

	// Verify all expected files are matched
	for _, expected := range expectedFiles {
		require.True(t, testutil.ContainsPath(foundFiles, expected),
			"%s should be matched", expected)
	}

	// Verify files from "other" directory are NOT matched (wrong directory)
	require.False(t, testutil.ContainsPath(foundFiles, path.Join("other", "access.log")),
		"other/access.log should not be matched (wrong directory)")

	// Verify excluded extensions are NOT matched
	require.False(t, testutil.ContainsPath(foundFiles, "access.log.gz"), ".gz files should be excluded")
	require.False(t, testutil.ContainsPath(foundFiles, "error.txt.zip"), ".zip files should be excluded")
	require.False(t, testutil.ContainsPath(foundFiles, "config.json.bak"), ".bak files should be excluded")
	require.False(t, testutil.ContainsPath(foundFiles, "old.log.old"), ".old files should be excluded")

	// Verify wrong extensions are NOT matched
	require.False(t, testutil.ContainsPath(foundFiles, "debug.yaml"), ".yaml files should not be matched")
}

func TestMultiLabels(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t3")
	os.MkdirAll(dir, 0755)
	testutil.WriteFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := testCreateComponentWithLabels(t, dir, []string{path.Join(dir, "**", "*.txt"), path.Join(dir, "**", "*.txt")}, nil, map[string]string{
		"foo":   "bar",
		"fruit": "apple",
	})
	tb := discovery.NewTargetBuilderFrom(c.args.PathTargets[0])
	tb.Set("newlabel", "test")
	c.args.PathTargets[0] = tb.Target()
	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 40*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		f := c.getWatchedFiles()
		assert.Len(collect, f, 2)
		assert.True(collect, testutil.ContainsPath([]discovery.Target{f[0]}, "t1.txt"))
		assert.True(collect, testutil.ContainsPath([]discovery.Target{f[1]}, "t1.txt"))
	}, 10*time.Second, 300*time.Millisecond)
}
