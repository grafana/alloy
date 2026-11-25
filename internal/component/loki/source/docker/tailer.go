package docker

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/loki/pkg/push"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
)

var restartBackoff = backoff.Config{
	MinBackoff: time.Second,
	MaxBackoff: time.Minute,
}

// tailer for Docker container logs.
type tailer struct {
	logger         *slog.Logger
	recv           loki.LogsReceiver
	positions      positions.Positions
	containerID    string
	labels         model.LabelSet
	labelsStr      string
	relabelConfig  []*relabel.Config
	metrics        *metrics
	restartBackoff backoff.Config

	client client.APIClient

	running *atomic.Bool
	err     *atomic.Error
	last    *atomic.Int64
	since   *atomic.Int64
}

// newTailer starts a new tailer to read logs from a given container ID.
func newTailer(
	metrics *metrics, logger *slog.Logger, recv loki.LogsReceiver, position positions.Positions, containerID string,
	labels model.LabelSet, relabelConfig []*relabel.Config, client client.APIClient, restartBackoff backoff.Config,
) (*tailer, error) {

	labelsStr := labels.String()
	pos, err := position.Get(positions.CursorKey(containerID), labelsStr)
	if err != nil {
		return nil, err
	}

	return &tailer{
		logger:         logger.With("container", containerID),
		recv:           recv,
		positions:      position,
		containerID:    containerID,
		labels:         labels,
		labelsStr:      labelsStr,
		relabelConfig:  relabelConfig,
		metrics:        metrics,
		client:         client,
		restartBackoff: restartBackoff,
		running:        atomic.NewBool(false),
		err:            atomic.NewError(nil),
		since:          atomic.NewInt64(pos),
		last:           atomic.NewInt64(0),
	}, nil
}

func (t *tailer) Run(ctx, stop context.Context) {
	defer func() {
		// stop is only canceled on shutdown or reset. Otherwise the target or
		// container is gone and its position is no longer needed.
		if stop.Err() == nil {
			t.positions.Remove(positions.CursorKey(t.containerID), t.labelsStr)
		}
	}()

	bo := backoff.New(ctx, t.restartBackoff)

	for bo.Ongoing() {
		start := time.Now()
		t.tail(ctx, stop)
		// Only short-lived sessions can cause a tight restart loop, so a
		// session that outlived the maximum backoff resets it.
		if time.Since(start) >= t.restartBackoff.MaxBackoff {
			bo.Reset()
		}

		if !t.waitForRestart(ctx, bo) {
			return
		}
	}
}

// waitForRestart waits until the container has logs to tail again. It returns
// false if the container no longer exists or ctx is canceled.
func (t *tailer) waitForRestart(ctx context.Context, bo *backoff.Backoff) bool {
	for {
		bo.Wait()
		if !bo.Ongoing() {
			return false
		}

		res, err := t.client.ContainerInspect(ctx, t.containerID, client.ContainerInspectOptions{})
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				t.logger.Info("container no longer exists, stopping tailer")
				return false
			}
			if !errors.Is(err, context.Canceled) {
				t.logger.Error("error inspecting Docker container", "error", err)
			}
			continue
		}

		finished, err := time.Parse(time.RFC3339Nano, res.Container.State.FinishedAt)
		if err != nil {
			t.logger.Error("error parsing finished time for Docker container", "error", err)
			finished = time.Unix(0, 0)
		}

		if res.Container.State.Running || finished.Unix() >= t.last.Load() {
			return true
		}
	}
}

// tail reads container logs until the log stream ends or ctx is canceled.
// Entries not yet forwarded when stop is canceled are dropped.
func (t *tailer) tail(ctx, stop context.Context) {
	t.running.Store(true)
	defer t.running.Store(false)
	t.logger.Info("starting tailing container")

	info, err := t.client.ContainerInspect(ctx, t.containerID, client.ContainerInspectOptions{})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		t.logger.Error("could not inspect container info", "err", err)
		t.err.Store(err)
		return
	}

	reader, err := t.client.ContainerLogs(ctx, t.containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Since:      strconv.FormatInt(t.since.Load(), 10),
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		t.logger.Error("could not fetch logs for container", "err", err)
		t.err.Store(err)
		return
	}

	t.processLoop(ctx, stop, info.Container.Config.Tty, reader)
	t.logger.Info("stopped tailing container")
}

func (t *tailer) Key() positions.Entry {
	return positions.Entry{
		Path:   t.containerID,
		Labels: t.labelsStr,
	}
}

func (t *tailer) DebugInfo() sourceInfo {
	var (
		errMsg string
		err    = t.err.Load()
	)

	if err != nil {
		errMsg = err.Error()
	}

	return sourceInfo{
		ID:         t.containerID,
		LastError:  errMsg,
		Labels:     t.labelsStr,
		IsRunning:  t.running.Load(),
		ReadOffset: t.positions.GetString(positions.CursorKey(t.containerID), t.labelsStr),
	}
}

// processLoop reads reader until it ends or ctx is canceled.
func (t *tailer) processLoop(ctx, stop context.Context, tty bool, reader io.ReadCloser) {
	defer reader.Close()
	stopClose := context.AfterFunc(ctx, func() { reader.Close() })
	defer stopClose()

	rstdout, wstdout := io.Pipe()
	rstderr, wstderr := io.Pipe()

	var wg sync.WaitGroup

	wg.Go(func() {
		defer func() {
			wstdout.Close()
			wstderr.Close()
		}()

		var (
			err     error
			written int64
		)

		if tty {
			written, err = io.Copy(wstdout, reader)
		} else {
			// For non-TTY, wrap the pipe writers with our chunk writer to reassemble frames.
			wcstdout := newChunkWriter(wstdout, t.logger)
			defer wcstdout.Close()
			wcstderr := newChunkWriter(wstderr, t.logger)
			defer wcstderr.Close()
			written, err = stdcopy.StdCopy(wcstdout, wcstderr, reader)
		}

		if err != nil {
			t.logger.Warn("could not transfer logs", "written", written, "err", err)
		} else {
			t.logger.Info("finished transferring logs", "written", written)
		}
	})

	wg.Go(func() {
		defer rstdout.Close()
		t.processStream(stop, rstdout, t.getStreamLabels("stdout"))
	})
	wg.Go(func() {
		defer rstderr.Close()
		t.processStream(stop, rstderr, t.getStreamLabels("stderr"))
	})

	wg.Wait()
	t.logger.Debug("done processing Docker logs")
}

func (t *tailer) processStream(stop context.Context, r io.Reader, logStreamLset model.LabelSet) {
	const (
		// initialBufSize is the size that bufio.Scanner would allocate without a Buffer call.
		initialBufSize = 4096
		maxCapacity    = dockerMaxChunkSize * 64
	)

	reader := bufio.NewReader(r)
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, initialBufSize)
	scanner.Buffer(buf, maxCapacity)

	for {
		if !scanner.Scan() {
			err := scanner.Err()
			// We got EOF and should stop scanning.
			if err == nil {
				return
			}

			if errors.Is(err, bufio.ErrTooLong) {
				t.logger.Error("line too big, skipping")
				err = skipUntilNewline(reader)
			}

			if err != nil {
				// No more to read after we skipped to newline so we should stop.
				if errors.Is(err, io.EOF) {
					return
				}
				// An unexpected error and we stop reading.
				t.logger.Error("error reading docker log line", "err", err)
				t.metrics.dockerErrors.Inc()
			}

			scanner = bufio.NewScanner(reader)
			scanner.Buffer(buf[:0], maxCapacity)
			continue
		}

		ts, content, err := extractTsFromBytes(scanner.Bytes())
		if err != nil {
			t.logger.Error("could not extract timestamp, skipping line", "err", err)
			t.metrics.dockerErrors.Inc()
			continue
		}

		if len(content) == 0 {
			t.logger.Debug("empty log, skipping line")

			continue
		}

		select {
		case t.recv.Chan() <- loki.NewEntry(logStreamLset, push.Entry{
			Timestamp: ts,
			Line:      string(content),
		}):
		case <-stop.Done():
			return
		}
		t.metrics.dockerEntries.Inc()

		// NOTE(@tpaschalis) We don't save the positions entry with the
		// filtered labels, but with the default label set, as this is the one
		// used to find the original read offset from the client. This might be
		// problematic if we have the same container with a different set of
		// labels (e.g. duplicated and relabeled), but this shouldn't be the
		// case anyway.
		t.positions.Put(positions.CursorKey(t.containerID), t.labelsStr, ts.Unix())
		t.since.Store(ts.Unix())
		t.last.Store(time.Now().Unix())
	}
}

func (t *tailer) getStreamLabels(logStream string) model.LabelSet {
	// Add all labels from the config, relabel and filter them.
	lb := labels.NewBuilder(labels.EmptyLabels())
	for k, v := range t.labels {
		lb.Set(string(k), string(v))
	}

	lb.Set(dockerLabelLogStream, logStream)
	processed := labels.EmptyLabels()
	if relabel.ProcessBuilder(lb, t.relabelConfig...) {
		processed = lb.Labels()
	}

	filtered := make(model.LabelSet)
	processed.Range(func(lbl labels.Label) {
		if strings.HasPrefix(lbl.Name, "__") {
			return
		}
		filtered[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})

	return filtered
}

// extractTsFromBytes parses an RFC3339Nano timestamp from the byte slice.
func extractTsFromBytes(line []byte) (time.Time, []byte, error) {
	spaceIdx := bytes.IndexByte(line, ' ')
	if spaceIdx == -1 || spaceIdx >= len(line) {
		return time.Time{}, nil, fmt.Errorf("could not find timestamp in bytes")
	}

	// The unsafe.String is used here to avoid allocation and string conversion when parsing the timestamp.
	// This is safe because time.Parse doesn't retain the string after returning and
	// the underlying bytes aren't modified during parsing.
	ts, err := time.Parse(time.RFC3339Nano, unsafe.String(&line[0], spaceIdx))
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("could not parse timestamp: %w", err)
	}
	return ts, line[spaceIdx+1:], nil
}

func skipUntilNewline(r *bufio.Reader) error {
	for {
		_, err := r.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return err
	}
}

// dockerChunkWriter implements io.Writer to preprocess and reassemble Docker log frames.
type dockerChunkWriter struct {
	writer      io.Writer
	logger      *slog.Logger
	buffer      *bytes.Buffer
	isBuffering bool
}

var bufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, dockerMaxChunkSize*2))
	},
}

func newChunkWriter(writer io.Writer, logger *slog.Logger) *dockerChunkWriter {
	return &dockerChunkWriter{
		writer: writer,
		logger: logger,
		buffer: bufferPool.Get().(*bytes.Buffer),
	}
}

func (fw *dockerChunkWriter) Close() error {
	if fw.buffer != nil {
		fw.buffer.Reset()
		bufferPool.Put(fw.buffer)
		fw.buffer = nil
	}
	return nil
}

func (fw *dockerChunkWriter) Write(p []byte) (int, error) {
	if !fw.isBuffering {
		if len(p) < dockerMaxChunkSize || p[len(p)-1] == 0x0A {
			// Short or complete frame: write directly without buffering.
			return fw.writer.Write(p)
		}
		// Long frame start: buffer the first chunk.
		fw.buffer.Write(p)
		fw.isBuffering = true
		return len(p), nil
	}

	// Continuation chunk: strip redundant timestamp and append content.
	_, content, err := extractTsFromBytes(p)
	if err != nil {
		fw.logger.Warn("could not strip timestamp from continuation chunk", "err", err)
		fw.buffer.Write(p)
	} else {
		fw.buffer.Write(content)
	}
	// If this is the last continuation chunk (ends with newline), flush the buffer
	if len(p) > 0 && p[len(p)-1] == 0x0A {
		if _, err := fw.writer.Write(fw.buffer.Bytes()); err != nil {
			return 0, err
		}
		fw.buffer.Reset()
		fw.isBuffering = false
	}
	return len(p), nil
}
