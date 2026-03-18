//go:build windows

package fileext

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenFile_NonExistentReturnsPathError(t *testing.T) {
	name := filepath.Join(t.TempDir(), "nonexistent.log")

	_, err := OpenFile(name)
	require.Error(t, err)

	var pathErr *os.PathError
	require.ErrorAs(t, err, &pathErr, "error should be wrapped in *os.PathError")
	assert.Equal(t, "open", pathErr.Op)
	assert.Equal(t, name, pathErr.Path)
	assert.True(t, os.IsNotExist(err), "os.IsNotExist should still return true")
}
