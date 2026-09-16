package savepoint

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrateLegacyMarker(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	markerPath := func(dir string) string {
		return filepath.Join(dir, markerFolderName, markerFileName)
	}

	savepointPath := func(dir string) string {
		return filepath.Join(dir, folderName, fileName)
	}

	t.Run("no marker file leaves savepoints empty", func(t *testing.T) {
		dir := t.TempDir()

		require.NoError(t, MigrateLegacyMarker(dir, []string{"endpoint-a"}))

		require.NoFileExists(t, savepointPath(dir))

		f, err := NewFile(logger, dir)
		require.NoError(t, err)
		require.Equal(t, noSegment, f.LastStoredSegment("endpoint-a"))
	})

	t.Run("seeds every endpoint from the marker", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, markerFolderName), folderMode))
		require.NoError(t, os.WriteFile(markerPath(dir), encodeMarker(10), 0o600))

		require.NoError(t, MigrateLegacyMarker(dir, []string{"endpoint-a", "endpoint-b"}))

		f, err := NewFile(logger, dir)
		require.NoError(t, err)
		require.Equal(t, 10, f.LastStoredSegment("endpoint-a"))
		require.Equal(t, 10, f.LastStoredSegment("endpoint-b"))

		// the marker is left in place so that a rollback to an Alloy without
		// savepoints still finds it
		require.FileExists(t, markerPath(dir))
	})

	t.Run("does nothing when a savepoint file already exists", func(t *testing.T) {
		dir := t.TempDir()
		f, err := NewFile(logger, dir)
		require.NoError(t, err)
		f.StoreSegment("endpoint-a", 5)

		require.NoError(t, os.WriteFile(markerPath(dir), encodeMarker(99), 0o600))

		require.NoError(t, MigrateLegacyMarker(dir, []string{"endpoint-a", "endpoint-b"}))

		reloaded, err := NewFile(logger, dir)
		require.NoError(t, err)
		require.Equal(t, 5, reloaded.LastStoredSegment("endpoint-a"))
		require.Equal(t, noSegment, reloaded.LastStoredSegment("endpoint-b"))
	})

	t.Run("a corrupt marker is reported and leaves savepoints empty", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, markerFolderName), folderMode))
		require.NoError(t, os.WriteFile(markerPath(dir), []byte("not a marker"), 0o600))

		require.Error(t, MigrateLegacyMarker(dir, []string{"endpoint-a"}))

		require.NoFileExists(t, savepointPath(dir))

		f, err := NewFile(logger, dir)
		require.NoError(t, err)
		require.Equal(t, noSegment, f.LastStoredSegment("endpoint-a"))
	})

	t.Run("no endpoints means no savepoint file", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, markerFolderName), folderMode))
		require.NoError(t, os.WriteFile(markerPath(dir), encodeMarker(10), 0o600))

		require.NoError(t, MigrateLegacyMarker(dir, nil))

		require.NoFileExists(t, savepointPath(dir))
	})
}
