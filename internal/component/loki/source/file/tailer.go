package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tail"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/util"
)

type tailer struct {
	metrics   *metrics
	logger    *slog.Logger
	batcher   loki.StreamBatcher
	positions positions.Positions

	key                positions.Entry
	legacyPositionUsed bool

	tailFromEnd          bool
	onPositionsFileError OnPositionsFileError
	watcherConfig        tail.WatcherConfig

	running *atomic.Bool

	report sync.Once

	file          *tail.File
	encoding      string
	decompression DecompressionConfig
}

func newTailer(
	metrics *metrics,
	logger *slog.Logger,
	newBatcher func(labels model.LabelSet) loki.StreamBatcher,
	pos positions.Positions,
	opts sourceOptions,
) *tailer {

	labels := opts.labels.Merge(model.LabelSet{labelFilename: model.LabelValue(opts.path)})

	return &tailer{
		metrics:              metrics,
		logger:               logger.With("component", "tailer", "path", opts.path),
		batcher:              newBatcher(labels),
		positions:            pos,
		key:                  positions.Entry{Path: opts.path, Labels: opts.labels.String()},
		running:              atomic.NewBool(false),
		tailFromEnd:          opts.tailFromEnd,
		legacyPositionUsed:   opts.legacyPositionUsed,
		onPositionsFileError: opts.onPositionsFileError,
		watcherConfig: tail.WatcherConfig{
			MinPollFrequency: opts.fileWatch.MinPollFrequency,
			MaxPollFrequency: opts.fileWatch.MaxPollFrequency,
		},
		report:        sync.Once{},
		encoding:      opts.encoding,
		decompression: opts.decompressionConfig,
	}
}

func (t *tailer) Run(ctx, stop context.Context) {
	// Check if context was canceled between two calls to Run.
	if ctx.Err() != nil {
		return
	}

	pos, err := t.initRun()
	if err != nil {
		// We are retrying tailers until the target has disappeared.
		// We are mostly interested in this log if this happens directly when
		// the tailer is scheduled and not on retries.
		t.report.Do(func() {
			t.logger.Error("failed to run tailer", "error", err)
		})
		return
	}

	// We call report so that retries won't log.
	t.report.Do(func() {})
	t.tail(ctx, stop, pos)
}

func (t *tailer) initRun() (int64, error) {
	fi, err := os.Stat(t.key.Path)
	if err != nil {
		return 0, fmt.Errorf("failed to tail file: %w", err)
	}

	startFromEnd := t.tailFromEnd

	pos, err := t.positions.Get(t.key.Path, t.key.Labels)
	if err != nil {
		switch t.onPositionsFileError {
		case OnPositionsFileErrorSkip:
			return 0, fmt.Errorf("failed to get file position: %w", err)
		case OnPositionsFileErrorRestartEnd:
			startFromEnd = true
			t.logger.Info("reset position to end of file after position error")
		default:
			t.logger.Debug("unrecognized `on_positions_file_error` option, defaulting to `restart_from_beginning`", "option", t.onPositionsFileError)
			fallthrough
		case OnPositionsFileErrorRestartBeginning:
			pos = 0
			t.logger.Info("reset position to start of file after positions error")
		}
	}

	// If we translated legacy positions we should try to get position offset without labels
	// when no other position was matched.
	if pos == 0 && t.legacyPositionUsed {
		pos, err = t.positions.Get(t.key.Path, "{}")
		if err != nil {
			return 0, fmt.Errorf("failed to get file position with empty labels: %w", err)
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

	file, err := tail.NewFile(t.logger, &tail.Config{
		Filename:      t.key.Path,
		Offset:        pos,
		StartFromEnd:  startFromEnd,
		Encoding:      t.encoding,
		Compression:   t.decompression.GetFormat(),
		WatcherConfig: t.watcherConfig,
	})

	if err != nil {
		return pos, fmt.Errorf("failed to tail the file: %w", err)
	}

	t.file = file

	return pos, nil
}

// tail forwards lines from the file to the batcher. It returns when ctx or stop
// is canceled, or when a file that is read once reaches the end.
// If ctx is canceled it delivers what is left in the file first, if stop is
// canceled it returns without reading more.
func (t *tailer) tail(ctx, stop context.Context, pos int64) {
	t.running.Store(true)
	t.metrics.filesActive.Add(1.)
	t.logger.Info("start tailing file")

	if t.decompression.Enabled && t.decompression.InitialDelay > 0 {
		t.logger.Info("sleeping before reading file", "duration", t.decompression.InitialDelay.String())
		time.Sleep(t.decompression.InitialDelay)
	}

	// metricsInterval is how often the file metrics are updated while the tailer is delivering entries.
	const metricsInterval = 10 * time.Second
	var lastUpdatedMetrics = time.Time{}

	var (
		// readOffset is the offset after the last line added to the batcher,
		// deliveredOffset after the last flushed one. Only deliveredOffset is
		// stored in positions.
		readOffset      = pos
		deliveredOffset = pos
	)

	defer func() { t.finish(stop, deliveredOffset) }()

	var (
		// stopped reports whether we should return right away.
		stopped = func() bool { return stop.Err() != nil }
		// draining reports whether we should finish what is in the file and return.
		draining = func() bool { return ctx.Err() != nil }
	)

	for {
		if stopped() {
			return
		}

		line, err := t.next(t.readOnce() || draining())

		switch {
		case errors.Is(err, io.EOF):
			// Nothing more to read right now so we flush what we have.
			deliveredOffset = t.flush(stop, readOffset, deliveredOffset)
			t.updateMetrics(t.file, deliveredOffset)
			if t.readOnce() || draining() {
				return
			}

			// Wait returns early if ctx is canceled. We then read what is left and return above.
			err := t.file.Wait(ctx)
			if err != nil && !draining() {
				if !errors.Is(err, io.EOF) {
					t.logger.Error("failed to wait for new data", "err", err)
				}
				return
			}
			continue
		case err != nil:
			// Lines read before the error are not guaranteed to be read again, so deliver them.
			deliveredOffset = t.flush(stop, readOffset, deliveredOffset)

			// While draining, a deleted file may already be closed, so the read fails. That is expected.
			expected := draining() && util.IsEphemeralOrFileClosed(err)
			if !expected {
				t.logger.Error("failed to tail file", "err", err)
			}
			return
		}

		readOffset = line.Offset
		t.metrics.readLines.WithLabelValues(t.key.Path).Inc()
		full := t.batcher.Add(push.Entry{
			Timestamp: line.Time,
			Line:      line.Text,
		})
		if full {
			deliveredOffset = t.flush(stop, readOffset, deliveredOffset)
			// The metrics need a stat call, so outside of EOF they are only updated once per interval.
			// FIXME(kalleep): We can remove the inteval check once we have fully migrated to V2.
			if time.Since(lastUpdatedMetrics) >= metricsInterval {
				lastUpdatedMetrics = time.Now()
				t.updateMetrics(t.file, deliveredOffset)
			}
		}
	}
}

// next returns the next line from the file. If final is true, any trailing data
// without a newline is returned as a last line.
func (t *tailer) next(final bool) (*tail.Line, error) {
	line, err := t.file.Next()
	if errors.Is(err, io.EOF) && final {
		return t.file.Flush()
	}

	return line, err
}

// flush delivers added entries and returns the offset that is now delivered.
// If stop is canceled the entries are not delivered and the offset is not advanced,
// so they are read again on the next run. Other delivery errors drop the entries.
func (t *tailer) flush(stop context.Context, readOffset, deliveredOffset int64) int64 {
	if err := t.batcher.Flush(stop); err != nil {
		if stop.Err() != nil {
			return deliveredOffset
		}
		t.logger.Error("failed to send entries, dropping them", "err", err)
	}

	t.positions.Put(t.key.Path, t.key.Labels, readOffset)
	return readOffset
}

// finish closes the file, stores or removes the position and cleans up metrics
// when the tailer returns.
func (t *tailer) finish(stop context.Context, deliveredOffset int64) {
	t.running.Store(false)

	if err := t.file.Close(); err != nil {
		if util.IsEphemeralOrFileClosed(err) {
			// Don't log as error if the file is already closed, or we got an ephemeral error - it's a common case
			// when files are rotating while being read and the tailer would have stopped correctly anyway.
			t.logger.Debug("failed to stop tailer", "error", err)
		} else if !errors.Is(err, os.ErrNotExist) {
			// Log as error for other reasons, as a resource leak may have happened.
			t.logger.Error("failed to stop tailer", "error", err)
		}
	}

	if t.shouldKeepPosition(stop) {
		t.positions.Put(t.key.Path, t.key.Labels, deliveredOffset)
	} else {
		t.positions.Remove(t.key.Path, t.key.Labels)
	}

	t.cleanupMetrics()

	t.logger.Info("stopped tailing file")
}

func (t *tailer) Key() positions.Entry {
	return t.key
}

// shouldKeepPosition reports whether the position is kept when the tailer returns.
// It is kept if stop is canceled so we continue from it on restart, and for a file
// that is read once so it is not ingested again.
func (t *tailer) shouldKeepPosition(stop context.Context) bool {
	return stop.Err() != nil || t.readOnce()
}

// readOnce reports whether the file is read to the end once and then the tailer exits.
func (t *tailer) readOnce() bool {
	return t.decompression.Enabled
}

func (t *tailer) updateMetrics(f *tail.File, offset int64) {
	size, _ := f.Size()
	t.metrics.totalBytes.WithLabelValues(t.key.Path).Set(float64(size))
	t.metrics.readBytes.WithLabelValues(t.key.Path).Set(float64(offset))
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
