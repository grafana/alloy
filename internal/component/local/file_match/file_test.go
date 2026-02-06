package file_match

import (
	"context"
	"os"
	"path"
	"testing"
	"time"

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
	time.Sleep(20 * time.Millisecond)
	ct.Done()
	foundFiles := c.getWatchedFiles()
	require.Len(t, foundFiles, 1)
	require.True(t, testutil.ContainsPath(foundFiles, "t1.txt"))
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
	time.Sleep(20 * time.Millisecond)
	ct.Done()
	foundFiles := c.getWatchedFiles()
	require.Len(t, foundFiles, 1)
	require.True(t, testutil.ContainsPath(foundFiles, "t1.txt"))
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

	foundFiles := c.getWatchedFiles()
	require.Len(t, foundFiles, 1)
	require.True(t, testutil.ContainsPath(foundFiles, "t1.txt"))
	time.Sleep(150 * time.Millisecond)

	testutil.WriteFile(t, dir, "t2.txt")
	ct.Done()
	foundFiles = c.getWatchedFiles()
	require.Len(t, foundFiles, 1)
	require.True(t, testutil.ContainsPath(foundFiles, "t2.txt"))
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
	time.Sleep(20 * time.Millisecond)
	testutil.WriteFile(t, dir, "t2.txt")
	ct.Done()
	foundFiles := c.getWatchedFiles()
	require.Len(t, foundFiles, 2)
	require.True(t, testutil.ContainsPath(foundFiles, "t1.txt"))
	require.True(t, testutil.ContainsPath(foundFiles, "t2.txt"))
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
	time.Sleep(20 * time.Millisecond)
	testutil.WriteFile(t, dir, "t2.txt")
	subdir := path.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	time.Sleep(20 * time.Millisecond)
	err := os.WriteFile(path.Join(subdir, "t3.txt"), []byte("asdf"), 0664)
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
	ct.Done()
	foundFiles := c.getWatchedFiles()
	require.Len(t, foundFiles, 3)
	require.True(t, testutil.ContainsPath(foundFiles, "t1.txt"))
	require.True(t, testutil.ContainsPath(foundFiles, "t2.txt"))
	require.True(t, testutil.ContainsPath(foundFiles, "t3.txt"))
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
	time.Sleep(20 * time.Millisecond)
	testutil.WriteFile(t, dir, "t2.txt")
	subdir := path.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	subdir2 := path.Join(dir, "subdir2")
	os.Mkdir(subdir2, 0755)
	time.Sleep(20 * time.Millisecond)
	// This file will not be included, since it is in the excluded subdir
	err := os.WriteFile(path.Join(subdir, "exclude_me.txt"), []byte("asdf"), 0664)
	require.NoError(t, err)
	// This file will be included, since it is in another subdir
	err = os.WriteFile(path.Join(subdir2, "another.txt"), []byte("asdf"), 0664)
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
	ct.Done()
	foundFiles := c.getWatchedFiles()
	require.Len(t, foundFiles, 3)
	require.True(t, testutil.ContainsPath(foundFiles, "t1.txt"))
	require.True(t, testutil.ContainsPath(foundFiles, "t2.txt"))
	require.True(t, testutil.ContainsPath(foundFiles, "another.txt"))
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
	time.Sleep(20 * time.Millisecond)
	testutil.WriteFile(t, dir, "t2.txt")
	subdir := path.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	time.Sleep(100 * time.Millisecond)
	err := os.WriteFile(path.Join(subdir, "t3.txt"), []byte("asdf"), 0664)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	foundFiles := c.getWatchedFiles()
	require.Len(t, foundFiles, 3)
	require.True(t, testutil.ContainsPath(foundFiles, "t1.txt"))
	require.True(t, testutil.ContainsPath(foundFiles, "t2.txt"))
	require.True(t, testutil.ContainsPath(foundFiles, "t3.txt"))

	err = os.RemoveAll(subdir)
	require.NoError(t, err)
	time.Sleep(1000 * time.Millisecond)
	foundFiles = c.getWatchedFiles()
	require.Len(t, foundFiles, 2)
	require.True(t, testutil.ContainsPath(foundFiles, "t1.txt"))
	require.True(t, testutil.ContainsPath(foundFiles, "t2.txt"))
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
	time.Sleep(100 * time.Millisecond)
	subdir := path.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	testutil.WriteFile(t, subdir, "t3.txt")
	time.Sleep(100 * time.Millisecond)
	foundFiles := c.getWatchedFiles()
	require.Len(t, foundFiles, 2)
	require.True(t, testutil.ContainsPath(foundFiles, "t1.txt"))
	require.True(t, testutil.ContainsPath(foundFiles, "t3.txt"))
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
	time.Sleep(100 * time.Millisecond)
	foundFiles := c.getWatchedFiles()
	require.Len(t, foundFiles, 2)
	require.True(t, testutil.ContainsPath([]discovery.Target{foundFiles[0]}, "t1.txt"))
	require.True(t, testutil.ContainsPath([]discovery.Target{foundFiles[1]}, "t1.txt"))
}
