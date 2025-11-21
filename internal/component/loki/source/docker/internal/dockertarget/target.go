package dockertarget

// NOTE: This code is adapted from Promtail (90a1d4593e2d690b37333386383870865fe177bf).
// The dockertarget package is used to configure and run the targets that can
// read logs from Docker containers and forward them to other loki components.

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

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

const (
	// See github.com/prometheus/prometheus/discovery/moby
	dockerLabel                = model.MetaLabelPrefix + "docker_"
	dockerLabelContainerPrefix = dockerLabel + "container_"
	dockerLabelLogStream       = dockerLabelContainerPrefix + "log_stream"
)

// Target enables reading Docker container logs.
type Target struct {
	logger        log.Logger
	handler       loki.EntryHandler
	positions     positions.Positions
	containerName string
	labels        model.LabelSet
	labelsStr     string
	relabelConfig []*relabel.Config
	metrics       *Metrics

	client client.APIClient

	mu      sync.Mutex // protects cancel, running and err fields
	err     error
	running bool
	cancel  context.CancelFunc

	wg sync.WaitGroup

	last  *atomic.Int64
	since *atomic.Int64
}

// NewTarget starts a new target to read logs from a given container ID.
func NewTarget(metrics *Metrics, logger log.Logger, handler loki.EntryHandler, position positions.Positions, containerID string, labels model.LabelSet, relabelConfig []*relabel.Config, client client.APIClient) (*Target, error) {
	labelsStr := labels.String()
	pos, err := position.Get(positions.CursorKey(containerID), labelsStr)
	if err != nil {
		return nil, err
	}
	var since int64
	if pos != 0 {
		since = pos
	}

	t := &Target{
		logger:        logger,
		handler:       handler,
		since:         atomic.NewInt64(since),
		last:          atomic.NewInt64(0),
		positions:     position,
		containerName: containerID,
		labels:        labels,
		labelsStr:     labelsStr,
		relabelConfig: relabelConfig,
		metrics:       metrics,
		client:        client,
	}

	// NOTE (@tpaschalis) The original Promtail implementation would call
	// t.StartIfNotRunning() right here to start tailing.
	// We manage targets from a task's Run method.
	return t, nil
}

func (t *Target) processLoop(ctx context.Context, tty bool, reader io.ReadCloser) {
	defer reader.Close()

	// Start transferring
	rstdout, wstdout := io.Pipe()
	rstderr, wstderr := io.Pipe()
	go func() {
		defer func() {
			t.wg.Done()
			wstdout.Close()
			wstderr.Close()
			t.Stop()
		}()
		var written int64
		var err error
		if tty {
			written, err = io.Copy(wstdout, reader)
		} else {
			written, err = stdcopy.StdCopy(wstdout, wstderr, reader)
		}
		if err != nil {
			level.Warn(t.logger).Log("msg", "could not transfer logs", "written", written, "container", t.containerName, "err", err)
		} else {
			level.Info(t.logger).Log("msg", "finished transferring logs", "written", written, "container", t.containerName)
		}
	}()

	// Start processing
	go t.process(rstdout, t.getStreamLabels("stdout"))
	go t.process(rstderr, t.getStreamLabels("stderr"))

	// Wait until done
	<-ctx.Done()
	level.Debug(t.logger).Log("msg", "done processing Docker logs", "container", t.containerName)
}

// extractTs tries for read the timestamp from the beginning of the log line.
// It's expected to follow the format 2006-01-02T15:04:05.999999999Z07:00.
func extractTs(line string) (time.Time, string, error) {
	pair := strings.SplitN(line, " ", 2)
	if len(pair) != 2 {
		return time.Now(), line, fmt.Errorf("could not find timestamp in '%s'", line)
	}
	ts, err := time.Parse("2006-01-02T15:04:05.999999999Z07:00", pair[0])
	if err != nil {
		return time.Now(), line, fmt.Errorf("could not parse timestamp from '%s': %w", pair[0], err)
	}
	return ts, pair[1], nil
}

// https://devmarkpro.com/working-big-files-golang
func readLine(r *bufio.Reader) (string, error) {
	var (
		isPrefix = true
		err      error
		line, ln []byte
	)

	for isPrefix && err == nil {
		line, isPrefix, err = r.ReadLine()
		ln = append(ln, line...)
	}

	return string(ln), err
}

func (t *Target) process(r io.Reader, logStreamLset model.LabelSet) {
	defer func() {
		t.wg.Done()
	}()

	reader := bufio.NewReader(r)
	for {
		line, err := readLine(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			level.Error(t.logger).Log("msg", "error reading docker log line, skipping line", "err", err)
			t.metrics.dockerErrors.Inc()
		}

		ts, line, err := extractTs(line)
		if err != nil {
			level.Error(t.logger).Log("msg", "could not extract timestamp, skipping line", "err", err)
			t.metrics.dockerErrors.Inc()
			continue
		}

		t.handler.Chan() <- loki.Entry{
			Labels: logStreamLset,
			Entry: push.Entry{
				Timestamp: ts,
				Line:      line,
			},
		}
		t.metrics.dockerEntries.Inc()

		// NOTE(@tpaschalis) We don't save the positions entry with the
		// filtered labels, but with the default label set, as this is the one
		// used to find the original read offset from the client. This might be
		// problematic if we have the same container with a different set of
		// labels (e.g. duplicated and relabeled), but this shouldn't be the
		// case anyway.
		t.positions.Put(positions.CursorKey(t.containerName), t.labelsStr, ts.Unix())
		t.since.Store(ts.Unix())
		t.last.Store(time.Now().Unix())
	}
}

// StartIfNotRunning starts processing container logs. The operation is idempotent , i.e. the processing cannot be started twice.
func (t *Target) StartIfNotRunning() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.running {
		level.Debug(t.logger).Log("msg", "starting process loop", "container", t.containerName)

		ctx := context.Background()
		info, err := t.client.ContainerInspect(ctx, t.containerName)
		if err != nil {
			level.Error(t.logger).Log("msg", "could not inspect container info", "container", t.containerName, "err", err)
			t.err = err
			return
		}

		reader, err := t.client.ContainerLogs(ctx, t.containerName, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Timestamps: true,
			Since:      strconv.FormatInt(t.since.Load(), 10),
		})
		if err != nil {
			level.Error(t.logger).Log("msg", "could not fetch logs for container", "container", t.containerName, "err", err)
			t.err = err
			return
		}

		ctx, cancel := context.WithCancel(ctx)
		t.cancel = cancel
		t.running = true
		// proccessLoop will start 3 goroutines that we need to wait for if Stop is called.
		t.wg.Add(3)
		go t.processLoop(ctx, info.Config.Tty, reader)
	}
}

// Stop shuts down the target.
func (t *Target) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running {
		t.running = false
		if t.cancel != nil {
			t.cancel()
		}
		t.wg.Wait()
		level.Debug(t.logger).Log("msg", "stopped Docker target", "container", t.containerName)
	}
}

// Ready reports whether the target is running.
func (t *Target) Ready() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

// LabelsStr returns the target's original labels string representation.
func (t *Target) LabelsStr() string {
	return t.labelsStr
}

// Name reports the container name.
func (t *Target) Name() string {
	return t.containerName
}

// Hash is used when comparing targets in tasks.
func (t *Target) Hash() uint64 {
	return uint64(t.labels.Fingerprint())
}

// Path returns the target's container name.
func (t *Target) Path() string {
	return t.containerName
}

// Last returns the unix timestamp of the target's last processing loop.
func (t *Target) Last() int64 { return t.last.Load() }

// Details returns target-specific details.
func (t *Target) Details() map[string]string {
	t.mu.Lock()
	running := t.running

	var errMsg string
	if t.err != nil {
		errMsg = t.err.Error()
	}
	t.mu.Unlock()

	return map[string]string{
		"id":       t.containerName,
		"error":    errMsg,
		"position": t.positions.GetString(positions.CursorKey(t.containerName), t.labelsStr),
		"running":  strconv.FormatBool(running),
	}
}

func (t *Target) getStreamLabels(logStream string) model.LabelSet {
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
