package internal

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func TestMarkerFileHandler(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stdout)
	getTempDir := func(t *testing.T) string {
		dir := t.TempDir()
		return dir
	}

	t.Run("invalid last marked segment when there's no marker file", func(t *testing.T) {
		dir := getTempDir(t)
		fh, err := NewMarkerFileHandler(logger, dir)
		require.NoError(t, err)

		require.Equal(t, -1, fh.LastMarkedSegment())
	})

	t.Run("reads the last segment from existing marker file", func(t *testing.T) {
		dir := getTempDir(t)
		fh, err := NewMarkerFileHandler(logger, dir)
		require.NoError(t, err)

		// write first something to marker
		markerFile := filepath.Join(dir, MarkerFolderName, MarkerFileName)
		bs, err := EncodeMarkerV1(10)
		require.NoError(t, err)
		err = os.WriteFile(markerFile, bs, MarkerFileMode)
		require.NoError(t, err)

		require.Equal(t, 10, fh.LastMarkedSegment())
	})

	t.Run("marks segment, and then reads value from it", func(t *testing.T) {
		dir := getTempDir(t)
		fh, err := NewMarkerFileHandler(logger, dir)
		require.NoError(t, err)

		fh.MarkSegment(12)
		require.Equal(t, 12, fh.LastMarkedSegment())
	})

	t.Run("marker file and directory is created with correct permissions", func(t *testing.T) {
		dir := getTempDir(t)
		fh, err := NewMarkerFileHandler(logger, dir)
		require.NoError(t, err)

		fh.MarkSegment(12)
		// check folder first
		stats, err := os.Stat(filepath.Join(dir, MarkerFolderName))
		require.NoError(t, err)
		if runtime.GOOS == "windows" {
			require.Equal(t, MarkerWindowsFolderMode, stats.Mode().Perm())
		} else {
			require.Equal(t, MarkerFolderMode, stats.Mode().Perm())
		}
		// then file
		stats, err = os.Stat(filepath.Join(dir, MarkerFolderName, MarkerFileName))
		require.NoError(t, err)
		if runtime.GOOS == "windows" {
			require.Equal(t, MarkerWindowsFileMode, stats.Mode().Perm())
		} else {
			require.Equal(t, MarkerFileMode, stats.Mode().Perm())
		}
	})
}
