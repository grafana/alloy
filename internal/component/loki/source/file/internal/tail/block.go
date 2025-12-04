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
func blockUntilEvent(ctx context.Context, f *os.File, pos int64, cfg *Config) (event, error) {
	origFi, err := f.Stat()
	if err != nil {
		return eventNone, err
	}

	backoff := backoff.New(ctx, backoff.Config{
		MinBackoff: cfg.WatcherConfig.MinPollFrequency,
		MaxBackoff: cfg.WatcherConfig.MaxPollFrequency,
	})

	var (
		prevSize    = pos
		prevModTime = origFi.ModTime()
	)
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

		// File got moved/renamed?
		if !os.SameFile(origFi, fi) {
			return eventDeleted, nil
		}

		// File got truncated?
		currentSize := fi.Size()
		if prevSize > 0 && prevSize > currentSize {
			return eventTruncated, nil
		}

		// File got bigger?
		if prevSize > 0 && prevSize < currentSize {
			return eventModified, nil
		}
		prevSize = currentSize

		// File was appended to (changed)?
		modTime := fi.ModTime()
		if modTime != prevModTime {
			prevModTime = modTime
			return eventModified, nil
		}

		// File hasn't changed; increase backoff for next sleep.
		backoff.Wait()
	}

	return eventNone, backoff.Err()
}
