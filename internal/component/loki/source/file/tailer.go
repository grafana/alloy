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

	componentStopping func() bool

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
	componentStopping func() bool,
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
		componentStopping: componentStopping,
		report:            sync.Once{},
		encoding:          opts.encoding,
		decompression:     opts.decompressionConfig,
	}
}

func (t *tailer) Run(ctx context.Context) {
	// Check if context was canceled between two calls to Run.
	select {
	case <-ctx.Done():
		return
	default:
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
	t.tail(ctx, pos)
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

// tail reads lines from the file and forwards them to the batcher, periodically
// recording the read offset. It blocks until ctx is canceled or the file is fully read.
func (t *tailer) tail(ctx context.Context, pos int64) {
	t.running.Store(true)
	t.metrics.filesActive.Add(1.)
	t.logger.Info("start tailing file")

	if t.decompression.Enabled && t.decompression.InitialDelay > 0 {
		t.logger.Info("sleeping before reading file", "duration", t.decompression.InitialDelay.String())
		time.Sleep(t.decompression.InitialDelay)
	}

	var (
		positionInterval    = t.positions.SyncPeriod()
		lastUpdatedPosition = time.Time{}

		// readOffset is the offset after the last line added to the batcher,
		// deliveredOffset after the last flushed one. Only deliveredOffset is
		// stored in positions.
		readOffset      = pos
		deliveredOffset = pos
	)

	defer func() {
		t.running.Store(false)
		size, _ := t.file.Size()
		t.updateStats(deliveredOffset, size)

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

		t.logger.Info("stopped tailing file")

		// We need to cleanup created metrics.
		t.cleanupMetrics()
		if !t.shouldKeepPosition() {
			t.positions.Remove(t.key.Path, t.key.Labels)
		}
	}()

	// flush delivers added entries and then advances deliveredOffset. Entries that
	// fail to be delivered are dropped, unless the tailer is stopping so they are
	// read again on the next run.
	flush := func() {
		if err := t.batcher.Flush(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			t.logger.Error("failed to send entries, dropping them", "err", err)
		}

		deliveredOffset = readOffset
		if time.Since(lastUpdatedPosition) >= positionInterval {
			lastUpdatedPosition = time.Now()
			size, _ := t.file.Size()
			t.updateStats(deliveredOffset, size)
		}
	}

	for {
		if ctx.Err() != nil {
			return
		}

		line, err := t.file.Next()
		if errors.Is(err, io.EOF) && t.decompression.Enabled {
			// Compressed files are read once, so return any trailing data without a newline.
			line, err = t.file.Flush()
		}

		switch {
		case errors.Is(err, io.EOF):
			// Nothing more to read right now so we flush what we have.
			flush()
			if t.decompression.Enabled {
				return
			}
			if err := t.file.Wait(ctx); err != nil {
				if !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
					t.logger.Error("failed to wait for new data", "err", err)
				}
				return
			}
			continue
		case err != nil:
			// Lines read before the error are not guaranteed to be read again, so deliver them.
			flush()
			t.logger.Error("failed to tail file", "err", err)
			return
		}

		readOffset = line.Offset
		t.metrics.readLines.WithLabelValues(t.key.Path).Inc()
		full := t.batcher.Add(push.Entry{
			Timestamp: line.Time,
			Line:      line.Text,
		})
		if full {
			flush()
		}
	}
}

func (t *tailer) updateStats(offset int64, size int64) {
	// Update metrics and the positions file together so the reported offset stays consistent.
	t.metrics.totalBytes.WithLabelValues(t.key.Path).Set(float64(size))
	t.metrics.readBytes.WithLabelValues(t.key.Path).Set(float64(offset))
	t.positions.Put(t.key.Path, t.key.Labels, offset)
}

func (t *tailer) Key() positions.Entry {
	return t.key
}

func (t *tailer) shouldKeepPosition() bool {
	// NOTE: We want to keep position if component is stopping or decompression is enabled.
	// If component is not stopping that means that target is gone and we should no longer tail the file.
	// If decompression is enabled we read file until we reach EOF and stop so tailer will exit, but we need
	// to remember the position so that we don't re-ingest it on restart.
	return t.componentStopping() || t.decompression.Enabled
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
