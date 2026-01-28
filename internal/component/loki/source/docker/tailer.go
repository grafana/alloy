package docker

// NOTE: This code is adapted from Promtail (90a1d4593e2d690b37333386383870865fe177bf).
// The dockertarget package is used to configure and run the targets that can
// read logs from Docker containers and forward them to other loki components.

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/go-kit/log"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// tailer for Docker container logs.
type tailer struct {
	logger            log.Logger
	recv              loki.LogsReceiver
	positions         positions.Positions
	containerID       string
	labels            model.LabelSet
	labelsStr         string
	relabelConfig     []*relabel.Config
	metrics           *metrics
	restartInterval   time.Duration
	componentStopping func() bool

	client client.APIClient

	mu      sync.Mutex // protects cancel, running and err fields
	err     error
	running bool
	cancel  context.CancelFunc

	wg sync.WaitGroup

	last  *atomic.Int64
	since *atomic.Int64
}

// newTailer starts a new tailer to read logs from a given container ID.
func newTailer(
	metrics *metrics, logger log.Logger, recv loki.LogsReceiver, position positions.Positions, containerID string,
	labels model.LabelSet, relabelConfig []*relabel.Config, client client.APIClient, restartInterval time.Duration,
	componentStopping func() bool,
) (*tailer, error) {

	labelsStr := labels.String()
	pos, err := position.Get(positions.CursorKey(containerID), labelsStr)
	if err != nil {
		return nil, err
	}

	return &tailer{
		logger:            logger,
		recv:              recv,
		since:             atomic.NewInt64(pos),
		last:              atomic.NewInt64(0),
		positions:         position,
		containerID:       containerID,
		labels:            labels,
		labelsStr:         labelsStr,
		relabelConfig:     relabelConfig,
		metrics:           metrics,
		client:            client,
		restartInterval:   restartInterval,
		componentStopping: componentStopping,
	}, nil
}

func (t *tailer) Run(ctx context.Context) {
	ticker := time.NewTicker(t.restartInterval)
	defer ticker.Stop()

	// start on initial call to Run.
	t.startIfNotRunning()

	for {
		select {
		case <-ticker.C:
			res, err := t.client.ContainerInspect(ctx, t.containerID)
			if err != nil {
				level.Error(t.logger).Log("msg", "error inspecting Docker container", "id", t.containerID, "error", err)
				continue
			}

			finished, err := time.Parse(time.RFC3339Nano, res.State.FinishedAt)
			if err != nil {
				level.Error(t.logger).Log("msg", "error parsing finished time for Docker container", "id", t.containerID, "error", err)
				finished = time.Unix(0, 0)
			}

			if res.State.Running || finished.Unix() >= t.last.Load() {
				t.startIfNotRunning()
			}
		case <-ctx.Done():
			t.stop()
			return
		}
	}
}

// startIfNotRunning starts processing container logs. The operation is idempotent, i.e. the processing cannot be started twice.
func (t *tailer) startIfNotRunning() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.running {
		level.Debug(t.logger).Log("msg", "starting process loop", "container", t.containerID)

		ctx := context.Background()
		info, err := t.client.ContainerInspect(ctx, t.containerID)
		if err != nil {
			level.Error(t.logger).Log("msg", "could not inspect container info", "container", t.containerID, "err", err)
			t.err = err
			return
		}

		reader, err := t.client.ContainerLogs(ctx, t.containerID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Timestamps: true,
			Since:      strconv.FormatInt(t.since.Load(), 10),
		})
		if err != nil {
			level.Error(t.logger).Log("msg", "could not fetch logs for container", "container", t.containerID, "err", err)
			t.err = err
			return
		}

		ctx, cancel := context.WithCancel(ctx)
		t.cancel = cancel
		t.running = true
		// processLoop will start 3 goroutines that we need to wait for if Stop is called.
		t.wg.Add(3)
		go t.processLoop(ctx, info.Config.Tty, reader)
	}
}

// stop shuts down the target.
func (t *tailer) stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running {
		t.running = false
		if t.cancel != nil {
			t.cancel()
		}
		t.wg.Wait()
		level.Debug(t.logger).Log("msg", "stopped Docker target", "container", t.containerID)

		// If the component is not stopping, then it means that the target for this component is gone and that
		// we should clear the entry from the positions file.
		if !t.componentStopping() {
			t.positions.Remove(positions.CursorKey(t.containerID), t.labelsStr)
		}
	}
}

func (t *tailer) Key() string {
	return t.containerID
}

func (t *tailer) DebugInfo() sourceInfo {
	t.mu.Lock()
	defer t.mu.Unlock()
	running := t.running

	var errMsg string
	if t.err != nil {
		errMsg = t.err.Error()
	}

	return sourceInfo{
		ID:         t.containerID,
		LastError:  errMsg,
		Labels:     t.labelsStr,
		IsRunning:  running,
		ReadOffset: t.positions.GetString(positions.CursorKey(t.containerID), t.labelsStr),
	}
}

func (t *tailer) processLoop(ctx context.Context, tty bool, reader io.ReadCloser) {
	defer reader.Close()

	// Start transferring
	rstdout, wstdout := io.Pipe()
	rstderr, wstderr := io.Pipe()
	go func() {
		defer func() {
			t.wg.Done()
			wstdout.Close()
			wstderr.Close()
			t.stop()
		}()
		var written int64
		var err error
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
			level.Warn(t.logger).Log("msg", "could not transfer logs", "written", written, "container", t.containerID, "err", err)
		} else {
			level.Info(t.logger).Log("msg", "finished transferring logs", "written", written, "container", t.containerID)
		}
	}()

	// Start processing
	go t.process(rstdout, t.getStreamLabels("stdout"))
	go t.process(rstderr, t.getStreamLabels("stderr"))

	// Wait until done
	<-ctx.Done()
	level.Debug(t.logger).Log("msg", "done processing Docker logs", "container", t.containerID)
}

// extractTsFromBytes parses an RFC3339Nano timestamp from the byte slice.
func extractTsFromBytes(line []byte) (time.Time, []byte, error) {
	const timestampLayout = "2006-01-02T15:04:05.999999999Z07:00"

	spaceIdx := bytes.IndexByte(line, ' ')
	if spaceIdx == -1 || spaceIdx >= len(line)-1 {
		return time.Time{}, nil, fmt.Errorf("could not find timestamp in bytes")
	}

	// The unsafe.String is used here to avoid allocation and string conversion when parsing the timestamp
	// This is safe because:
	// 1. spaceIdx > 0 and spaceIdx < len(line)-1 is guaranteed by the check above
	// 2. time.Parse doesn't retain the string after returning
	// 3. The underlying bytes aren't modified during parsing
	ts, err := time.Parse(timestampLayout, unsafe.String(&line[0], spaceIdx))
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("could not parse timestamp: %w", err)
	}
	return ts, line[spaceIdx+1:], nil
}

func (t *tailer) process(r io.Reader, logStreamLset model.LabelSet) {
	defer t.wg.Done()

	scanner := bufio.NewScanner(r)
	const maxCapacity = dockerMaxChunkSize * 64
	buf := make([]byte, 0, maxCapacity)
	scanner.Buffer(buf, maxCapacity)
	for scanner.Scan() {
		line := scanner.Bytes()

		ts, content, err := extractTsFromBytes(line)
		if err != nil {
			level.Error(t.logger).Log("msg", "could not extract timestamp, skipping line", "err", err)
			t.metrics.dockerErrors.Inc()
			continue
		}

		t.recv.Chan() <- loki.Entry{
			Labels: logStreamLset,
			Entry: push.Entry{
				Timestamp: ts,
				Line:      string(content),
			},
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
	if err := scanner.Err(); err != nil {
		level.Error(t.logger).Log("msg", "error reading docker log line", "err", err)
		t.metrics.dockerErrors.Inc()
	}
}

func (t *tailer) getStreamLabels(logStream string) model.LabelSet {
	// Add all labels from the config, relabel and filter them.
	lb := labels.NewBuilder(labels.EmptyLabels())
	for k, v := range t.labels {
		lb.Set(string(k), string(v))
	}
	lb.Set(dockerLabelLogStream, logStream)
	processed, _ := relabel.Process(lb.Labels(), t.relabelConfig...)

	filtered := make(model.LabelSet)
	processed.Range(func(lbl labels.Label) {
		if strings.HasPrefix(lbl.Name, "__") {
			return
		}
		filtered[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})

	return filtered
}

// dockerChunkWriter implements io.Writer to preprocess and reassemble Docker log frames.
type dockerChunkWriter struct {
	writer      io.Writer
	logger      log.Logger
	buffer      *bytes.Buffer
	isBuffering bool
}

var bufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, dockerMaxChunkSize*2))
	},
}

func newChunkWriter(writer io.Writer, logger log.Logger) *dockerChunkWriter {
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
		// Should not normally happen, but flog.log has entries like this for some reason.
		level.Warn(fw.logger).Log("msg", "could not strip timestamp from continuation chunk", "err", err)
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
