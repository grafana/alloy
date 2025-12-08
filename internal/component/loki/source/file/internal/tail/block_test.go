package tail

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tail/fileext"
	"github.com/stretchr/testify/require"
)

func TestBlockUntilExists(t *testing.T) {
	watcherConfig := WatcherConfig{
		MinPollFrequency: 5 * time.Millisecond,
		MaxPollFrequency: 5 * time.Millisecond,
	}

	t.Run("should block until file exists", func(t *testing.T) {
		filename := filepath.Join(t.TempDir(), "eventually")

		go func() {
			time.Sleep(10 * time.Millisecond)
			createFileWithPath(t, filename, "")
		}()

		err := blockUntilExists(context.Background(), &Config{
			Filename:      filename,
			WatcherConfig: watcherConfig,
		})
		require.NoError(t, err)
	})

	t.Run("should exit when context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		err := blockUntilExists(ctx, &Config{
			Filename:      filepath.Join(t.TempDir(), "never"),
			WatcherConfig: watcherConfig,
		})
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestBlockUntilEvent(t *testing.T) {
	watcherConfig := WatcherConfig{
		MinPollFrequency: 5 * time.Millisecond,
		MaxPollFrequency: 5 * time.Millisecond,
	}

	t.Run("should return modified event when file is written to", func(t *testing.T) {
		f := createEmptyFile(t, "startempty")
		defer f.Close()

		go func() {
			time.Sleep(10 * time.Millisecond)
			_, err := f.WriteString("updated")
			require.NoError(t, err)
		}()

		event, err := blockUntilEvent(context.Background(), f, 0, &Config{
			Filename:      f.Name(),
			WatcherConfig: watcherConfig,
		})
		require.NoError(t, err)
		require.Equal(t, eventModified, event)
	})

	t.Run("should return modified event if mod time is updated", func(t *testing.T) {
		f := createEmptyFile(t, "startempty")
		defer f.Close()

		go func() {
			time.Sleep(10 * time.Millisecond)
			require.NoError(t, os.Chtimes(f.Name(), time.Now(), time.Now()))
		}()

		event, err := blockUntilEvent(context.Background(), f, 0, &Config{
			Filename:      f.Name(),
			WatcherConfig: watcherConfig,
		})
		require.NoError(t, err)
		require.Equal(t, eventModified, event)
	})

	/*
		t.Run("should return deleted event if file is deleted", func(t *testing.T) {
			f := createEmptyFile(t, "startempty")
			defer f.Close()

			go func() {
				time.Sleep(10 * time.Millisecond)
				removeFile(t, f.Name())
			}()

			event, err := blockUntilEvent(context.Background(), f, 0, &Config{
				Filename:      f.Name(),
				WatcherConfig: watcherConfig,
			})
			require.NoError(t, err)
			require.Equal(t, eventDeleted, event)
		})
	*/

	t.Run("should return deleted event if file is deleted before", func(t *testing.T) {
		f := createEmptyFile(t, "startempty")
		require.NoError(t, f.Close())

		// NOTE: important for windows that we open with correct flags.
		f, err := fileext.OpenFile(f.Name())
		require.NoError(t, err)

		removeFile(t, f.Name())

		event, err := blockUntilEvent(context.Background(), f, 0, &Config{
			Filename:      f.Name(),
			WatcherConfig: watcherConfig,
		})
		require.NoError(t, err)
		require.Equal(t, eventDeleted, event)
	})

	t.Run("should return truncated event", func(t *testing.T) {
		f := createFileWithContent(t, "truncate", "content")
		defer f.Close()

		offset, err := f.Seek(0, io.SeekEnd)
		require.NoError(t, err)

		go func() {
			time.Sleep(10 * time.Millisecond)
			require.NoError(t, f.Truncate(0))
		}()

		event, err := blockUntilEvent(context.Background(), f, offset, &Config{
			Filename:      f.Name(),
			WatcherConfig: watcherConfig,
		})
		require.NoError(t, err)
		require.Equal(t, eventTruncated, event)
	})

	t.Run("should exit when context is canceled", func(t *testing.T) {
		f := createEmptyFile(t, "startempty")
		defer f.Close()

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		event, err := blockUntilEvent(ctx, f, 0, &Config{
			Filename:      f.Name(),
			WatcherConfig: watcherConfig,
		})
		require.ErrorIs(t, err, context.Canceled)
		require.Equal(t, eventNone, event)
	})
}

func createEmptyFile(t *testing.T, name string) *os.File {
	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	require.NoError(t, err)
	return f
}

func createFileWithContent(t *testing.T, name, content string) *os.File {
	path := createFile(t, name, content)
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	return f
}

func createFileWithPath(t *testing.T, path, content string) {
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
}
