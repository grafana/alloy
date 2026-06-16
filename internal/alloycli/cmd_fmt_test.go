package alloycli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAlloyFmt_NoWriteIfUnchanged(t *testing.T) {
	// 1. Create a temporary file with already-formatted content.
	content := []byte("logging {\n\tlevel  = \"debug\"\n\tformat = \"logfmt\"\n}\n")
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.alloy")
	err := os.WriteFile(tmpFile, content, 0600)
	require.NoError(t, err)

	// 2. Set the file to read-only so that any attempt to open it with O_WRONLY/O_TRUNC will fail.
	err = os.Chmod(tmpFile, 0400)
	require.NoError(t, err)
	defer func() {
		// Restore write permissions so cleanup doesn't fail.
		_ = os.Chmod(tmpFile, 0600)
	}()

	// 3. Run alloyFmt with write: true. Since the file is already formatted,
	// it should NOT attempt to open/truncate the file, and should succeed.
	f := &alloyFmt{
		write: true,
		test:  false,
	}
	err = f.Run(tmpFile)
	require.NoError(t, err)
}

func TestAlloyFmt_WriteIfChanged(t *testing.T) {
	// 1. Create a temporary file with unformatted content.
	content := []byte("logging {\nlevel = \"debug\"\n}\n")
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.alloy")
	err := os.WriteFile(tmpFile, content, 0600)
	require.NoError(t, err)

	// 2. Run alloyFmt with write: true. It should format and overwrite the file.
	f := &alloyFmt{
		write: true,
		test:  false,
	}
	err = f.Run(tmpFile)
	require.NoError(t, err)

	// 3. Verify the file content was formatted.
	newContent, err := os.ReadFile(tmpFile)
	require.NoError(t, err)
	expected := "logging {\n\tlevel = \"debug\"\n}\n"
	require.Equal(t, expected, string(newContent))
}

func TestAlloyFmt_TestFlag(t *testing.T) {
	// Test file with unformatted content
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.alloy")
	content := []byte("logging {\nlevel = \"debug\"\n}\n")
	err := os.WriteFile(tmpFile, content, 0600)
	require.NoError(t, err)

	// Run alloyFmt with test: true. It should return an error indicating changes would be made.
	f := &alloyFmt{
		write: false,
		test:  true,
	}
	err = f.Run(tmpFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "is not formatted correctly")

	// Now run it on formatted content. It should succeed without error.
	formattedContent := []byte("logging {\n\tlevel = \"debug\"\n}\n")
	err = os.WriteFile(tmpFile, formattedContent, 0600)
	require.NoError(t, err)

	err = f.Run(tmpFile)
	require.NoError(t, err)
}
