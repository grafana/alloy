package tailv2

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tail/util"
	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tailv2/fileext"
	"github.com/grafana/dskit/backoff"
)

// watcher polls the file for changes.
type watcher struct {
	filename string
	cfg      WatcherConfig

	ctx    context.Context
	cancel context.CancelFunc
}

func newWatcher(filename string, cfg WatcherConfig) (*watcher, error) {
	if cfg == (WatcherConfig{}) {
		cfg = DefaultWatcherConfig
	}

	if cfg.MinPollFrequency == 0 || cfg.MaxPollFrequency == 0 {
		return nil, fmt.Errorf("MinPollFrequency and MaxPollFrequency must be greater than 0")
	} else if cfg.MaxPollFrequency < cfg.MinPollFrequency {
		return nil, fmt.Errorf("MaxPollFrequency must be larger than MinPollFrequency")
	}

	return &watcher{
		filename: filename,
		cfg:      cfg,
	}, nil
}

// blockUntilExists will block until either file exists or context is canceled.
func (fw *watcher) blockUntilExists(ctx context.Context) error {
	backoff := backoff.New(ctx, backoff.Config{
		MinBackoff: fw.cfg.MinPollFrequency,
		MaxBackoff: fw.cfg.MaxPollFrequency,
	})

	for backoff.Ongoing() {
		if _, err := os.Stat(fw.filename); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}

		backoff.Wait()
	}

	return backoff.Err()
}

// blockUntilEvent will block until it detects a new event for file or context is canceled.
func (fw *watcher) blockUntilEvent(ctx context.Context, f *os.File, pos int64) (event, error) {
	origFi, err := f.Stat()
	if err != nil {
		return eventNone, err
	}

	backoff := backoff.New(ctx, backoff.Config{
		MinBackoff: fw.cfg.MinPollFrequency,
		MaxBackoff: fw.cfg.MaxPollFrequency,
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

		fi, err := os.Stat(fw.filename)
		if err != nil {
			// Windows cannot delete a file if a handle is still open (tail keeps one open)
			// so it gives access denied to anything trying to read it until all handles are released.
			if os.IsNotExist(err) || (runtime.GOOS == "windows" && os.IsPermission(err)) {
				// File does not exist (has been deleted).
				return eventDeleted, nil
			}

			// XXX: report this error back to the user
			util.Fatal("Failed to stat file %v: %v", fw.filename, err)
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

type event int

const (
	eventNone event = iota
	eventTruncated
	eventModified
	eventDeleted
)
