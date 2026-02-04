package tail

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

func TestLastNewline(t *testing.T) {
	encoder := encoding.Nop.NewEncoder()
	t.Run("empty file", func(t *testing.T) {
		lastNewlineTest(t, "empty", encoder, "", 0)
	})

	t.Run("UTF-8 no newline", func(t *testing.T) {
		lastNewlineTest(t, "no-nl", encoder, "line1", 0)
	})

	t.Run("UTF-8 single newline at end", func(t *testing.T) {
		lastNewlineTest(t, "end", encoder, "line1\n", 6)
	})

	t.Run("UTF-8 newline in middle", func(t *testing.T) {
		lastNewlineTest(t, "middle", encoder, "line1\nline2", 6)
	})

	t.Run("UTF-8 last", func(t *testing.T) {
		lastNewlineTest(t, "last", encoder, "line1\nline2\nline3\n", 18)
	})

	encoder = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	t.Run("UTF-16LE empty", func(t *testing.T) {
		lastNewlineTest(t, "empty", encoder, "", 0)
	})

	t.Run("UTF-16LE no newline", func(t *testing.T) {
		lastNewlineTest(t, "no-nl", encoder, "line1", 0)
	})

	t.Run("UTF-16LE single newline at end", func(t *testing.T) {
		lastNewlineTest(t, "end", encoder, "line1\n", 12)
	})

	t.Run("UTF-16LE newline in middle", func(t *testing.T) {
		lastNewlineTest(t, "middle", encoder, "line1\nline2", 12)
	})

	t.Run("UTF-16LE last", func(t *testing.T) {
		lastNewlineTest(t, "last", encoder, "line1\nline2\nline3\n", 36)
	})

	encoder = unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewEncoder()
	t.Run("UTF-16BE empty", func(t *testing.T) {
		lastNewlineTest(t, "empty", encoder, "", 0)
	})

	t.Run("UTF-16BE no newline", func(t *testing.T) {
		lastNewlineTest(t, "no-nl", encoder, "line1", 0)
	})

	t.Run("UTF-16BE single newline at end", func(t *testing.T) {
		lastNewlineTest(t, "end", encoder, "line1\n", 12)
	})

	t.Run("UTF-16BE newline in middle", func(t *testing.T) {
		lastNewlineTest(t, "middle", encoder, "line1\nline2", 12)
	})

	t.Run("UTF-16BE last", func(t *testing.T) {
		lastNewlineTest(t, "last", encoder, "line1\nline2\nline3\n", 36)
	})
}

func lastNewlineTest(t *testing.T, name string, encoder *encoding.Encoder, content string, expectedPos int64) {
	encoded, err := encoder.String(content)
	require.NoError(t, err)

	f := createFileWithContent(t, name, encoded)
	defer os.Remove(f.Name())
	defer f.Close()

	nl, err := encodedNewline(encoder)
	require.NoError(t, err)

	got, err := lastNewline(f, nl)
	require.NoError(t, err)
	require.Equal(t, expectedPos, got)
}

// createTempFile creates a temp file with content and returns the open file (read-only seekable).
func createTempFile(t *testing.T, content []byte) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "seektest")
	require.NoError(t, os.WriteFile(path, content, 0600))
	f, err := os.Open(path)
	require.NoError(t, err)
	return f
}
