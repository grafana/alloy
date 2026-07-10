package collector

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/database_observability/postgres/fingerprint"
	"github.com/grafana/alloy/internal/runtime/logging"
)

// The server_id label is applied to all of this component's metrics at scrape
// time via GetRelabelingRules, so the gauge itself carries no labels.
func TestLogsCollector_LogsProcessingEnabledMetric(t *testing.T) {
	testCases := []struct {
		name     string
		enabled  bool
		expected string
	}{
		{
			name:    "enabled reports 1",
			enabled: true,
			expected: `
# HELP database_observability_logs_processing_enabled Whether logs processing (error-log capture) is enabled for this database instance.
# TYPE database_observability_logs_processing_enabled gauge
database_observability_logs_processing_enabled 1
`,
		},
		{
			name:    "disabled reports 0",
			enabled: false,
			expected: `
# HELP database_observability_logs_processing_enabled Whether logs processing (error-log capture) is enabled for this database instance.
# TYPE database_observability_logs_processing_enabled gauge
database_observability_logs_processing_enabled 0
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.enabled && !fingerprint.Supported() {
				t.Skip("enable_error_logs_processing requires a cgo build; NewLogs rejects it otherwise")
			}
			reg := prometheus.NewRegistry()
			c, err := NewLogs(LogsArguments{
				Receiver:        loki.NewLogsReceiver(),
				EntryHandler:    loki.NewEntryHandler(make(chan loki.Entry, 1), func() {}),
				Logger:          logging.NewSlogNop(),
				Registry:        reg,
				EnableErrorLogs: tc.enabled,
			})
			require.NoError(t, err)

			require.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(tc.expected), "database_observability_logs_processing_enabled"))

			// Stop unregisters the metric.
			c.Stop()
			require.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(""), "database_observability_logs_processing_enabled"))
		})
	}
}
