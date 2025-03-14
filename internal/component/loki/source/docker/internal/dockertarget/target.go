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
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/grafana/alloy/internal/service/logging/level"
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
	since         int64
	positions     positions.Positions
	containerName string
	labels        model.LabelSet
	labelsStr     string
	relabelConfig []*relabel.Config
	metrics       *Metrics

	cancel  context.CancelFunc
	client  client.APIClient
	wg      sync.WaitGroup
	running *atomic.Bool
	err     error
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
		since:         since,
		positions:     position,
		containerName: containerID,
		labels:        labels,
		labelsStr:     labelsStr,
		relabelConfig: relabelConfig,
		metrics:       metrics,

		client:  client,
		running: atomic.NewBool(false),
	}

	// NOTE (@tpaschalis) The original Promtail implementation would call
	// t.StartIfNotRunning() right here to start tailing.
	// We manage targets from a task's Run method.
	return t, nil
}

func (t *Target) processLoop(ctx context.Context) {
	t.running.Store(true)
	defer t.running.Store(false)

	t.wg.Add(1)
	defer t.wg.Done()

	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Since:      strconv.FormatInt(t.since, 10),
	}
	inspectInfo, err := t.client.ContainerInspect(ctx, t.containerName)
	if err != nil {
		level.Error(t.logger).Log("msg", "could not inspect container info", "container", t.containerName, "err", err)
		t.err = err
		return
	}
	logs, err := t.client.ContainerLogs(ctx, t.containerName, opts)
	if err != nil {
		level.Error(t.logger).Log("msg", "could not fetch logs for container", "container", t.containerName, "err", err)
		t.err = err
		return
	}

	// Start transferring
	rstdout, wstdout := io.Pipe()
	rstderr, wstderr := io.Pipe()
	t.wg.Add(1)
	go func() {
		defer func() {
			t.wg.Done()
			wstdout.Close()
			wstderr.Close()
			t.Stop()
		}()
		var written int64
		var err error
		if inspectInfo.Config.Tty {
			written, err = io.Copy(wstdout, logs)
		} else {
			written, err = stdcopy.StdCopy(wstdout, wstderr, logs)
		}
		if err != nil {
			level.Warn(t.logger).Log("msg", "could not transfer logs", "written", written, "container", t.containerName, "err", err)
		} else {
			level.Info(t.logger).Log("msg", "finished transferring logs", "written", written, "container", t.containerName)
		}
	}()

	// Start processing
	t.wg.Add(2)
	go t.process(rstdout, t.getStreamLabels("stdout"))
	go t.process(rstderr, t.getStreamLabels("stderr"))

	// Wait until done
	<-ctx.Done()
	logs.Close()
	level.Debug(t.logger).Log("msg", "done processing Docker logs", "container", t.containerName)
}

// extractTs tries for read the timestamp from the beginning of the log line.
// It's expected to follow the format 2006-01-02T15:04:05.999999999Z07:00.
func extractTs(line string) (time.Time, string, error) {
	pair := strings.SplitN(line, " ", 2)
	if len(pair) != 2 {
		return time.Now(), line, fmt.Errorf("Could not find timestamp in '%s'", line)
	}
	ts, err := time.Parse("2006-01-02T15:04:05.999999999Z07:00", pair[0])
	if err != nil {
		return time.Now(), line, fmt.Errorf("Could not parse timestamp from '%s': %w", pair[0], err)
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
			Entry: logproto.Entry{
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
		t.since = ts.Unix()
	}
}

// StartIfNotRunning starts processing container logs. The operation is idempotent , i.e. the processing cannot be started twice.
func (t *Target) StartIfNotRunning() {
	if t.running.CompareAndSwap(false, true) {
		level.Debug(t.logger).Log("msg", "starting process loop", "container", t.containerName)
		ctx, cancel := context.WithCancel(context.Background())
		t.cancel = cancel
		go t.processLoop(ctx)
	}
}

// Stop shuts down the target.
func (t *Target) Stop() {
	if t.Ready() {
		t.cancel()
		t.wg.Wait()
		level.Debug(t.logger).Log("msg", "stopped Docker target", "container", t.containerName)
	}
}

// Ready reports whether the target is running.
func (t *Target) Ready() bool {
	return t.running.Load()
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

// Details returns target-specific details.
func (t *Target) Details() map[string]string {
	var errMsg string
	if t.err != nil {
		errMsg = t.err.Error()
	}
	return map[string]string{
		"id":       t.containerName,
		"error":    errMsg,
		"position": t.positions.GetString(positions.CursorKey(t.containerName), t.labelsStr),
		"running":  strconv.FormatBool(t.running.Load()),
	}
}

func (t *Target) getStreamLabels(logStream string) model.LabelSet {
	// Add all labels from the config, relabel and filter them.
	lb := labels.NewBuilder(nil)
	for k, v := range t.labels {
		lb.Set(string(k), string(v))
	}
	lb.Set(dockerLabelLogStream, logStream)
	processed, _ := relabel.Process(lb.Labels(), t.relabelConfig...)

	filtered := make(model.LabelSet)
	for _, lbl := range processed {
		if strings.HasPrefix(lbl.Name, "__") {
			continue
		}
		filtered[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	}

	return filtered
}
