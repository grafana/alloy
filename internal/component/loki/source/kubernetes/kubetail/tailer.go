package kubetail

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-kit/log"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/loki/v3/pkg/logproto"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runner"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// tailerTask is the payload used to create tailers. It implements runner.Task.
type tailerTask struct {
	Options *Options
	Target  *Target
}

var _ runner.Task = (*tailerTask)(nil)

const maxTailerLifetime = 1 * time.Hour

func (tt *tailerTask) Hash() uint64 { return tt.Target.Hash() }

func (tt *tailerTask) Equals(other runner.Task) bool {
	otherTask := other.(*tailerTask)

	// Quick path: pointers are exactly the same.
	if tt == otherTask {
		return true
	}

	// Slow path: check individual fields which are part of the task.
	return tt.Options == otherTask.Options &&
		tt.Target.UID() == otherTask.Target.UID() &&
		labels.Equal(tt.Target.Labels(), otherTask.Target.Labels())
}

// A tailer tails the logs of a Kubernetes container. It is created by a
// [Manager].
type tailer struct {
	log    log.Logger
	opts   *Options
	target *Target

	lset model.LabelSet
}

var _ runner.Worker = (*tailer)(nil)

// newTailer returns a new Tailer which tails logs from the target specified by
// the task.
func newTailer(l log.Logger, task *tailerTask) *tailer {
	return &tailer{
		log:    log.WithPrefix(l, "target", task.Target.String()),
		opts:   task.Options,
		target: task.Target,

		lset: newLabelSet(task.Target.Labels()),
	}
}

func newLabelSet(l labels.Labels) model.LabelSet {
	res := make(model.LabelSet, len(l))
	for _, pair := range l {
		res[model.LabelName(pair.Name)] = model.LabelValue(pair.Value)
	}
	return res
}

var retailBackoff = backoff.Config{
	// Since our tailers have a maximum lifetime and are expected to regularly
	// terminate to refresh their connection to the Kubernetes API, the minimum
	// backoff starts at 10ms so there's a small delay between expected
	// terminations.
	MinBackoff: 10 * time.Millisecond,
	MaxBackoff: time.Minute,
}

func (t *tailer) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	level.Info(t.log).Log("msg", "tailer running")
	defer level.Info(t.log).Log("msg", "tailer exited")

	bo := backoff.New(ctx, retailBackoff)

	handler := loki.NewEntryMutatorHandler(t.opts.Handler, func(e loki.Entry) loki.Entry {
		// A log line got read, we can reset the backoff period now.
		bo.Reset()
		return e
	})
	defer handler.Stop()

	for bo.Ongoing() {
		err := t.tail(ctx, handler)
		if err == nil {
			terminated, err := t.containerTerminated(ctx)
			if terminated {
				// The container shut down and won't come back; we can stop tailing it.
				return
			} else if err != nil {
				level.Warn(t.log).Log("msg", "could not determine if container terminated; will retry tailing", "err", err)
			}
		}

		if err != nil {
			t.target.Report(time.Now().UTC(), err)
			level.Warn(t.log).Log("msg", "tailer stopped; will retry", "err", err)
		}
		bo.Wait()
	}
}

func (t *tailer) tail(ctx context.Context, handler loki.EntryHandler) error {
	// Set a maximum lifetime of the tail to ensure that connections are
	// reestablished. This avoids an issue where the Kubernetes API server stops
	// responding with new logs while the connection is kept open.
	ctx, cancel := context.WithTimeout(ctx, maxTailerLifetime)
	defer cancel()

	var (
		key           = t.target.NamespacedName()
		containerName = t.target.ContainerName()

		positionsEnt = entryForTarget(t.target)
	)

	var lastReadTime time.Time

	if offset, err := t.opts.Positions.Get(positionsEnt.Path, positionsEnt.Labels); err != nil {
		level.Warn(t.log).Log("msg", "failed to load last read offset", "err", err)
	} else {
		lastReadTime = time.UnixMicro(offset)
	}

	// If the last entry for our target is after the positions cache, use that
	// instead.
	if lastEntry := t.target.LastEntry(); lastEntry.After(lastReadTime) {
		lastReadTime = lastEntry
	}

	var offsetTime *metav1.Time
	if !lastReadTime.IsZero() {
		offsetTime = &metav1.Time{Time: lastReadTime}
	}

	req := t.opts.Client.CoreV1().Pods(key.Namespace).GetLogs(key.Name, &corev1.PodLogOptions{
		Follow:     true,
		Container:  containerName,
		SinceTime:  offsetTime,
		Timestamps: true, // Should be forced to true so we can parse the original timestamp back out.
	})

	//TODO: Write a test with a fake req. Make sure Close() was called.
	stream, err := req.Stream(context.Background())
	if err != nil {
		return err
	}
	defer stream.Close()

	//TODO: The make some sort of interface to retrieve the k8s version.
	// That way we can mock it in tests.
	// Unfortunately, we can't set a version for the fake k8s API to return.
	//
	// k8sServerVersion, err := t.opts.Client.Discovery().ServerVersion()
	// if err != nil {
	// 	return err
	// }
	// k8sComparableServerVersion, err := semver.ParseTolerant(k8sServerVersion.GitVersion)
	// if err != nil {
	// 	return err
	// }
	k8sComparableServerVersion := semver.Version{
		Major: 1,
		Minor: 20,
	}

	// Create a new rolling average calculator to determine the average delta
	// time between log entries.
	//
	// Here, we track the most recent 10,000 delta times to compute a fairly
	// accurate average. If there are less than 100 deltas stored, the average
	// time defaults to 1h.
	//
	// The computed average will never be less than the minimum of 2s.
	calc := newRollingAverageCalculator(10000, 100, 2*time.Second, maxTailerLifetime)

	// Versions of Kubernetes which do not contain
	// kubernetes/kubernetes#115702 (<= v1.29.1) will fail to detect rotated log files
	// and stop sending logs to us.
	//
	// To work around this, we use a rolling average to determine how
	// frequent we usually expect to see entries. If 3x the normal delta has
	// elapsed, we'll restart the tailer.
	//
	// False positives here are acceptable, but false negatives mean that
	// we'll have a larger spike of missing logs until we detect a rolled
	// file.
	if k8sComparableServerVersion.LT(semver.Version{Major: 1, Minor: 29, Patch: 0}) {
		go func() {
			rolledFileTicker := time.NewTicker(1 * time.Second)
			defer func() {
				rolledFileTicker.Stop()
			}()
			for {
				select {
				case <-ctx.Done():
					return
				case <-rolledFileTicker.C:
					avg := calc.GetAverage()
					last := calc.GetLast()
					if last.IsZero() {
						continue
					}
					s := time.Since(last)
					if s > avg*3 {
						level.Debug(t.log).Log("msg", "have not seen a log line in 3x average time between lines, closing and re-opening tailer", "rolling_average", avg, "time_since_last", s)

						cancel()
						return
					}
				}
			}
		}()
	}

	level.Info(t.log).Log("msg", "opened log stream", "start time", lastReadTime)

	ch := handler.Chan()
	reader := bufio.NewReader(stream)

	for {
		line, err := reader.ReadString('\n')

		// Try processing the line before handling the error, since data may still
		// be returned alongside an EOF.
		if len(line) != 0 {
			calc.AddTimestamp(time.Now())

			entryTimestamp, entryLine := parseKubernetesLog(line)
			if !entryTimestamp.After(lastReadTime) {
				continue
			}
			lastReadTime = entryTimestamp

			entry := loki.Entry{
				Labels: t.lset.Clone(),
				Entry: logproto.Entry{
					Timestamp: entryTimestamp,
					Line:      entryLine,
				},
			}

			select {
			case <-ctx.Done():
				return nil
			case ch <- entry:
				// Save position after it's been sent over the channel.
				t.opts.Positions.Put(positionsEnt.Path, positionsEnt.Labels, entryTimestamp.UnixMicro())
				t.target.Report(entryTimestamp, nil)
			}
		}

		// Return an error if our stream closed. The caller will reopen the tailer
		// forever until our tailer is closed.
		//
		// Even if EOF is returned, we still want to allow the tailer to retry
		// until the tailer is shutdown; EOF being returned doesn't necessarily
		// indicate that the logs are done, and could point to a brief network
		// outage.
		if err != nil && (errors.Is(err, io.EOF) || ctx.Err() != nil) {
			return nil
		} else if err != nil {
			return err
		}
	}
}

// containerTerminated determines whether the container this tailer was
// watching has terminated and won't restart. If containerTerminated returns
// true, it means that no more logs will appear for the watched target.
func (t *tailer) containerTerminated(ctx context.Context) (terminated bool, err error) {
	var (
		key           = t.target.NamespacedName()
		containerName = t.target.ContainerName()
	)

	podInfo, err := t.opts.Client.CoreV1().Pods(key.Namespace).Get(ctx, key.Name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	// The pod UID is different than the one we were tailing; our UID has
	// terminated.
	if podInfo.GetUID() != kubetypes.UID(t.target.UID()) {
		return true, nil
	}

	containerInfo, containerType, found := findContainerStatus(podInfo, containerName)
	if !found {
		return false, fmt.Errorf("could not find container %q in pod status", containerName)
	}

	restartPolicy := podInfo.Spec.RestartPolicy

	switch containerType {
	case containerTypeApp:
		// An app container will only restart if:
		//
		// * It is in a waiting (meaning it's waiting to run) or running state
		//   (meaning it already restarted before we had a chance to check)
		// * It terminated with any exit code and restartPolicy is Always
		// * It terminated with non-zero exit code and restartPolicy is not Never
		//
		// https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#restart-policy
		switch {
		case containerInfo.State.Waiting != nil || containerInfo.State.Running != nil:
			return false, nil // Container will restart
		case containerInfo.State.Terminated != nil && restartPolicy == corev1.RestartPolicyAlways:
			return false, nil // Container will restart
		case containerInfo.State.Terminated != nil && containerInfo.State.Terminated.ExitCode != 0 && restartPolicy != corev1.RestartPolicyNever:
			return false, nil // Container will restart
		default:
			return true, nil // Container will *not* restart
		}

	case containerTypeInit:
		// An init container will only restart if:
		//
		// * It is in a waiting (meaning it's waiting to run) or running state
		//   (meaning it already restarted before we had a chance to check)
		// * It terminated with an exit code of non-zero and restartPolicy is not
		//   Never.
		//
		// https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#understanding-init-containers
		switch {
		case containerInfo.State.Waiting != nil || containerInfo.State.Running != nil:
			return false, nil // Container will restart
		case containerInfo.State.Terminated != nil && containerInfo.State.Terminated.ExitCode != 0 && restartPolicy != corev1.RestartPolicyNever:
			return false, nil // Container will restart
		default:
			return true, nil // Container will *not* restart
		}

	case containerTypeEphemeral:
		// Ephemeral containers never restart.
		//
		// https://kubernetes.io/docs/concepts/workloads/pods/ephemeral-containers/
		switch {
		case containerInfo.State.Waiting != nil || containerInfo.State.Running != nil:
			return false, nil // Container is running or is about to run
		default:
			return true, nil // Container will *not* restart
		}
	}

	return false, nil
}

// parseKubernetesLog parses a log line returned from the Kubernetes API,
// splitting out the timestamp and the log line. If the timestamp cannot be
// parsed, time.Now() is returned with the original log line intact.
//
// If the timestamp was parsed, it is stripped out of the resulting line of
// text.
func parseKubernetesLog(input string) (timestamp time.Time, line string) {
	timestampOffset := strings.IndexByte(input, ' ')
	if timestampOffset == -1 {
		return time.Now().UTC(), input
	}

	var remain string
	if timestampOffset < len(input) {
		remain = input[timestampOffset+1:]
	}

	// Kubernetes can return timestamps in either RFC3339Nano or RFC3339, so we
	// try both.
	timestampString := input[:timestampOffset]

	if timestamp, err := time.Parse(time.RFC3339Nano, timestampString); err == nil {
		return timestamp.UTC(), remain
	}

	if timestamp, err := time.Parse(time.RFC3339, timestampString); err == nil {
		return timestamp.UTC(), remain
	}

	return time.Now().UTC(), input
}
