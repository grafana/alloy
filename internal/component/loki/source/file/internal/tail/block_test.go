package tail

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBlockUntilEvent(t *testing.T) {
	watcherConfig := WatcherConfig{
		MinPollFrequency: 5 * time.Millisecond,
		MaxPollFrequency: 5 * time.Millisecond,
	}

	t.Run("should return modified event when file is written to", func(t *testing.T) {
		f := createEmptyFile(t, "startempty")
		defer f.Close()

		go func() {
			time.Sleep(50 * time.Millisecond)
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
			time.Sleep(50 * time.Millisecond)
			require.NoError(t, os.Chtimes(f.Name(), time.Now(), time.Now()))
		}()

		event, err := blockUntilEvent(context.Background(), f, 0, &Config{
			Filename:      f.Name(),
			WatcherConfig: watcherConfig,
		})
		require.NoError(t, err)
		require.Equal(t, eventModified, event)
	})

	t.Run("should return deleted event if file is deleted", func(t *testing.T) {
		f := createEmptyFile(t, "startempty")
		defer f.Close()

		go func() {
			time.Sleep(50 * time.Millisecond)
			removeFile(t, f.Name())
		}()

		event, err := blockUntilEvent(context.Background(), f, 0, &Config{
			Filename:      f.Name(),
			WatcherConfig: watcherConfig,
		})
		require.NoError(t, err)
		require.Equal(t, eventDeleted, event)
	})

	t.Run("should return deleted event if file is deleted before", func(t *testing.T) {
		f := createEmptyFile(t, "startempty")
		defer f.Close()

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
			time.Sleep(50 * time.Millisecond)
			err := f.Truncate(0)
			fmt.Println(err)
		}()

		event, err := blockUntilEvent(context.Background(), f, offset, &Config{
			Filename:      f.Name(),
			WatcherConfig: watcherConfig,
		})
		require.NoError(t, err)
		require.Equal(t, eventTruncated, event)
	})
}
