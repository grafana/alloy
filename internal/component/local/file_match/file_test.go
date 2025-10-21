//go:build !windows

// This should run on windows but windows does not like the tight timing of file creation and deletion.
package file_match

import (
	"context"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/discovery"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/util"
)

func TestFile(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t1")
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	writeFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := createComponent(t, dir, []string{path.Join(dir, "*.txt")}, nil)
	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 5*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	time.Sleep(20 * time.Millisecond)
	ct.Done()
	foundFiles := c.getWatchedFiles(false)
	require.Len(t, foundFiles, 1)
	require.True(t, contains(foundFiles, "t1.txt"))
}

func TestDirectoryFile(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t1")
	subdir := path.Join(dir, "subdir")
	err := os.MkdirAll(subdir, 0755)
	require.NoError(t, err)
	writeFile(t, subdir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := createComponent(t, dir, []string{path.Join(dir, "**/")}, nil)
	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 5*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	time.Sleep(20 * time.Millisecond)
	ct.Done()
	foundFiles := c.getWatchedFiles(false)
	require.Len(t, foundFiles, 1)
	require.True(t, contains(foundFiles, "t1.txt"))
}

func TestFileIgnoreOlder(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t1")
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	writeFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := createComponent(t, dir, []string{path.Join(dir, "*.txt")}, nil)
	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 5*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	c.args.IgnoreOlderThan = 100 * time.Millisecond
	c.Update(c.args)
	go c.Run(ct)

	foundFiles := c.getWatchedFiles(false)
	require.Len(t, foundFiles, 1)
	require.True(t, contains(foundFiles, "t1.txt"))
	time.Sleep(150 * time.Millisecond)

	writeFile(t, dir, "t2.txt")
	ct.Done()
	foundFiles = c.getWatchedFiles(false)
	require.Len(t, foundFiles, 1)
	require.True(t, contains(foundFiles, "t2.txt"))
}

func TestAddingFile(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t2")
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	writeFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := createComponent(t, dir, []string{path.Join(dir, "*.txt")}, nil)

	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 40*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	time.Sleep(20 * time.Millisecond)
	writeFile(t, dir, "t2.txt")
	ct.Done()
	foundFiles := c.getWatchedFiles(false)
	require.Len(t, foundFiles, 2)
	require.True(t, contains(foundFiles, "t1.txt"))
	require.True(t, contains(foundFiles, "t2.txt"))
}

func TestAddingFileInSubDir(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t3")
	os.MkdirAll(dir, 0755)
	writeFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := createComponent(t, dir, []string{path.Join(dir, "**", "*.txt")}, nil)
	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 40*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	time.Sleep(20 * time.Millisecond)
	writeFile(t, dir, "t2.txt")
	subdir := path.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	time.Sleep(20 * time.Millisecond)
	err := os.WriteFile(path.Join(subdir, "t3.txt"), []byte("asdf"), 0664)
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
	ct.Done()
	foundFiles := c.getWatchedFiles(false)
	require.Len(t, foundFiles, 3)
	require.True(t, contains(foundFiles, "t1.txt"))
	require.True(t, contains(foundFiles, "t2.txt"))
	require.True(t, contains(foundFiles, "t3.txt"))
}

func TestAddingFileInAnExcludedSubDir(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t3")
	os.MkdirAll(dir, 0755)
	writeFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	included := []string{path.Join(dir, "**", "*.txt")}
	excluded := []string{path.Join(dir, "subdir", "*.txt")}
	c := createComponent(t, dir, included, excluded)
	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 40*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	time.Sleep(20 * time.Millisecond)
	writeFile(t, dir, "t2.txt")
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
	foundFiles := c.getWatchedFiles(false)
	require.Len(t, foundFiles, 3)
	require.True(t, contains(foundFiles, "t1.txt"))
	require.True(t, contains(foundFiles, "t2.txt"))
	require.True(t, contains(foundFiles, "another.txt"))
}

func TestAddingRemovingFileInSubDir(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t3")
	os.MkdirAll(dir, 0755)
	writeFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := createComponent(t, dir, []string{path.Join(dir, "**", "*.txt")}, nil)

	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 40*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	time.Sleep(20 * time.Millisecond)
	writeFile(t, dir, "t2.txt")
	subdir := path.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	time.Sleep(100 * time.Millisecond)
	err := os.WriteFile(path.Join(subdir, "t3.txt"), []byte("asdf"), 0664)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	foundFiles := c.getWatchedFiles(false)
	require.Len(t, foundFiles, 3)
	require.True(t, contains(foundFiles, "t1.txt"))
	require.True(t, contains(foundFiles, "t2.txt"))
	require.True(t, contains(foundFiles, "t3.txt"))

	err = os.RemoveAll(subdir)
	require.NoError(t, err)
	time.Sleep(1000 * time.Millisecond)
	foundFiles = c.getWatchedFiles(false)
	require.Len(t, foundFiles, 2)
	require.True(t, contains(foundFiles, "t1.txt"))
	require.True(t, contains(foundFiles, "t2.txt"))
}

func TestExclude(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t3")
	os.MkdirAll(dir, 0755)
	writeFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := createComponent(t, dir, []string{path.Join(dir, "**", "*.txt")}, []string{path.Join(dir, "**", "*.bad")})
	ct := t.Context()
	ct, ccl := context.WithTimeout(ct, 40*time.Second)
	defer ccl()
	c.args.SyncPeriod = 10 * time.Millisecond
	go c.Run(ct)
	time.Sleep(100 * time.Millisecond)
	subdir := path.Join(dir, "subdir")
	os.Mkdir(subdir, 0755)
	writeFile(t, subdir, "t3.txt")
	time.Sleep(100 * time.Millisecond)
	foundFiles := c.getWatchedFiles(false)
	require.Len(t, foundFiles, 2)
	require.True(t, contains(foundFiles, "t1.txt"))
	require.True(t, contains(foundFiles, "t3.txt"))
}

func TestMultiLabels(t *testing.T) {
	dir := path.Join(os.TempDir(), "alloy_testing", "t3")
	os.MkdirAll(dir, 0755)
	writeFile(t, dir, "t1.txt")
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	c := createComponentWithLabels(t, dir, []string{path.Join(dir, "**", "*.txt"), path.Join(dir, "**", "*.txt")}, nil, map[string]string{
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
	foundFiles := c.getWatchedFiles(false)
	require.Len(t, foundFiles, 2)
	require.True(t, contains([]discovery.Target{foundFiles[0]}, "t1.txt"))
	require.True(t, contains([]discovery.Target{foundFiles[1]}, "t1.txt"))
}

// createComponent creates a component with the given paths and labels. The paths and excluded slices are zipped together
// to create the set of targets to pass to the component.
func createComponent(t *testing.T, dir string, paths []string, excluded []string) *Component {
	return createComponentWithLabels(t, dir, paths, excluded, nil)
}

// createComponentWithLabels creates a component with the given paths and labels. The paths and excluded slices are
// zipped together to create the set of targets to pass to the component.
func createComponentWithLabels(t *testing.T, dir string, paths []string, excluded []string, labels map[string]string) *Component {
	tPaths := make([]discovery.Target, 0)
	for i, p := range paths {
		tb := discovery.NewTargetBuilder()
		tb.Set("__path__", p)
		for k, v := range labels {
			tb.Set(k, v)
		}
		if i < len(excluded) {
			tb.Set("__path_exclude__", excluded[i])
		}
		tPaths = append(tPaths, tb.Target())
	}
	c, err := New(component.Options{
		ID:       "test",
		Logger:   util.TestAlloyLogger(t),
		DataPath: dir,
		OnStateChange: func(e component.Exports) {

		},
		Registerer: prometheus.DefaultRegisterer,
		Tracer:     nil,
	}, Arguments{
		PathTargets: tPaths,
		SyncPeriod:  1 * time.Second,
	})

	require.NoError(t, err)
	require.NotNil(t, c)
	return c
}

func contains(sources []discovery.Target, match string) bool {
	for _, s := range sources {
		p, _ := s.Get("__path__")
		if strings.Contains(p, match) {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, dir string, name string) {
	err := os.WriteFile(path.Join(dir, name), []byte("asdf"), 0664)
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
}
