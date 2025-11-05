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
	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tail"
	"github.com/grafana/alloy/internal/component/loki/source/file/internal/tail/watch"
	"github.com/grafana/alloy/internal/loki/util"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/transform"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/component/common/loki/utils"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

type tailer struct {
	metrics   *metrics
	logger    log.Logger
	receiver  loki.LogsReceiver
	positions positions.Positions

	key                positions.Entry
	labels             model.LabelSet
	legacyPositionUsed bool

	tailFromEnd bool
	pollOptions watch.PollingFileWatcherOptions

	posAndSizeMtx sync.Mutex

	running *atomic.Bool

	componentStopping func() bool

	report sync.Once

	tail    *tail.Tail
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

	tailer := &tailer{
		metrics:            metrics,
		logger:             log.With(logger, "component", "tailer"),
		receiver:           receiver,
		positions:          pos,
		key:                positions.Entry{Path: opts.path, Labels: opts.labels.String()},
		labels:             opts.labels,
		running:            atomic.NewBool(false),
		tailFromEnd:        opts.tailFromEnd,
		legacyPositionUsed: opts.legacyPositionUsed,
		pollOptions: watch.PollingFileWatcherOptions{
			MinPollFrequency: opts.fileWatch.MinPollFrequency,
			MaxPollFrequency: opts.fileWatch.MaxPollFrequency,
		},
		componentStopping: componentStopping,
		report:            sync.Once{},
	}

	if opts.encoding != "" {
		level.Info(tailer.logger).Log("msg", "Will decode messages", "from", opts.encoding, "to", "UTF8")
		encoder, err := ianaindex.IANA.Encoding(opts.encoding)
		if err != nil {
			return nil, fmt.Errorf("failed to get IANA encoding %s: %w", opts.encoding, err)
		}
		decoder := encoder.NewDecoder()
		tailer.decoder = decoder
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
		return nil, fmt.Errorf("failed to get file position: %w", err)
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

	tail, err := tail.TailFile(t.key.Path, tail.Config{
		Follow:    true,
		Poll:      true,
		ReOpen:    true,
		MustExist: true,
		Location: &tail.SeekInfo{
			Offset: pos,
			Whence: 0,
		},
		Logger:      util.NewLogAdapter(t.logger),
		PollOptions: t.pollOptions,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to tail the file: %w", err)
	}

	t.tail = tail

	labelsMiddleware := t.labels.Merge(model.LabelSet{labelFilename: model.LabelValue(t.key.Path)})
	handler := loki.AddLabelsMiddleware(labelsMiddleware).Wrap(loki.NewEntryHandler(t.receiver.Chan(), func() {}))

	return handler, nil
}

// updatePosition is run in a goroutine and checks the current size of the file
// and saves it to the positions file at a regular interval. If there is ever
// an error it stops the tailer and exits, the tailer will be re-opened by the
// backoff retry method if it still exists and will start reading from the
// last successful entry in the positions file.
func (t *tailer) updatePosition(posquit chan struct{}) {
	positionSyncPeriod := t.positions.SyncPeriod()
	positionWait := time.NewTicker(positionSyncPeriod)
	defer func() {
		positionWait.Stop()
		level.Info(t.logger).Log("msg", "position timer: exited", "path", t.key.Path)
		// NOTE: metrics must be cleaned up after the position timer exits, as markPositionAndSize() updates metrics.
		t.cleanupMetrics()
	}()

	for {
		select {
		case <-positionWait.C:
			err := t.markPositionAndSize()
			if err != nil {
				level.Error(t.logger).Log("msg", "position timer: error getting tail position and/or size, stopping tailer", "path", t.key.Path, "error", err)
				err := t.tail.Stop()
				if err != nil {
					level.Error(t.logger).Log("msg", "position timer: error stopping tailer", "path", t.key.Path, "error", err)
				}
				return
			}
		case <-posquit:
			return
		}
	}
}

// readLines consumes the t.tail.Lines channel from the
// underlying tailer. It will only exit when that channel is closed. This is
// important to avoid a deadlock in the underlying tailer which can happen if
// there are unread lines in this channel and the Stop method on the tailer is
// called, the underlying tailer will never exit if there are unread lines in
// the t.tail.Lines channel
func (t *tailer) readLines(handler loki.EntryHandler, done chan struct{}) {
	level.Info(t.logger).Log("msg", "tail routine: started", "path", t.key.Path)

	posquit, posdone := make(chan struct{}), make(chan struct{})
	go func() {
		t.updatePosition(posquit)
		close(posdone)
	}()

	// This function runs in a goroutine, if it exits this tailer will never do any more tailing.
	// Clean everything up.
	defer func() {
		level.Info(t.logger).Log("msg", "tail routine: exited", "path", t.key.Path)
		// Shut down the position marker thread
		close(posquit)
		<-posdone
		close(done)
	}()

	entries := handler.Chan()
	for {
		line, ok := <-t.tail.Lines
		if !ok {
			level.Info(t.logger).Log("msg", "tail routine: tail channel closed, stopping tailer", "path", t.key.Path, "reason", t.tail.Tomb.Err())
			return
		}

		// Note currently the tail implementation hardcodes Err to nil, this should never hit.
		if line.Err != nil {
			level.Error(t.logger).Log("msg", "tail routine: error reading line", "path", t.key.Path, "error", line.Err)
			continue
		}

		var text string
		if t.decoder != nil {
			var err error
			text, err = t.convertToUTF8(line.Text)
			if err != nil {
				level.Debug(t.logger).Log("msg", "failed to convert encoding", "error", err)
				t.metrics.encodingFailures.WithLabelValues(t.key.Path).Inc()
				text = fmt.Sprintf("the requested encoding conversion for this line failed in Alloy: %s", err.Error())
			}
		} else {
			text = line.Text
		}

		t.metrics.readLines.WithLabelValues(t.key.Path).Inc()
		entries <- loki.Entry{
			// Allocate the expected size of labels. This matches the number of labels added by the middleware
			// as configured in initRun().
			Labels: make(model.LabelSet, len(t.labels)+1),
			Entry: push.Entry{
				Timestamp: line.Time,
				Line:      text,
			},
		}
	}
}

func (t *tailer) markPositionAndSize() error {
	// Lock this update because it can be called in two different goroutines
	t.posAndSizeMtx.Lock()
	defer t.posAndSizeMtx.Unlock()

	size, err := t.tail.Size()
	if err != nil {
		// If the file no longer exists, no need to save position information
		if err == os.ErrNotExist {
			level.Info(t.logger).Log("msg", "skipping update of position for a file which does not currently exist", "path", t.key.Path)
			return nil
		}
		return err
	}

	pos, err := t.tail.Tell()
	if err != nil {
		return err
	}

	// Update metrics and positions file all together to avoid race conditions when `t.tail` is stopped.
	t.metrics.totalBytes.WithLabelValues(t.key.Path).Set(float64(size))
	t.metrics.readBytes.WithLabelValues(t.key.Path).Set(float64(pos))
	t.positions.Put(t.key.Path, t.key.Labels, pos)

	return nil
}

func (t *tailer) stop(done chan struct{}) {
	// Save the current position before shutting down tailer to ensure that if the file is tailed again
	// it start where it left off.
	if err := t.markPositionAndSize(); err != nil {
		level.Error(t.logger).Log("msg", "error marking file position when stopping tailer", "path", t.key.Path, "error", err)
	}
	if err := t.tail.Stop(); err != nil {
		if utils.IsEphemeralOrFileClosed(err) {
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

func (t *tailer) convertToUTF8(text string) (string, error) {
	res, _, err := transform.String(t.decoder, text)
	if err != nil {
		return "", fmt.Errorf("failed to decode text to UTF8: %w", err)
	}

	return res, nil
}

// cleanupMetrics removes all metrics exported by this tailer
func (t *tailer) cleanupMetrics() {
	// When we stop tailing the file, also un-export metrics related to the file
	t.metrics.filesActive.Add(-1.)
	t.metrics.readLines.DeleteLabelValues(t.key.Path)
	t.metrics.readBytes.DeleteLabelValues(t.key.Path)
	t.metrics.totalBytes.DeleteLabelValues(t.key.Path)
}
