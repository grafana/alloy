package file

// This code is adapted from loki/promtail. Last revision used to port changes to Alloy was a8d5815510bd959a6dd8c176a5d9fd9bbfc8f8b5.
// tailer implements the reader interface by using the github.com/grafana/tail package to tail files.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/grafana/loki/v3/pkg/util"
	"github.com/grafana/tail"
	"github.com/grafana/tail/watch"
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

	path      string
	labelsStr string
	labels    model.LabelSet

	tailFromEnd bool
	pollOptions watch.PollingFileWatcherOptions

	posAndSizeMtx sync.Mutex

	running *atomic.Bool

	componentStopping func() bool

	tail    *tail.Tail
	decoder *encoding.Decoder
}

func newTailer(metrics *metrics, logger log.Logger, receiver loki.LogsReceiver, positions positions.Positions, path string,
	labels model.LabelSet, encoding string, pollOptions watch.PollingFileWatcherOptions, tailFromEnd bool, componentStopping func() bool) (*tailer, error) {

	tailer := &tailer{
		metrics:           metrics,
		logger:            log.With(logger, "component", "tailer"),
		receiver:          receiver,
		positions:         positions,
		path:              path,
		labels:            labels,
		labelsStr:         labels.String(),
		running:           atomic.NewBool(false),
		tailFromEnd:       tailFromEnd,
		pollOptions:       pollOptions,
		componentStopping: componentStopping,
	}

	if encoding != "" {
		level.Info(tailer.logger).Log("msg", "Will decode messages", "from", encoding, "to", "UTF8")
		encoder, err := ianaindex.IANA.Encoding(encoding)
		if err != nil {
			return nil, fmt.Errorf("failed to get IANA encoding %s: %w", encoding, err)
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
		level.Error(t.logger).Log("msg", "failed to run tailer", "err", err)
		return
	}
	defer handler.Stop()

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
	fi, err := os.Stat(t.path)
	if err != nil {
		return nil, fmt.Errorf("failed to tail file: %w", err)
	}

	pos, err := t.positions.Get(t.path, t.labelsStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get file position: %w", err)
	}

	// NOTE: The code assumes that if a position is available and that the file is bigger than the position, then
	// the tail should start from the position. This may not be always desired in situation where the file was rotated
	// with a file that has the same name but different content and a bigger size that the previous one. This problem would
	// mostly show up on Windows because on Unix systems, the readlines function is not exited on file rotation.
	// If this ever becomes a problem, we may want to consider saving and comparing file creation timestamps.
	if fi.Size() < pos {
		t.positions.Remove(t.path, t.labelsStr)
	}

	// If no cached position is found and the tailFromEnd option is enabled.
	if pos == 0 && t.tailFromEnd {
		pos, err = getLastLinePosition(t.path)
		if err != nil {
			level.Error(t.logger).Log("msg", "failed to get a position from the end of the file, default to start of file", err)
		} else {
			t.positions.Put(t.path, t.labelsStr, pos)
			level.Info(t.logger).Log("msg", "retrieved and stored the position of the last line")
		}
	}

	tail, err := tail.TailFile(t.path, tail.Config{
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

	labelsMiddleware := t.labels.Merge(model.LabelSet{filenameLabel: model.LabelValue(t.path)})
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
		level.Info(t.logger).Log("msg", "position timer: exited", "path", t.path)
		// NOTE: metrics must be cleaned up after the position timer exits, as markPositionAndSize() updates metrics.
		t.cleanupMetrics()
	}()

	for {
		select {
		case <-positionWait.C:
			err := t.markPositionAndSize()
			if err != nil {
				level.Error(t.logger).Log("msg", "position timer: error getting tail position and/or size, stopping tailer", "path", t.path, "error", err)
				err := t.tail.Stop()
				if err != nil {
					level.Error(t.logger).Log("msg", "position timer: error stopping tailer", "path", t.path, "error", err)
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
	level.Info(t.logger).Log("msg", "tail routine: started", "path", t.path)

	posquit, posdone := make(chan struct{}), make(chan struct{})
	go func() {
		t.updatePosition(posquit)
		close(posdone)
	}()

	// This function runs in a goroutine, if it exits this tailer will never do any more tailing.
	// Clean everything up.
	defer func() {
		level.Info(t.logger).Log("msg", "tail routine: exited", "path", t.path)
		// Shut down the position marker thread
		close(posquit)
		<-posdone
		close(done)
	}()

	entries := handler.Chan()
	for {
		line, ok := <-t.tail.Lines
		if !ok {
			level.Info(t.logger).Log("msg", "tail routine: tail channel closed, stopping tailer", "path", t.path, "reason", t.tail.Tomb.Err())
			return
		}

		// Note currently the tail implementation hardcodes Err to nil, this should never hit.
		if line.Err != nil {
			level.Error(t.logger).Log("msg", "tail routine: error reading line", "path", t.path, "error", line.Err)
			continue
		}

		var text string
		if t.decoder != nil {
			var err error
			text, err = t.convertToUTF8(line.Text)
			if err != nil {
				level.Debug(t.logger).Log("msg", "failed to convert encoding", "error", err)
				t.metrics.encodingFailures.WithLabelValues(t.path).Inc()
				text = fmt.Sprintf("the requested encoding conversion for this line failed in Alloy: %s", err.Error())
			}
		} else {
			text = line.Text
		}

		t.metrics.readLines.WithLabelValues(t.path).Inc()
		entries <- loki.Entry{
			Labels: model.LabelSet{},
			Entry: logproto.Entry{
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
			level.Info(t.logger).Log("msg", "skipping update of position for a file which does not currently exist", "path", t.path)
			return nil
		}
		return err
	}

	pos, err := t.tail.Tell()
	if err != nil {
		return err
	}

	// Update metrics and positions file all together to avoid race conditions when `t.tail` is stopped.
	t.metrics.totalBytes.WithLabelValues(t.path).Set(float64(size))
	t.metrics.readBytes.WithLabelValues(t.path).Set(float64(pos))
	t.positions.Put(t.path, t.labelsStr, pos)

	return nil
}

func (t *tailer) stop(done chan struct{}) {
	// Save the current position before shutting down tailer to ensure that if the file is tailed again
	// it start where it left off.
	err := t.markPositionAndSize()
	if err != nil {
		level.Error(t.logger).Log("msg", "error marking file position when stopping tailer", "path", t.path, "error", err)
	}

	err = t.tail.Stop()
	if err != nil {
		if utils.IsEphemeralOrFileClosed(err) {
			// Don't log as error if the file is already closed, or we got an ephemeral error - it's a common case
			// when files are rotating while being read and the tailer would have stopped correctly anyway.
			level.Debug(t.logger).Log("msg", "tailer stopped with file I/O error", "path", t.path, "error", err)
		} else {
			// Log as error for other reasons, as a resource leak may have happened.
			level.Error(t.logger).Log("msg", "error stopping tailer", "path", t.path, "error", err)
		}
	}

	level.Debug(t.logger).Log("msg", "waiting for readline and position marker to exit", "path", t.path)

	// Wait for readLines() to consume all the remaining messages and exit when the channel is closed
	<-done

	level.Info(t.logger).Log("msg", "stopped tailing file", "path", t.path)

	// If the component is not stopping, then it means that the target for this component is gone and that
	// we should clear the entry from the positions file.
	if !t.componentStopping() {
		t.positions.Remove(t.path, t.labelsStr)
	}
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
	t.metrics.readLines.DeleteLabelValues(t.path)
	t.metrics.readBytes.DeleteLabelValues(t.path)
	t.metrics.totalBytes.DeleteLabelValues(t.path)
}
