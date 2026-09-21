package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

const testPerm os.FileMode = 0600

// requirePerm asserts the permissions of a file. Windows only records whether
// a file is read only, so the check is meaningless there.
func requirePerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	if runtime.GOOS == "windows" {
		return
	}

	fi, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, want, fi.Mode().Perm())
}

func TestWriteCreatesFile(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "state")

	require.NoError(t, Write(dest, []byte("hello"), testPerm))

	contents, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, "hello", string(contents))
	requirePerm(t, dest, testPerm)
}

func TestWriteCreatesEmptyFile(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "state")

	require.NoError(t, Write(dest, nil, testPerm))

	contents, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Empty(t, contents)
}

// Replacing a longer file with a shorter one must not leave any of the old
// contents in place.
func TestWriteReplacesLongerContents(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "state")

	require.NoError(t, Write(dest, []byte("a much longer set of contents"), testPerm))
	require.NoError(t, Write(dest, []byte("short"), testPerm))

	contents, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, "short", string(contents))
}

func TestWriteKeepsExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not record the permissions this asserts on")
	}

	dest := filepath.Join(t.TempDir(), "state")
	require.NoError(t, os.WriteFile(dest, []byte("original"), 0640))

	require.NoError(t, Write(dest, []byte("replaced"), testPerm))

	requirePerm(t, dest, 0640)
}

// The temporary file has to be named after the destination. A name picked at
// random is what this package exists to avoid, so the test pins the path by
// putting a directory in its way: an implementation that used a random name
// would not touch it and the write would succeed.
func TestWriteUsesTempPathDerivedFromDestination(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "state")
	require.NoError(t, os.WriteFile(dest, []byte("original"), testPerm))

	require.NoError(t, os.Mkdir(TempPath(dest), 0700))

	err := Write(dest, []byte("replaced"), testPerm)
	require.Error(t, err)
	require.ErrorContains(t, err, TempPath(dest))

	contents, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, "original", string(contents), "a failed write must leave the previous contents in place")
}

func TestWriteLeavesNoTempFileBehind(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "state")

	for i := 0; i < 5; i++ {
		require.NoError(t, Write(dest, []byte("contents"), testPerm))
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, filepath.Base(dest), entries[0].Name())
}

// A crash between creating the temporary file and moving it leaves the file
// behind. Because its name is reused rather than unique, the next write has to
// cope with it, including the permissions it was created with.
func TestWriteReplacesStaleTempFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "state")

	require.NoError(t, os.WriteFile(TempPath(dest), []byte("contents of an interrupted write"), 0666))

	require.NoError(t, Write(dest, []byte("replaced"), testPerm))

	contents, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, "replaced", string(contents))
	requirePerm(t, dest, testPerm)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestWriteFailsWhenDirectoryIsMissing(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "missing", "state")

	require.Error(t, Write(dest, []byte("contents"), testPerm))
}

func TestTempPathIsStable(t *testing.T) {
	require.Equal(t, TempPath("/var/lib/alloy/state"), TempPath("/var/lib/alloy/state"))
	require.NotEqual(t, TempPath("/var/lib/alloy/state"), TempPath("/var/lib/alloy/other"))
	require.Equal(t, filepath.Clean("/var/lib/alloy/state")+".tmp", TempPath("/var/lib/alloy//state"))
}
