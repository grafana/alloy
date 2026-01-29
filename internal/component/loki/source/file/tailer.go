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

	running *atomic.Bool

	componentStopping func() bool

	report sync.Once

	file          *tail.File
	encoding      string
	decompression DecompressionConfig
}

func newTailer(
	metrics *metrics,
	logger log.Logger,
	receiver loki.LogsReceiver,
	pos positions.Positions,
	componentStopping func() bool,
	opts sourceOptions,
) *tailer {

	return &tailer{
		metrics:              metrics,
		logger:               log.With(logger, "component", "tailer", "path", opts.path),
		receiver:             receiver,
		positions:            pos,
		key:                  positions.Entry{Path: opts.path, Labels: opts.labels.String()},
		labels:               opts.labels.Merge(model.LabelSet{labelFilename: model.LabelValue(opts.path)}),
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
		encoding:          opts.encoding,
		decompression:     opts.decompressionConfig,
	}
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

	err := t.initRun()
	if err != nil {
		// We are retrying tailers until the target has disappeared.
		// We are mostly interested in this log if this happens directly when
		// the tailer is scheduled and not on retries.
		t.report.Do(func() {
			level.Error(t.logger).Log("msg", "failed to run tailer", "error", err)
		})
		return
	}

	// We call report so that retries won't log.
	t.report.Do(func() {})

	t.metrics.filesActive.Add(1.)

	done := make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		// readLines closes done on exit
		t.readLines(done)
		cancel()
	}()

	t.running.Store(true)
	defer t.running.Store(false)

	<-ctx.Done()
	t.stop(done)
}

func (t *tailer) initRun() error {
	fi, err := os.Stat(t.key.Path)
	if err != nil {
		return fmt.Errorf("failed to tail file: %w", err)
	}

	pos, err := t.positions.Get(t.key.Path, t.key.Labels)
	if err != nil {
		switch t.onPositionsFileError {
		case OnPositionsFileErrorSkip:
			return fmt.Errorf("failed to get file position: %w", err)
		case OnPositionsFileErrorRestartEnd:
			pos, err = getLastLinePosition(t.key.Path)
			if err != nil {
				return fmt.Errorf("failed to get last line position after positions error: %w", err)
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
			return fmt.Errorf("failed to get file position with empty labels: %w", err)
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
			level.Error(t.logger).Log("msg", "failed to get a position from the end of the file, default to start of file", "error", err)
		} else {
			t.positions.Put(t.key.Path, t.key.Labels, pos)
			level.Info(t.logger).Log("msg", "retrieved and stored the position of the last line")
		}
	}

	tail, err := tail.NewFile(t.logger, &tail.Config{
		Filename:      t.key.Path,
		Offset:        pos,
		Encoding:      t.encoding,
		Compression:   t.decompression.GetFormat(),
		WatcherConfig: t.watcherConfig,
	})

	if err != nil {
		return fmt.Errorf("failed to tail the file: %w", err)
	}

	t.file = tail

	return nil
}

// readLines reads lines from the tailed file by calling Next() in a loop.
// It processes each line by sending it to the receiver's channel and updates
// position tracking periodically. It exits when Next() returns an error,
// this happens when the tail.File is stopped or or we have a unrecoverable error.
func (t *tailer) readLines(done chan struct{}) {
	level.Info(t.logger).Log("msg", "start tailing file")

	if t.decompression.Enabled && t.decompression.InitialDelay > 0 {
		level.Info(t.logger).Log("msg", "sleeping before reading file", "duration", t.decompression.InitialDelay.String())
		time.Sleep(t.decompression.InitialDelay)
	}

	var (
		entries             = t.receiver.Chan()
		lastOffset          = int64(0)
		positionInterval    = t.positions.SyncPeriod()
		lastUpdatedPosition = time.Time{}
	)

	defer func() {
		size, _ := t.file.Size()
		t.updateStats(lastOffset, size)
		close(done)
	}()

	for {
		line, err := t.file.Next()
		if err != nil {
			// We get context.Canceled if tail.File was stopped so we don't have to log it.
			// If we get context.Canceled it means that tail.File was stopped. If we get EOF
			// that means that we consumed the file fully and don't wait for more events, this
			// happens when compression is configured.
			if !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
				level.Error(t.logger).Log("msg", "failed to tail file", "err", err)
			}
			return
		}

		t.metrics.readLines.WithLabelValues(t.key.Path).Inc()
		entries <- loki.Entry{
			Labels: t.labels,
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
			level.Debug(t.logger).Log("msg", "failed to stop tailer", "error", err)
		} else if !errors.Is(err, os.ErrNotExist) {
			// Log as error for other reasons, as a resource leak may have happened.
			level.Error(t.logger).Log("msg", "failed to stop tailer", "error", err)
		}
	}

	level.Debug(t.logger).Log("msg", "waiting for readLines to exit")

	// Wait for readLines() to consume all the remaining messages and exit when the channel is closed
	<-done

	level.Info(t.logger).Log("msg", "stopped tailing file")

	// We need to cleanup created metrics
	t.cleanupMetrics()

	// If the component is not stopping, then it means that the target for this component is gone and that
	// we should clear the entry from the positions file.
	if !t.componentStopping() {
		t.positions.Remove(t.key.Path, t.key.Labels)
	}
}

func (t *tailer) Key() positions.Entry {
	return t.key
}

// cleanupMetrics removes all metrics exported by this tailer
func (t *tailer) cleanupMetrics() {
	// When we stop tailing the file, also un-export metrics related to the file
	t.metrics.filesActive.Add(-1.)
	t.metrics.readLines.DeleteLabelValues(t.key.Path)
	t.metrics.readBytes.DeleteLabelValues(t.key.Path)
	t.metrics.totalBytes.DeleteLabelValues(t.key.Path)
}

func (t *tailer) DebugInfo() any {
	offset, _ := t.positions.Get(t.key.Path, t.key.Labels)
	return sourceDebugInfo{
		Path:       t.key.Path,
		Labels:     t.key.Labels,
		IsRunning:  t.running.Load(),
		ReadOffset: offset,
	}
}
