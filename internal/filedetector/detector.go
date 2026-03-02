package filedetector

import (
	"context"
	"encoding"
	"fmt"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/filedetector/internal/inotifyinfo"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// Detector is used to specify how changes to the file should be detected.
type Detector int

const (
	// DetectorInvalid indicates an invalid UpdateType.
	DetectorInvalid Detector = iota
	// DetectorFSNotify uses filesystem events to wait for changes to the file.
	DetectorFSNotify
	// DetectorPoll will re-read the file on an interval to detect changes.
	DetectorPoll

	// DetectorDefault holds the default UpdateType.
	DetectorDefault = DetectorFSNotify
)

var (
	_ encoding.TextMarshaler   = Detector(0)
	_ encoding.TextUnmarshaler = (*Detector)(nil)
)

// String returns the string representation of the UpdateType.
func (ut Detector) String() string {
	switch ut {
	case DetectorFSNotify:
		return "fsnotify"
	case DetectorPoll:
		return "poll"
	default:
		return fmt.Sprintf("Detector(%d)", ut)
	}
}

// MarshalText implements encoding.TextMarshaler.
func (ut Detector) MarshalText() (text []byte, err error) {
	return []byte(ut.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (ut *Detector) UnmarshalText(text []byte) error {
	switch string(text) {
	case "":
		*ut = DetectorDefault
	case "fsnotify":
		*ut = DetectorFSNotify
	case "poll":
		*ut = DetectorPoll
	default:
		return fmt.Errorf("unrecognized detector %q, expected fsnotify or poll", string(text))
	}
	return nil
}

type FSNotify struct {
	opts   FSNotifyOptions
	cancel context.CancelFunc

	// watcherMut is needed to prevent race conditions on Windows. This can be
	// removed once fsnotify/fsnotify#454 is merged and included in a patch
	// release.
	watcherMut sync.Mutex
	watcher    *fsnotify.Watcher
}

type FSNotifyOptions struct {
	Logger        log.Logger
	Filename      string
	ReloadFile    func()        // Callback to request file reload.
	PollFrequency time.Duration // How often to do fallback polling
}

// newFSNotify creates a new fsnotify detector which uses filesystem events to
// detect that a file has changed.
func NewFSNotify(opts FSNotifyOptions) (*FSNotify, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		kvs := []any{"msg", "failed to create fsnotify watcher", "err", err}

		if diags := inotifyinfo.DiagnosticsJson(opts.Logger); diags != "" {
			kvs = append(kvs, "diagnostics", diags)
		}

		level.Error(opts.Logger).Log(kvs...)
		return nil, err
	}
	if err := w.Add(opts.Filename); err != nil {
		// It's possible that the file already got deleted by the time our fsnotify
		// was created. We'll log the error and wait for our polling fallback for
		// the file to be recreated.
		level.Warn(opts.Logger).Log("msg", "failed to watch file", "err", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	wd := &FSNotify{
		opts:    opts,
		watcher: w,
		cancel:  cancel,
	}

	go wd.wait(ctx)
	return wd, nil
}

func (fsn *FSNotify) wait(ctx context.Context) {
	pollTick := time.NewTicker(fsn.opts.PollFrequency)
	defer pollTick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pollTick.C:
			// fsnotify falls back to polling in case the watch stopped (i.e., the
			// file got deleted) or failed.
			//
			// We'll use the poll period to re-establish the watch in case it was
			// stopped. This is a no-op if the watch is already active.
			fsn.watcherMut.Lock()
			err := fsn.watcher.Add(fsn.opts.Filename)
			fsn.watcherMut.Unlock()

			if err != nil {
				level.Warn(fsn.opts.Logger).Log("msg", "failed re-watch file", "err", err)
			}

			fsn.opts.ReloadFile()

		case err := <-fsn.watcher.Errors:
			// The fsnotify watcher can generate errors for OS-level reasons (watched
			// failed, failed when closing the file, etc.). We don't know if the error
			// is related to the file, so we always treat it as if the file updated.
			//
			// This will force the component to reload the file and report the error
			// directly to the user via the component health.
			if err != nil {
				level.Warn(fsn.opts.Logger).Log("msg", "got error from fsnotify watcher; treating as file updated event", "err", err)
				fsn.opts.ReloadFile()
			}
		case ev := <-fsn.watcher.Events:
			level.Debug(fsn.opts.Logger).Log("msg", "got fsnotify event", "op", ev.Op.String())
			fsn.opts.ReloadFile()
		}
	}
}

func (fsn *FSNotify) Close() error {
	fsn.watcherMut.Lock()
	defer fsn.watcherMut.Unlock()

	fsn.cancel()
	return fsn.watcher.Close()
}

type Poller struct {
	opts   PollerOptions
	cancel context.CancelFunc
}

type PollerOptions struct {
	Filename      string
	ReloadFile    func() // Callback to request file reload.
	PollFrequency time.Duration
}

// newPoller creates a new poll-based file update detector.
func NewPoller(opts PollerOptions) *Poller {
	ctx, cancel := context.WithCancel(context.Background())

	pw := &Poller{
		opts:   opts,
		cancel: cancel,
	}

	go pw.run(ctx)
	return pw
}

func (p *Poller) run(ctx context.Context) {
	t := time.NewTicker(p.opts.PollFrequency)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			// Always tell the component to re-check the file. This avoids situations
			// where the file changed without changing any of the stats (like modify
			// time).
			p.opts.ReloadFile()
		}
	}
}

// Close terminates the poller.
func (p *Poller) Close() error {
	p.cancel()
	return nil
}
