package tail

import (
	"context"
	"os"
	"runtime"

	"github.com/grafana/dskit/backoff"

	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tail/fileext"
)

// blockUntilExists blocks until the file specified in cfg exists or the context is canceled.
// It polls the file system at intervals defined by WatcherConfig polling frequencies.
// Returns an error if the context is canceled or an unrecoverable error occurs.
func blockUntilExists(ctx context.Context, cfg *Config) error {
	backoff := backoff.New(ctx, backoff.Config{
		MinBackoff: cfg.WatcherConfig.MinPollFrequency,
		MaxBackoff: cfg.WatcherConfig.MaxPollFrequency,
	})

	for backoff.Ongoing() {
		if _, err := os.Stat(cfg.Filename); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		backoff.Wait()
	}

	return backoff.Err()
}

// event represents a file system event detected during polling.
type event int

const (
	eventNone      event = iota // no event detected
	eventTruncated              // file was truncated (size decreased)
	eventModified               // file was modified (size increased or modification time changed)
	eventDeleted                // file was deleted, moved, or renamed
)

// blockUntilEvent blocks until it detects a file system event for the given file or the context is canceled.
// It polls the file system to detect modifications, truncations, deletions, or renames.
// The pos parameter is the current file position and is used to detect truncation events.
// Returns the detected event type and any error encountered. Returns eventNone if the context is canceled.
func blockUntilEvent(ctx context.Context, f *os.File, prevSize int64, cfg *Config) (event, error) {
	// NOTE: it is important that we stat the open file here. Later we do os.Stat(cfg.Filename)
	// and use os.IsSameFile to detect if file was rotated.
	origFi, err := f.Stat()
	if err != nil {
		// If file no longer exists we treat it as a delete event.
		if os.IsNotExist(err) {
			return eventDeleted, nil
		}
		return eventNone, err
	}

	backoff := backoff.New(ctx, backoff.Config{
		MinBackoff: cfg.WatcherConfig.MinPollFrequency,
		MaxBackoff: cfg.WatcherConfig.MaxPollFrequency,
	})

	prevModTime := origFi.ModTime()

	for backoff.Ongoing() {
		deletePending, err := fileext.IsDeletePending(f)

		// DeletePending is a windows state where the file has been queued
		// for delete but won't actually get deleted until all handles are
		// closed. It's a variation on the NotifyDeleted call below.
		//
		// IsDeletePending may fail in cases where the file handle becomes
		// invalid, so we treat a failed call the same as a pending delete.
		if err != nil || deletePending {
			return eventDeleted, nil
		}

		// Check current open file descriptor FIRST before checking the file at the configured path.
		// After rename / file rotation, the path points to a NEW file, but we're still reading from
		// the OLD file (via current open file descriptor). We must read all data from the current
		// old file before detecting rename / rotation, to avoid losing data.
		currentFi, err := f.Stat()
		if err != nil {
			// If we can't stat our open file, treat it as deleted.
			return eventDeleted, nil
		}

		currentSize := currentFi.Size()

		// Check if our open file got truncated
		if prevSize > 0 && prevSize > currentSize {
			return eventTruncated, nil
		}

		// Check if our open file got bigger - this takes priority over rotation detection
		// as we want to read the remaining data from it first.
		if prevSize < currentSize {
			return eventModified, nil
		}

		// Check if our open file was modified (by mod time)
		if currentFi.ModTime() != prevModTime {
			return eventModified, nil
		}

		// Only now, after confirming our file has no new data, check if path was rotated
		fi, err := os.Stat(cfg.Filename)
		if err != nil {
			// Windows cannot delete a file if a handle is still open (tail keeps one open)
			// so it gives access denied to anything trying to read it until all handles are released.
			if os.IsNotExist(err) || (runtime.GOOS == "windows" && os.IsPermission(err)) {
				// File does not exist (has been deleted).
				return eventDeleted, nil
			}
			return eventNone, err
		}

		// File got moved/renamed? Only report this if our open file has no more data.
		if !os.SameFile(origFi, fi) {
			return eventDeleted, nil
		}

		// File hasn't changed so wait until next retry.
		backoff.Wait()
	}

	return eventNone, backoff.Err()
}
