package remotecfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateFileName(t *testing.T) {
	for _, name := range []string{"a.txt", "ca.pem", ".hidden", "with space"} {
		assert.NoError(t, validateFileName(name), name)
	}
	for _, name := range []string{"", ".", "..", "../x", "a/b", `a\b`, "/etc/passwd", tmpFilePrefix + "a"} {
		assert.Error(t, validateFileName(name), name)
	}
}

func TestSyncFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "files")

	// Reading a missing directory returns no files.
	files, err := readFiles(dir)
	require.NoError(t, err)
	require.Empty(t, files)

	require.NoError(t, syncFiles(dir, map[string][]byte{"a": []byte("1"), "b": []byte("2")}))
	files, err = readFiles(dir)
	require.NoError(t, err)
	require.Equal(t, map[string][]byte{"a": []byte("1"), "b": []byte("2")}, files)

	// Stale files and directories are removed, changed files are updated.
	require.NoError(t, os.Mkdir(filepath.Join(dir, "stale-dir"), 0750))
	require.NoError(t, syncFiles(dir, map[string][]byte{"a": []byte("3")}))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	b, err := os.ReadFile(filepath.Join(dir, "a"))
	require.NoError(t, err)
	require.Equal(t, "3", string(b))

	// An invalid name fails before anything is modified.
	require.Error(t, syncFiles(dir, map[string][]byte{"a": []byte("4"), "../b": []byte("x")}))
	b, err = os.ReadFile(filepath.Join(dir, "a"))
	require.NoError(t, err)
	require.Equal(t, "3", string(b))
}

func TestSyncFilesDoesNotFollowSymlinks(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "files")
	require.NoError(t, os.Mkdir(dir, 0750))

	outside := filepath.Join(base, "outside")
	require.NoError(t, os.WriteFile(outside, []byte("original"), 0640))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "link")))

	require.NoError(t, syncFiles(dir, map[string][]byte{"link": []byte("overwritten")}))

	b, err := os.ReadFile(outside)
	require.NoError(t, err)
	require.Equal(t, "original", string(b))
}

func TestGetConfigHash(t *testing.T) {
	content := []byte("cfg")
	require.Equal(t, getHash(content), getConfigHash(content, nil))
	require.Equal(t, getHash(content), getConfigHash(content, map[string][]byte{}))

	h := getConfigHash(content, map[string][]byte{"a": []byte("1")})
	require.NotEqual(t, getHash(content), h)
	require.NotEqual(t, h, getConfigHash(content, map[string][]byte{"a": []byte("2")}))
	require.NotEqual(t, h, getConfigHash(content, map[string][]byte{"b": []byte("1")}))
	// Field boundaries are unambiguous.
	require.NotEqual(t,
		getConfigHash(content, map[string][]byte{"ab": []byte("c")}),
		getConfigHash(content, map[string][]byte{"a": []byte("bc")}))
}
