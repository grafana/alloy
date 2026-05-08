package marker

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func TestFile(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stdout)
	getTempDir := func(t *testing.T) string {
		dir := t.TempDir()
		return dir
	}

	t.Run("invalid last marked segment when there's no marker file", func(t *testing.T) {
		dir := getTempDir(t)
		fh, err := NewFile(logger, dir)
		require.NoError(t, err)

		require.Equal(t, -1, fh.LastMarkedSegment())
	})

	t.Run("reads the last segment from existing marker file", func(t *testing.T) {
		dir := getTempDir(t)
		fh, err := NewFile(logger, dir)
		require.NoError(t, err)

		// write first something to marker
		markerFile := filepath.Join(dir, markerFolderName, markerFileName)
		bs := encodeV1(10)
		err = os.WriteFile(markerFile, bs, markerFileMode)
		require.NoError(t, err)

		require.Equal(t, 10, fh.LastMarkedSegment())
	})

	t.Run("marks segment, and then reads value from it", func(t *testing.T) {
		dir := getTempDir(t)
		fh, err := NewFile(logger, dir)
		require.NoError(t, err)

		fh.MarkSegment(12)
		require.Equal(t, 12, fh.LastMarkedSegment())
	})

	t.Run("marker file and directory is created with correct permissions", func(t *testing.T) {
		dir := getTempDir(t)
		fh, err := NewFile(logger, dir)
		require.NoError(t, err)

		fh.MarkSegment(12)
		// check folder first
		stats, err := os.Stat(filepath.Join(dir, markerFolderName))
		require.NoError(t, err)
		if runtime.GOOS == "windows" {
			require.Equal(t, markerWindowsFolderMode, stats.Mode().Perm())
		} else {
			require.Equal(t, markerFolderMode, stats.Mode().Perm())
		}
		// then file
		stats, err = os.Stat(filepath.Join(dir, markerFolderName, markerFileName))
		require.NoError(t, err)
		if runtime.GOOS == "windows" {
			require.Equal(t, markerWindowsFileMode, stats.Mode().Perm())
		} else {
			require.Equal(t, markerFileMode, stats.Mode().Perm())
		}
	})
}
