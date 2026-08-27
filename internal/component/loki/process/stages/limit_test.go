package stages

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestLimitStage(t *testing.T) {
	type testCase struct {
		name            string
		config          string
		entries         []Entry
		expected        []Entry
		expectedMetrics string
	}

	now := time.Now()

	tests := []testCase{
		{
			name: "never drops entries",
			config: `
			stage.limit {
				rate  = 1
				burst = 1
				drop  = false
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "1", now),
				newEntry(map[string]any{}, model.LabelSet{}, "2", now),
				newEntry(map[string]any{}, model.LabelSet{}, "3", now),
				newEntry(map[string]any{}, model.LabelSet{}, "4", now),
				newEntry(map[string]any{}, model.LabelSet{}, "5", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "1", now),
				newEntry(map[string]any{}, model.LabelSet{}, "2", now),
				newEntry(map[string]any{}, model.LabelSet{}, "3", now),
				newEntry(map[string]any{}, model.LabelSet{}, "4", now),
				newEntry(map[string]any{}, model.LabelSet{}, "5", now),
			},
		},
		{
			name: "drop throttled entries",
			config: `
			stage.limit {
				rate  = 1
				burst = 1
				drop  = true
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "1", now),
				newEntry(map[string]any{}, model.LabelSet{}, "2", now),
				newEntry(map[string]any{}, model.LabelSet{}, "3", now),
				newEntry(map[string]any{}, model.LabelSet{}, "4", now),
				newEntry(map[string]any{}, model.LabelSet{}, "5", now),
				newEntry(map[string]any{}, model.LabelSet{}, "6", now),
				newEntry(map[string]any{}, model.LabelSet{}, "7", now),
				newEntry(map[string]any{}, model.LabelSet{}, "8", now),
				newEntry(map[string]any{}, model.LabelSet{}, "9", now),
				newEntry(map[string]any{}, model.LabelSet{}, "10", now),
			},
			expected: []Entry{
				newEntry(map[string]any{}, model.LabelSet{}, "1", now),
			},
			expectedMetrics: `
# HELP loki_process_dropped_lines_total A count of all log lines dropped as a result of a pipeline stage
# TYPE loki_process_dropped_lines_total counter
loki_process_dropped_lines_total{reason="ratelimit_drop_stage"} 9
`,
		},
		{
			name: "by label drops all but the first per distinct label value",
			config: `
			stage.limit {
				rate  = 1
				burst = 1
				drop  = true

				by_label_name = "app"
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "loki-1", now),
				newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "loki-2", now),
				newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "loki-3", now),
				newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "loki-4", now),
				newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, "loki-5", now),
				newEntry(map[string]any{}, model.LabelSet{"app": "poki"}, "poki-1", now),
				newEntry(map[string]any{}, model.LabelSet{"app": "poki"}, "poki-2", now),
				newEntry(map[string]any{}, model.LabelSet{"app": "poki"}, "poki-3", now),
				newEntry(map[string]any{}, model.LabelSet{"app": "poki"}, "poki-4", now),
				newEntry(map[string]any{}, model.LabelSet{"app": "poki"}, "poki-5", now),
				newEntry(map[string]any{}, model.LabelSet{}, "noapp-1", now),
				newEntry(map[string]any{}, model.LabelSet{}, "noapp-2", now),
				newEntry(map[string]any{}, model.LabelSet{}, "noapp-3", now),
				newEntry(map[string]any{}, model.LabelSet{}, "noapp-4", now),
				newEntry(map[string]any{}, model.LabelSet{}, "noapp-5", now),
			},
			expected: []Entry{
				newEntry(map[string]any{"app": "loki"}, model.LabelSet{"app": "loki"}, "loki-1", now),
				newEntry(map[string]any{"app": "poki"}, model.LabelSet{"app": "poki"}, "poki-1", now),
				newEntry(map[string]any{}, model.LabelSet{}, "noapp-1", now),
				newEntry(map[string]any{}, model.LabelSet{}, "noapp-2", now),
				newEntry(map[string]any{}, model.LabelSet{}, "noapp-3", now),
				newEntry(map[string]any{}, model.LabelSet{}, "noapp-4", now),
				newEntry(map[string]any{}, model.LabelSet{}, "noapp-5", now),
			},
			expectedMetrics: `
# HELP loki_process_dropped_lines_total A count of all log lines dropped as a result of a pipeline stage
# TYPE loki_process_dropped_lines_total counter
loki_process_dropped_lines_total{reason="ratelimit_drop_stage"} 8
# HELP loki_process_dropped_lines_by_label_total A count of all log lines dropped as a result of a pipeline stage
# TYPE loki_process_dropped_lines_by_label_total counter
loki_process_dropped_lines_by_label_total{label_name="app",label_value="loki"} 4
loki_process_dropped_lines_by_label_total{label_name="app",label_value="poki"} 4
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, loadConfig(tt.config), tt.entries, tt.expected, entryCheckFNs{
				metrics: func(reg *prometheus.Registry) error {
					return testutil.GatherAndCompare(reg, strings.NewReader(tt.expectedMetrics))
				},
			})
		})
	}
}

// TestLimitStageShutdown verifies that an entry blocked in rateLimiter.Wait
// is released promptly when the pipeline shuts down.
func TestLimitStageShutdown(t *testing.T) {
	type testCase struct {
		name string
		cfg  string
	}

	tests := []testCase{
		{
			name: "stage.limit",
			cfg: `
			stage.limit {
				rate  = 0.1
				burst = 1
				drop  = false
			}
			`,
		},
		{
			name: "stage.limit inside stage.match",
			cfg: `
			stage.match {
				selector = "{app=\"loki\"}"
				action = "keep"
				stage.limit {
					rate  = 0.1
					burst = 1
					drop  = false
				}
			}
			`,
		},
	}

	t.Run("Stage", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pl, err := NewPipeline(logging.NewSlogNop(), loadConfig(tt.cfg), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable)
				require.NoError(t, err)

				in := make(chan loki.Entry)
				out := make(chan loki.Entry, 1)
				handler := pl.Start(in, out)

				entry := loki.Entry{
					Labels: model.LabelSet{"app": "loki"},
					Entry:  push.Entry{Line: testMatchLogLineApp1, Timestamp: time.Now()},
				}

				in <- entry
				<-out       // burst consumed; next Wait() will block
				in <- entry // blocks the limit stage in rateLimiter.Wait

				done := make(chan struct{})
				go func() { defer close(done); handler.Stop() }()

				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Fatal("Stop() did not release the entry blocked in rateLimiter.Wait")
				}

				select {
				case e := <-out:
					t.Fatalf("expected the entry blocked in rateLimiter.Wait to be dropped on shutdown, but it was forwarded: %+v", e)
				default:
				}
			})
		}
	})

	t.Run("New Stage", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var collected []Entry
				next := func(_ context.Context, entries []Entry) error {
					collected = append(collected, entries...)
					return nil
				}
				p, err := NewPipeline2(logging.NewSlogNop(), prometheus.NewRegistry(), featuregate.StabilityGenerallyAvailable, loadConfig(tt.cfg), next)
				require.NoError(t, err)

				entry := func() Entry {
					return newEntry(map[string]any{}, model.LabelSet{"app": "loki"}, testMatchLogLineApp1, time.Now())
				}

				require.NoError(t, p.process(context.Background(), []Entry{entry()})) // burst consumed

				done := make(chan struct{})
				go func() {
					defer close(done)
					_ = p.process(context.Background(), []Entry{entry()}) // blocks in rateLimiter.Wait
				}()

				// Give the goroutine above time to actually enter Wait() before cancelling.
				time.Sleep(50 * time.Millisecond)
				p.Stop()

				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Fatal("Cleanup() did not release the entry blocked in rateLimiter.Wait")
				}

				require.Len(t, collected, 1, "expected the entry blocked in rateLimiter.Wait to be dropped on shutdown")
			})
		}
	})
}
