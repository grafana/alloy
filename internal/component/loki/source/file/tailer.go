package file

// This code is adapted from loki/promtail. Last revision used to port changes to Alloy was a8d5815510bd959a6dd8c176a5d9fd9bbfc8f8b5.
// tailer implements the reader interface by using the github.com/grafana/tail package to tail files.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tail"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
)

type tailer struct {
	metrics   *metrics
	logger    log.Logger
	receiver  loki.LogsReceiver
	positions positions.Positions

	key                positions.Entry
	labels             model.LabelSet
	legacyPositionUsed bool

	tailFromEnd          bool
	onPositionsFileError OnPositionsFileError
	watcherConfig        tail.WatcherConfig

	posAndSizeMtx sync.Mutex

	running *atomic.Bool

	componentStopping func() bool

	report sync.Once

	file    *tail.File
	decoder *encoding.Decoder
}

func newTailer(
	metrics *metrics,
	logger log.Logger,
	receiver loki.LogsReceiver,
	pos positions.Positions,
	componentStopping func() bool,
	opts sourceOptions,
) (*tailer, error) {

	decoder, err := getDecoder(opts.encoding)
	if err != nil {
		return nil, fmt.Errorf("failed to get decoder: %w", err)
	}

	tailer := &tailer{
		metrics:              metrics,
		logger:               log.With(logger, "component", "tailer"),
		receiver:             receiver,
		positions:            pos,
		key:                  positions.Entry{Path: opts.path, Labels: opts.labels.String()},
		labels:               opts.labels,
		running:              atomic.NewBool(false),
		tailFromEnd:          opts.tailFromEnd,
		legacyPositionUsed:   opts.legacyPositionUsed,
		onPositionsFileError: opts.onPositionsFileError,
		watcherConfig: tail.WatcherConfig{
			MinPollFrequency: opts.fileWatch.MinPollFrequency,
			MaxPollFrequency: opts.fileWatch.MaxPollFrequency,
		},
		componentStopping: componentStopping,
		report:            sync.Once{},
		decoder:           decoder,
	}

	return tailer, nil
}

// getLastLinePosition returns the offset of the start of the last line in the file at the given path.
// It will read chunks of bytes starting from the end of the file to return the position of the last '\n' + 1.
// If it cannot find any '\n' it will return 0.
func getLastLinePosition(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	const chunkSize = 1024

	buf := make([]byte, chunkSize)
	fi, err := file.Stat()
	if err != nil {
		return 0, err
	}

	if fi.Size() == 0 {
		return 0, nil
	}

	var pos = fi.Size() - chunkSize
	if pos < 0 {
		pos = 0
	}

	for {
		_, err = file.Seek(pos, io.SeekStart)
		if err != nil {
			return 0, err
		}

		bytesRead, err := file.Read(buf)
		if err != nil {
			return 0, err
		}

		idx := bytes.LastIndexByte(buf[:bytesRead], '\n')
		// newline found
		if idx != -1 {
			return pos + int64(idx) + 1, nil
		}

		// no newline found in the entire file
		if pos == 0 {
			return 0, nil
		}

		pos -= chunkSize
		if pos < 0 {
			pos = 0
		}
	}
}

func (t *tailer) Run(ctx context.Context) {
	// Check if context was canceled between two calls to Run.
	select {
	case <-ctx.Done():
		return
	default:
	}

	handler, err := t.initRun()

	if err != nil {
		// We are retrying tailers until the target has disappeared.
		// We are mostly interested in this log if this happens directly when
		// the tailer is scheduled and not on retries.
		t.report.Do(func() {
			level.Error(t.logger).Log("msg", "failed to run tailer", "err", err)
		})
		return
	}
	defer handler.Stop()

	// We call report so that retries won't log.
	t.report.Do(func() {})

	t.metrics.filesActive.Add(1.)

	done := make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		// readLines closes done on exit
		t.readLines(handler, done)
		cancel()
	}()

	t.running.Store(true)
	defer t.running.Store(false)

	<-ctx.Done()
	t.stop(done)
}

func (t *tailer) initRun() (loki.EntryHandler, error) {
	fi, err := os.Stat(t.key.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to tail file: %w", err)
	}

	pos, err := t.positions.Get(t.key.Path, t.key.Labels)
	if err != nil {
		switch t.onPositionsFileError {
		case OnPositionsFileErrorSkip:
			return nil, fmt.Errorf("failed to get file position: %w", err)
		case OnPositionsFileErrorRestartEnd:
			pos, err = getLastLinePosition(t.key.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to get last line position after positions error: %w", err)
			}
			level.Info(t.logger).Log("msg", "retrieved the position of the last line after positions error")
		default:
			level.Debug(t.logger).Log("msg", "unrecognized `on_positions_file_error` option, defaulting to `restart_from_beginning`", "option", t.onPositionsFileError)
			fallthrough
		case OnPositionsFileErrorRestartBeginning:
			pos = 0
			level.Info(t.logger).Log("msg", "reset position to start of file after positions error")
		}
	}

	// If we translated legacy positions we should try to get position offset without labels
	// when no other position was matched.
	if pos == 0 && t.legacyPositionUsed {
		pos, err = t.positions.Get(t.key.Path, "{}")
		if err != nil {
			return nil, fmt.Errorf("failed to get file position with empty labels: %w", err)
		}
	}

	// NOTE: The code assumes that if a position is available and that the file is smaller than the position, then
	// the tail should start from the position. This may not be always desired in situation where the file was rotated
	// with a file that has the same name but different content and a bigger size that the previous one. This problem would
	// mostly show up on Windows because on Unix systems, the readlines function is not exited on file rotation.
	// If this ever becomes a problem, we may want to consider saving and comparing file creation timestamps.
	if fi.Size() < pos {
		t.positions.Remove(t.key.Path, t.key.Labels)
	}

	// If no cached position is found and the tailFromEnd option is enabled.
	if pos == 0 && t.tailFromEnd {
		pos, err = getLastLinePosition(t.key.Path)
		if err != nil {
			level.Error(t.logger).Log("msg", "failed to get a position from the end of the file, default to start of file", err)
		} else {
			t.positions.Put(t.key.Path, t.key.Labels, pos)
			level.Info(t.logger).Log("msg", "retrieved and stored the position of the last line")
		}
	}

	tail, err := tail.NewFile(t.logger, &tail.Config{
		Filename:      t.key.Path,
		Offset:        pos,
		Decoder:       t.decoder,
		WatcherConfig: t.watcherConfig,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to tail the file: %w", err)
	}

	t.file = tail

	labelsMiddleware := t.labels.Merge(model.LabelSet{labelFilename: model.LabelValue(t.key.Path)})
	handler := loki.AddLabelsMiddleware(labelsMiddleware).Wrap(loki.NewEntryHandler(t.receiver.Chan(), func() {}))

	return handler, nil
}

func getDecoder(encoding string) (*encoding.Decoder, error) {
	if encoding == "" {
		return nil, nil
	}

	encoder, err := ianaindex.IANA.Encoding(encoding)
	if err != nil {
		return nil, fmt.Errorf("failed to get IANA encoding %s: %w", encoding, err)
	}
	return encoder.NewDecoder(), nil
}

// readLines consumes the t.tail.Lines channel from the
// underlying tailer. It will only exit when that channel is closed. This is
// important to avoid a deadlock in the underlying tailer which can happen if
// there are unread lines in this channel and the Stop method on the tailer is
// called, the underlying tailer will never exit if there are unread lines in
// the t.tail.Lines channel
func (t *tailer) readLines(handler loki.EntryHandler, done chan struct{}) {
	level.Info(t.logger).Log("msg", "tail routine: started", "path", t.key.Path)
	var (
		entries             = handler.Chan()
		lastOffset          = int64(0)
		positionInterval    = t.positions.SyncPeriod()
		lastUpdatedPosition = time.Time{}
	)

	defer func() {
		level.Info(t.logger).Log("msg", "tail routine: exited", "path", t.key.Path)
		size, _ := t.file.Size()
		t.updateStats(lastOffset, size)
		close(done)
	}()

	for {
		line, err := t.file.Next()
		if err != nil {
			// Maybe we should use a better signal than context canceled to indicate normal stop...
			if !errors.Is(err, context.Canceled) {
				level.Info(t.logger).Log("msg", "tail routine: stopping tailer", "path", t.key.Path, "err", err)
			}
			return
		}

		t.metrics.readLines.WithLabelValues(t.key.Path).Inc()
		entries <- loki.Entry{
			// Allocate the expected size of labels. This matches the number of labels added by the middleware
			// as configured in initRun().
			Labels: make(model.LabelSet, len(t.labels)+1),
			Entry: push.Entry{
				Timestamp: line.Time,
				Line:      line.Text,
			},
		}

		lastOffset = line.Offset
		if time.Since(lastUpdatedPosition) >= positionInterval {
			lastUpdatedPosition = time.Now()
			size, _ := t.file.Size()
			t.updateStats(lastOffset, size)
		}
	}
}

func (t *tailer) updateStats(offset int64, size int64) {
	// Update metrics and positions file all together to avoid race conditions when `t.tail` is stopped.
	t.metrics.totalBytes.WithLabelValues(t.key.Path).Set(float64(size))
	t.metrics.readBytes.WithLabelValues(t.key.Path).Set(float64(offset))
	t.positions.Put(t.key.Path, t.key.Labels, offset)

}

func (t *tailer) stop(done chan struct{}) {
	if err := t.file.Stop(); err != nil {
		if util.IsEphemeralOrFileClosed(err) {
			// Don't log as error if the file is already closed, or we got an ephemeral error - it's a common case
			// when files are rotating while being read and the tailer would have stopped correctly anyway.
			level.Debug(t.logger).Log("msg", "tailer stopped with file I/O error", "path", t.key.Path, "error", err)
		} else if !errors.Is(err, os.ErrNotExist) {
			// Log as error for other reasons, as a resource leak may have happened.
			level.Error(t.logger).Log("msg", "error stopping tailer", "path", t.key.Path, "error", err)
		}
	}

	level.Debug(t.logger).Log("msg", "waiting for readline and position marker to exit", "path", t.key.Path)

	// Wait for readLines() to consume all the remaining messages and exit when the channel is closed
	<-done

	level.Info(t.logger).Log("msg", "stopped tailing file", "path", t.key.Path)

	// If the component is not stopping, then it means that the target for this component is gone and that
	// we should clear the entry from the positions file.
	if !t.componentStopping() {
		t.positions.Remove(t.key.Path, t.key.Labels)
	}
}

func (t *tailer) Key() positions.Entry {
	return t.key
}

func (t *tailer) IsRunning() bool {
	return t.running.Load()
}

// cleanupMetrics removes all metrics exported by this tailer
func (t *tailer) cleanupMetrics() {
	// When we stop tailing the file, also un-export metrics related to the file
	t.metrics.filesActive.Add(-1.)
	t.metrics.readLines.DeleteLabelValues(t.key.Path)
	t.metrics.readBytes.DeleteLabelValues(t.key.Path)
	t.metrics.totalBytes.DeleteLabelValues(t.key.Path)
}
