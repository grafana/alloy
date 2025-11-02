package kubetail

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/positions"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
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

func Test_processLogStream_duplicateTimestamps(t *testing.T) {
	baseTime := time.Date(2023, time.January, 23, 17, 0, 10, 0, time.UTC)

	tt := []struct {
		name         string
		logLines     []string
		lastReadTime time.Time
		expectLines  []string
	}{
		{
			name: "duplicate timestamps are not discarded",
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
			lastReadTime: baseTime,
			expectLines:  []string{"line1\n", "line2\n", "line3\n", "line4\n"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock tailer with minimal setup
			lset := labels.FromStrings(
				LabelPodNamespace, "default",
				LabelPodName, "test-pod",
				LabelPodContainerName, "test-container",
				LabelPodUID, "test-uid-123",
				"test", "value",
			)
			target := NewTarget(lset, lset)
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
				_ = tailer.processLogStream(ctx, stream, handler, tc.lastReadTime, positionsEnt, calc)
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
		})
	}
}
