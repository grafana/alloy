package stages

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
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
func TestValidateLimitConfig(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name    string
		cfg     string
		wantErr bool
	}

	tests := []testCase{
		{
			name: "should pass on rate and burst set",
			cfg: `
			rate  = 1
			burst = 1
			`,
		},
		{
			name: "should fail on zero rate",
			cfg: `
			rate  = 0
			burst = 1
			`,
			wantErr: true,
		},
		{
			name: "should fail on zero burst",
			cfg: `
			rate  = 1
			burst = 0
			`,
			wantErr: true,
		},
		{
			name: "should fail on by_label_name without drop",
			cfg: `
			rate          = 1
			burst         = 1
			by_label_name = "app"
			`,
			wantErr: true,
		},
		{
			name: "should pass on by_label_name with drop",
			cfg: `
			rate          = 1
			burst         = 1
			drop          = true
			by_label_name = "app"
			`,
		},
		{
			name: "should fail on invalid by_label_name",
			cfg: `
			rate          = 1
			burst         = 1
			drop          = true
			by_label_name = "bad-name"
			`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var cfg LimitConfig
			err := syntax.Unmarshal([]byte(tt.cfg), &cfg)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestLimitConfigDefaults(t *testing.T) {
	var cfg LimitConfig
	err := syntax.Unmarshal([]byte(`
	rate  = 1
	burst = 1
	`), &cfg)
	require.NoError(t, err)
	require.Equal(t, defaultMaxDistinctLabels, cfg.MaxDistinctLabels)
}
