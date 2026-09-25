package kubetail

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	fakerest "k8s.io/client-go/rest/fake"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/internal/positions"
	"github.com/grafana/alloy/internal/runtime/logging"
)

// mockPositions is a no-op implementation of positions.Positions for testing.
type mockPositions struct{}

func (m *mockPositions) GetString(path, labels string) string { return "" }

func (m *mockPositions) Get(path, labels string) (int64, error) { return 0, nil }

func (m *mockPositions) PutString(path, labels, pos string) {}

func (m *mockPositions) Put(path, labels string, pos int64) {}

func (m *mockPositions) Remove(path, labels string) {}

func (m *mockPositions) Stop() {}

func (m *mockPositions) SyncPeriod() time.Duration { return 0 }

func (m *mockPositions) WatchConfig(cfg positions.Config) {}

// mockEntryHandler is a simple implementation of loki.EntryHandler for testing.
type mockEntryHandler struct {
	ch chan loki.Entry
}

func newMockEntryHandler() *mockEntryHandler {
	return &mockEntryHandler{
		ch: make(chan loki.Entry, 100),
	}
}

func (m *mockEntryHandler) Chan() chan<- loki.Entry { return m.ch }

func (m *mockEntryHandler) Stop() {}

// mockReadCloser wraps a strings.Reader to provide io.ReadCloser interface.
type mockReadCloser struct {
	*strings.Reader
}

func (m *mockReadCloser) Close() error { return nil }

func Test_parseKubernetesLog(t *testing.T) {
	tt := []struct {
		inputLine  string
		expectTS   time.Time
		expectLine string
	}{
		{
			// Test normal RFC3339Nano log line.
			inputLine:  `2023-01-23T17:00:10.000000001Z hello, world!`,
			expectTS:   time.Date(2023, time.January, 23, 17, 0, 10, 1, time.UTC),
			expectLine: "hello, world!",
		},
		{
			// Test normal RFC3339 log line.
			inputLine:  `2023-01-23T17:00:10Z hello, world!`,
			expectTS:   time.Date(2023, time.January, 23, 17, 0, 10, 0, time.UTC),
			expectLine: "hello, world!",
		},
		{
			// Test empty log line. There will always be a space prepended by
			// Kubernetes.
			inputLine:  `2023-01-23T17:00:10.000000001Z `,
			expectTS:   time.Date(2023, time.January, 23, 17, 0, 10, 1, time.UTC),
			expectLine: "",
		},
	}

	for _, tc := range tt {
		t.Run(tc.inputLine, func(t *testing.T) {
			actualTS, actualLine := parseKubernetesLog(tc.inputLine)
			require.Equal(t, tc.expectTS, actualTS)
			require.Equal(t, tc.expectLine, actualLine)
		})
	}
}

func Test_processLogStream(t *testing.T) {
	baseTime := time.Date(2023, time.January, 23, 17, 0, 10, 0, time.UTC)

	tt := []struct {
		name               string
		preserveMetaLabels bool
		logLines           []string
		lastReadTime       time.Time
		expectLines        []string
	}{
		{name: "duplicate timestamps are not discarded",
			logLines: []string{
				"2023-01-23T17:00:10Z line1\n",
				"2023-01-23T17:00:10Z line2\n",
				"2023-01-23T17:00:10Z line3\n",
			},
			lastReadTime: baseTime.Add(-1 * time.Second), // Before all entries
			expectLines:  []string{"line1\n", "line2\n", "line3\n"},
		},
		{
			name: "entries before lastReadTime are discarded",
			logLines: []string{
				"2023-01-23T17:00:09Z old_line\n",
				"2023-01-23T17:00:10Z line1\n",
				"2023-01-23T17:00:11Z line2\n",
			},
			lastReadTime: baseTime, // Equal to second entry
			expectLines:  []string{"line1\n", "line2\n"},
		},
		{
			name: "entries equal to lastReadTime are included",
			logLines: []string{
				"2023-01-23T17:00:10Z line1\n",
				"2023-01-23T17:00:10Z line2\n",
				"2023-01-23T17:00:11Z line3\n",
			},
			lastReadTime: baseTime, // Equal to first two entries
			expectLines:  []string{"line1\n", "line2\n", "line3\n"},
		},
		{
			name: "mixed timestamps with duplicates",
			logLines: []string{
				"2023-01-23T17:00:08Z old1\n",
				"2023-01-23T17:00:09Z old2\n",
				"2023-01-23T17:00:10Z line1\n",
				"2023-01-23T17:00:10Z line2\n",
				"2023-01-23T17:00:11Z line3\n",
				"2023-01-23T17:00:11Z line4\n",
			},
			lastReadTime: baseTime,
			expectLines:  []string{"line1\n", "line2\n", "line3\n", "line4\n"},
		},
		{
			name: "all entries have same timestamp",
			logLines: []string{
				"2023-01-23T17:00:10Z line1\n",
				"2023-01-23T17:00:10Z line2\n",
				"2023-01-23T17:00:10Z line3\n",
				"2023-01-23T17:00:10Z line4\n",
			},
			lastReadTime:       baseTime,
			expectLines:        []string{"line1\n", "line2\n", "line3\n", "line4\n"},
			preserveMetaLabels: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock tailer with minimal setup
			lset := labels.FromStrings(
				kubePodName, "test-pod",
				kubePodNamespace, "default",
				kubePodContainerName, "test-container",
				kubePodUID, "test-uid-123",
				"test", "value",
			)

			lset, err := PrepareLabelsWithMetaPreservation(lset, "test", tc.preserveMetaLabels)
			require.NoError(t, err)

			target := NewTarget(lset, lset, tc.preserveMetaLabels)
			opts := &Options{
				Positions: &mockPositions{},
			}
			tailer := &tailer{
				target: target,
				lset:   newLabelSet(target.Labels()),
				opts:   opts,
			}

			// Create a stream from the log lines
			logData := strings.Join(tc.logLines, "")
			stream := &mockReadCloser{strings.NewReader(logData)}

			// Create a mock handler
			handler := newMockEntryHandler()

			// Create a mock positions entry
			positionsEnt := positions.Entry{}

			// Create a rolling average calculator
			calc := newRollingAverageCalculator(10000, 100, 2*time.Second, 1*time.Hour)

			// Create a context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			// Process the log stream in a goroutine
			go func() {
				_, _ = tailer.processLogStream(ctx, stream, handler, tc.lastReadTime, positionsEnt, calc)
			}()

			// Collect all entries
			var receivedLines []string
			timeout := time.After(500 * time.Millisecond)

		collectLoop:
			for {
				select {
				case entry := <-handler.ch:
					receivedLines = append(receivedLines, entry.Line)
					if len(receivedLines) == len(tc.expectLines) {
						break collectLoop
					}
				case <-timeout:
					break collectLoop
				}
			}

			require.Equal(t, tc.expectLines, receivedLines, "received lines should match expected lines")

			if tc.preserveMetaLabels {
				lbls := target.Labels()
				require.Equal(t, "test", lbls.Get("job"))
				require.Equal(t, "value", lbls.Get("test"))
				require.Equal(t, "default/test-pod:test-container", lbls.Get("instance"))
				require.Equal(t, "test-pod", lbls.Get(kubePodName))
				require.Equal(t, "default", lbls.Get(kubePodNamespace))
				require.Equal(t, "test-container", lbls.Get(kubePodContainerName))
				require.Equal(t, "test-uid-123", lbls.Get(kubePodUID))
			} else {
				lbls := target.Labels()
				require.Equal(t, "test", lbls.Get("job"))
				require.Equal(t, "value", lbls.Get("test"))
				require.Equal(t, "default/test-pod:test-container", lbls.Get("instance"))
			}
		})
	}
}

func Test_processLogStream_retail(t *testing.T) {
	// Each tail processes a stream like the Kubernetes API returns it after a
	// re-tail: SinceTime has second precision, so the stream repeats lines
	// which were already forwarded.
	type tail struct {
		stream    []string // Lines in the stream.
		streamErr error    // Error which ends the stream. The stream ends with io.EOF if nil.
		expect    []string // Lines expected to be forwarded.
	}

	tt := []struct {
		name  string
		tails []tail
	}{
		{
			name: "last line is not forwarded again",
			tails: []tail{
				{
					stream: []string{
						"2023-01-23T17:00:10.1Z line1\n",
						"2023-01-23T17:00:10.2Z line2\n",
					},
					expect: []string{"line1\n", "line2\n"},
				},
				{
					stream: []string{
						"2023-01-23T17:00:10.1Z line1\n",
						"2023-01-23T17:00:10.2Z line2\n",
					},
				},
				{
					stream: []string{
						"2023-01-23T17:00:10.1Z line1\n",
						"2023-01-23T17:00:10.2Z line2\n",
						"2023-01-23T17:00:10.3Z line3\n",
					},
					expect: []string{"line3\n"},
				},
			},
		},
		{
			name: "lines sharing the last timestamp are forwarded once each",
			tails: []tail{
				{
					stream: []string{
						"2023-01-23T17:00:10.1Z line1\n",
						"2023-01-23T17:00:10.2Z line2\n",
						"2023-01-23T17:00:10.2Z line3\n",
						"2023-01-23T17:00:10.2Z line4\n",
					},
					expect: []string{"line1\n", "line2\n", "line3\n", "line4\n"},
				},
				{
					stream: []string{
						"2023-01-23T17:00:10.1Z line1\n",
						"2023-01-23T17:00:10.2Z line2\n",
						"2023-01-23T17:00:10.2Z line3\n",
						"2023-01-23T17:00:10.2Z line4\n",
					},
				},
				{
					stream: []string{
						"2023-01-23T17:00:10.1Z line1\n",
						"2023-01-23T17:00:10.2Z line2\n",
						"2023-01-23T17:00:10.2Z line3\n",
						"2023-01-23T17:00:10.2Z line4\n",
						"2023-01-23T17:00:10.2Z line5\n",
						"2023-01-23T17:00:10.3Z line6\n",
					},
					expect: []string{"line5\n", "line6\n"},
				},
			},
		},
		{
			name: "re-tail between lines sharing a timestamp",
			tails: []tail{
				{
					stream: []string{
						"2023-01-23T17:00:10.1Z line1\n",
						"2023-01-23T17:00:10.2Z line2\n",
					},
					expect: []string{"line1\n", "line2\n"},
				},
				{
					stream: []string{
						"2023-01-23T17:00:10.1Z line1\n",
						"2023-01-23T17:00:10.2Z line2\n",
						"2023-01-23T17:00:10.2Z line3\n",
						"2023-01-23T17:00:10.2Z line4\n",
					},
					expect: []string{"line3\n", "line4\n"},
				},
			},
		},
		{
			name: "timestamps with second precision",
			tails: []tail{
				{
					stream: []string{
						"2023-01-23T17:00:10Z line1\n",
						"2023-01-23T17:00:10Z line2\n",
					},
					expect: []string{"line1\n", "line2\n"},
				},
				{
					stream: []string{
						"2023-01-23T17:00:10Z line1\n",
						"2023-01-23T17:00:10Z line2\n",
						"2023-01-23T17:00:10Z line3\n",
						"2023-01-23T17:00:11Z line4\n",
					},
					expect: []string{"line3\n", "line4\n"},
				},
			},
		},
		{
			name: "lines older than the resume point are skipped",
			tails: []tail{
				{
					stream: []string{
						"2023-01-23T17:00:10.1Z line1\n",
						"2023-01-23T17:00:10.2Z line2\n",
					},
					expect: []string{"line1\n", "line2\n"},
				},
				{
					stream: []string{
						"2023-01-23T17:00:10.1Z line1\n",
						"2023-01-23T17:00:10.2Z line2\n",
						"2023-01-23T17:00:10.15Z out_of_order\n",
						"2023-01-23T17:00:10.3Z line3\n",
					},
					expect: []string{"line3\n"},
				},
			},
		},
		{
			name: "line cut off by a stream error is forwarded in full by the next tail",
			tails: []tail{
				{
					stream: []string{
						"2023-01-23T17:00:10.1Z line1\n",
						"2023-01-23T17:00:10.2Z line2 is cut",
					},
					streamErr: io.ErrUnexpectedEOF,
					expect:    []string{"line1\n"},
				},
				{
					stream: []string{
						"2023-01-23T17:00:10.1Z line1\n",
						"2023-01-23T17:00:10.2Z line2 is cut off\n",
					},
					expect: []string{"line2 is cut off\n"},
				},
			},
		},
		{
			name: "last line without a newline is forwarded once",
			tails: []tail{
				{
					stream: []string{
						"2023-01-23T17:00:10.1Z line1\n",
						"2023-01-23T17:00:10.2Z line2",
					},
					expect: []string{"line1\n", "line2"},
				},
				{
					stream: []string{
						"2023-01-23T17:00:10.1Z line1\n",
						"2023-01-23T17:00:10.2Z line2",
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			handler := newMockEntryHandler()
			tailer := newTestTailer(newTestTarget(t), nil, handler)

			for i, tail := range tc.tails {
				var stream io.Reader = strings.NewReader(strings.Join(tail.stream, ""))
				if tail.streamErr != nil {
					stream = io.MultiReader(stream, iotest.ErrReader(tail.streamErr))
				}
				calc := newRollingAverageCalculator(10000, 100, 2*time.Second, time.Hour)

				// Resume from the last forwarded line, like tail does.
				forwarded, err := tailer.processLogStream(t.Context(), io.NopCloser(stream), handler, tailer.target.LastEntry(), positions.Entry{}, calc)
				require.ErrorIs(t, err, tail.streamErr, "tail %d", i+1)
				require.Equal(t, tail.expect, drainLines(handler), "tail %d", i+1)
				require.Equal(t, len(tail.expect), forwarded, "tail %d", i+1)
			}
		})
	}
}

func TestTarget_reportEntry(t *testing.T) {
	var (
		target = newTestTarget(t)
		ts     = time.Date(2023, time.January, 23, 17, 0, 10, 0, time.UTC)
	)

	target.reportEntry(ts)
	target.reportEntry(ts)
	require.Equal(t, ts, target.LastEntry())
	require.Equal(t, 2, target.entriesAt(ts))
	require.Zero(t, target.entriesAt(ts.Add(-time.Second)))

	// A line older than LastEntry doesn't move it backwards.
	target.reportEntry(ts.Add(-time.Millisecond))
	require.Equal(t, ts, target.LastEntry())
	require.Equal(t, 2, target.entriesAt(ts))

	// An error doesn't move the resume point.
	target.reportError(errors.New("connection reset by peer"))
	require.EqualError(t, target.LastError(), "connection reset by peer")
	require.Equal(t, ts, target.LastEntry())
	require.Equal(t, 2, target.entriesAt(ts))

	// A newer line moves LastEntry and clears the error.
	target.reportEntry(ts.Add(time.Millisecond))
	require.NoError(t, target.LastError())
	require.Equal(t, ts.Add(time.Millisecond), target.LastEntry())
	require.Equal(t, 1, target.entriesAt(ts.Add(time.Millisecond)))
	require.Zero(t, target.entriesAt(ts))
}

// Log lines as returned by the Kubernetes API for the tests below.
const (
	testLine1 = "2023-01-23T17:00:10.1Z line1\n"
	testLine2 = "2023-01-23T17:00:10.2Z line2\n"
	testLine3 = "2023-01-23T17:00:10.3Z line3\n"
)

var testLine2Time = time.Date(2023, time.January, 23, 17, 0, 10, 200_000_000, time.UTC)

// A container which isn't running returns the same log lines on every
// re-tail. They must be forwarded once, and the tailer must back off instead of
// re-tailing in a tight loop.
func TestTailer_Run_containerNotRunning(t *testing.T) {
	tt := []struct {
		name string
		pod  func() *corev1.Pod
	}{
		{
			name: "container waiting to restart",
			pod: func() *corev1.Pod {
				return newTestPod(corev1.RestartPolicyAlways, corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
				})
			},
		},
		{
			name: "terminated container which will restart",
			pod: func() *corev1.Pod {
				return newTestPod(corev1.RestartPolicyAlways, corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
				})
			},
		},
		{
			name: "terminated Job container within its grace period",
			pod: func() *corev1.Pod {
				pod := newTestPod(corev1.RestartPolicyNever, corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, FinishedAt: metav1.Now()},
				})
				pod.OwnerReferences = []metav1.OwnerReference{{
					APIVersion: "batch/v1",
					Kind:       "Job",
					Name:       "test-job",
					UID:        "test-job-uid",
					Controller: &[]bool{true}[0],
				}}
				return pod
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			client := newFakeLogClient(tc.pod(), func(int) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(testLine1 + testLine2)), nil
			})
			handler := newMockEntryHandler()
			stop := runTailer(t, newTestTailer(newTestTarget(t), client, handler))

			const retails = 5
			require.Eventually(t, func() bool { return len(client.logRequests()) > retails }, 10*time.Second, time.Millisecond)
			stop()

			require.Equal(t, []string{"line1\n", "line2\n"}, drainLines(handler))

			requests := client.logRequests()
			for i := 1; i <= retails; i++ {
				require.WithinDuration(t, testLine2Time, requests[i].since, 0, "re-tail %d resumes from the last line", i)

				// Re-tails which only return lines that were already forwarded
				// don't reset the backoff, so the delay doubles every time.
				minDelay := retailBackoff.MinBackoff << (i - 1)
				require.GreaterOrEqual(t, requests[i].time.Sub(requests[i-1].time), minDelay, "delay before re-tail %d", i)
			}
		})
	}
}

// An error which stops the tailer must not move the resume point past lines
// which weren't read yet.
func TestTailer_Run_errorKeepsResumePoint(t *testing.T) {
	var (
		target      = newTestTarget(t)
		errAtRetail error
	)
	pod := newTestPod(corev1.RestartPolicyAlways, corev1.ContainerState{Running: &corev1.ContainerStateRunning{}})
	client := newFakeLogClient(pod, func(request int) (io.ReadCloser, error) {
		switch request {
		case 0:
			return io.NopCloser(strings.NewReader(testLine1 + testLine2)), nil
		case 1:
			return nil, errors.New("connection reset by peer")
		case 2:
			errAtRetail = target.LastError()
		}
		// line3 was written before the error, but not read yet.
		return io.NopCloser(strings.NewReader(testLine1 + testLine2 + testLine3)), nil
	})
	handler := newMockEntryHandler()
	stop := runTailer(t, newTestTailer(target, client, handler))

	require.Eventually(t, func() bool { return len(client.logRequests()) > 3 }, 10*time.Second, time.Millisecond)
	require.NoError(t, target.LastError(), "forwarding line3 clears the error")
	stop()

	require.Equal(t, []string{"line1\n", "line2\n", "line3\n"}, drainLines(handler))
	require.ErrorContains(t, errAtRetail, "connection reset by peer")
	require.WithinDuration(t, testLine2Time, client.logRequests()[2].since, 0, "re-tail after the error resumes from the last forwarded line")
}

// A log stream which stays open for its whole lifetime is healthy even if it
// has no new lines, so the tailer must re-tail without backing off.
func TestTailer_Run_fullLifetimeResetsBackoff(t *testing.T) {
	lifetime := maxTailerLifetime
	t.Cleanup(func() { maxTailerLifetime = lifetime })
	maxTailerLifetime = 50 * time.Millisecond

	pod := newTestPod(corev1.RestartPolicyAlways, corev1.ContainerState{Running: &corev1.ContainerStateRunning{}})
	client := newFakeLogClient(pod, func(int) (io.ReadCloser, error) {
		return &followStream{Reader: strings.NewReader(testLine1), closed: make(chan struct{})}, nil
	})
	handler := newMockEntryHandler()
	stop := runTailer(t, newTestTailer(newTestTarget(t), client, handler))

	// With a growing backoff, 10 re-tails would take more than 10s.
	require.Eventually(t, func() bool { return len(client.logRequests()) > 10 }, 5*time.Second, time.Millisecond)
	stop()

	require.Equal(t, []string{"line1\n"}, drainLines(handler))
}

// newTestTarget returns a target for the container test-container of the Pod
// default/test-pod.
func newTestTarget(t *testing.T) *Target {
	lset, err := PrepareLabels(labels.FromStrings(
		kubePodNamespace, "default",
		kubePodName, "test-pod",
		kubePodContainerName, "test-container",
		kubePodUID, "test-uid",
	), "test")
	require.NoError(t, err)
	return NewTarget(lset, lset, false)
}

// newTestPod returns the Pod default/test-pod, whose container test-container
// is in the given state.
func newTestPod(restartPolicy corev1.RestartPolicy, state corev1.ContainerState) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test-pod", UID: "test-uid"},
		Spec: corev1.PodSpec{
			RestartPolicy: restartPolicy,
			Containers:    []corev1.Container{{Name: "test-container"}},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{Name: "test-container", State: state}},
		},
	}
}

// newTestTailer returns a tailer for target which reads logs with client and
// forwards them to handler.
func newTestTailer(target *Target, client kubernetes.Interface, handler loki.EntryHandler) *tailer {
	return newTailer(logging.NewSlogNop(), &tailerTask{
		Options: &Options{Client: client, Handler: handler, Positions: &mockPositions{}},
		Target:  target,
	})
}

// runTailer runs tailer in the background. The returned function stops it and
// waits for it to exit; it's also called when the test ends.
func runTailer(t *testing.T, tailer *tailer) (stop func()) {
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		tailer.Run(ctx)
	}()

	stop = func() {
		cancel()
		<-done
	}
	t.Cleanup(stop)
	return stop
}

// drainLines returns the lines of the entries sent to handler since the
// last call.
func drainLines(handler *mockEntryHandler) []string {
	var lines []string
	for {
		select {
		case entry := <-handler.ch:
			lines = append(lines, entry.Line)
		default:
			return lines
		}
	}
}

// fakeLogClient is a fake Kubernetes client which answers log requests with the
// streams returned by logs, which gets the index of the request.
type fakeLogClient struct {
	*fake.Clientset
	logs func(request int) (io.ReadCloser, error)

	mut      sync.Mutex
	requests []logRequest
}

// logRequest is a log request received by a fakeLogClient.
type logRequest struct {
	time  time.Time // When the request was made.
	since time.Time // SinceTime of the request.
}

func newFakeLogClient(pod *corev1.Pod, logs func(request int) (io.ReadCloser, error)) *fakeLogClient {
	client := &fakeLogClient{Clientset: fake.NewClientset(pod), logs: logs}
	client.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &version.Info{GitVersion: "v1.30.0"}
	return client
}

// CoreV1 returns the CoreV1 client of the fake Clientset, whose Pods serve
// logs from the fakeLogClient.
func (c *fakeLogClient) CoreV1() typedcorev1.CoreV1Interface {
	return fakeLogCoreV1{CoreV1Interface: c.Clientset.CoreV1(), client: c}
}

func (c *fakeLogClient) getLogs(opts *corev1.PodLogOptions) *restclient.Request {
	c.mut.Lock()
	request := logRequest{time: time.Now()}
	if opts.SinceTime != nil {
		request.since = opts.SinceTime.Time
	}
	c.requests = append(c.requests, request)
	index := len(c.requests) - 1
	c.mut.Unlock()

	stream, err := c.logs(index)
	restClient := &fakerest.RESTClient{
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		GroupVersion:         corev1.SchemeGroupVersion,
		Client: fakerest.CreateHTTPClient(func(*http.Request) (*http.Response, error) {
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: http.StatusOK, Body: stream}, nil
		}),
	}
	return restClient.Request()
}

// logRequests returns the log requests received so far.
func (c *fakeLogClient) logRequests() []logRequest {
	c.mut.Lock()
	defer c.mut.Unlock()
	return slices.Clone(c.requests)
}

type fakeLogCoreV1 struct {
	typedcorev1.CoreV1Interface
	client *fakeLogClient
}

func (c fakeLogCoreV1) Pods(namespace string) typedcorev1.PodInterface {
	return fakeLogPods{PodInterface: c.CoreV1Interface.Pods(namespace), client: c.client}
}

type fakeLogPods struct {
	typedcorev1.PodInterface
	client *fakeLogClient
}

func (p fakeLogPods) GetLogs(_ string, opts *corev1.PodLogOptions) *restclient.Request {
	return p.client.getLogs(opts)
}

// followStream returns the log lines of Reader and then blocks until it's
// closed, like the log stream of a running container.
type followStream struct {
	io.Reader
	closed    chan struct{}
	closeOnce sync.Once
}

func (s *followStream) Read(p []byte) (int, error) {
	n, err := s.Reader.Read(p)
	if errors.Is(err, io.EOF) {
		<-s.closed
		return n, errors.New("stream closed")
	}
	return n, err
}

func (s *followStream) Close() error {
	s.closeOnce.Do(func() { close(s.closed) })
	return nil
}
