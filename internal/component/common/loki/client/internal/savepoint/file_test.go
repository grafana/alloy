package savepoint

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFile(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	savepointPath := func(dir string) string {
		return filepath.Join(dir, folderName, fileName)
	}

	t.Run("no saved segment when there is no savepoint file", func(t *testing.T) {
		f, err := NewFile(logger, t.TempDir())
		require.NoError(t, err)

		require.Equal(t, noSegment, f.LastStoredSegment("endpoint-a"))
	})

	t.Run("no file is written until a segment is stored", func(t *testing.T) {
		dir := t.TempDir()
		f, err := NewFile(logger, dir)
		require.NoError(t, err)
		require.NoFileExists(t, savepointPath(dir))

		f.StoreSegment("endpoint-a", 3)
		require.FileExists(t, savepointPath(dir))
	})

	t.Run("stores a segment and reads it back", func(t *testing.T) {
		f, err := NewFile(logger, t.TempDir())
		require.NoError(t, err)

		f.StoreSegment("endpoint-a", 12)
		require.Equal(t, 12, f.LastStoredSegment("endpoint-a"))
	})

	t.Run("stored segments survive a reload", func(t *testing.T) {
		dir := t.TempDir()

		f, err := NewFile(logger, dir)
		require.NoError(t, err)
		f.StoreSegment("endpoint-a", 7)

		reloaded, err := NewFile(logger, dir)
		require.NoError(t, err)
		require.Equal(t, 7, reloaded.LastStoredSegment("endpoint-a"))
	})

	t.Run("endpoints do not overwrite each other", func(t *testing.T) {
		dir := t.TempDir()

		f, err := NewFile(logger, dir)
		require.NoError(t, err)

		f.StoreSegment("endpoint-a", 4)
		f.StoreSegment("endpoint-b", 9)
		f.StoreSegment("endpoint-a", 5)

		require.Equal(t, 5, f.LastStoredSegment("endpoint-a"))
		require.Equal(t, 9, f.LastStoredSegment("endpoint-b"))

		reloaded, err := NewFile(logger, dir)
		require.NoError(t, err)

		require.Equal(t, 5, reloaded.LastStoredSegment("endpoint-a"))
		require.Equal(t, 9, reloaded.LastStoredSegment("endpoint-b"))
	})

	t.Run("no saved segment for an endpoint that was never stored", func(t *testing.T) {
		dir := t.TempDir()
		f, err := NewFile(logger, dir)
		require.NoError(t, err)

		f.StoreSegment("endpoint-a", 4)

		require.Equal(t, noSegment, f.LastStoredSegment("endpoint-b"))
	})

	t.Run("a corrupt savepoint file reports no saved segments", func(t *testing.T) {
		dir := t.TempDir()
		f, err := NewFile(logger, dir)
		require.NoError(t, err)

		f.StoreSegment("endpoint-a", 10)
		require.NoError(t, os.WriteFile(savepointPath(dir), []byte("not json"), 0o600))

		reloaded, err := NewFile(logger, dir)
		require.NoError(t, err)

		require.Equal(t, noSegment, reloaded.LastStoredSegment("endpoint-a"))
	})

	t.Run("an unsupported version reports no saved segments", func(t *testing.T) {
		dir := t.TempDir()
		f, err := NewFile(logger, dir)
		require.NoError(t, err)

		f.StoreSegment("endpoint-a", 10)
		require.NoError(t, os.WriteFile(savepointPath(dir), []byte(`{"version":9999,"endpoints":{"endpoint-a":{"segment":10}}}`), 0o600))

		reloaded, err := NewFile(logger, dir)
		require.NoError(t, err)

		require.Equal(t, noSegment, reloaded.LastStoredSegment("endpoint-a"))
	})
}
