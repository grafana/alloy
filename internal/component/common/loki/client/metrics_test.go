package client

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

// A config reload calls newMetrics again with the same registry. The
// zero-initialized counters must then use the registered collectors.
func TestNewMetricsReusedRegistryInitializesRegisteredCounters(t *testing.T) {
	reg := prometheus.NewRegistry()
	newMetrics(reg)
	m := newMetrics(reg)

	for _, counter := range m.countersWithHostReason {
		for _, reason := range reasons {
			counter.WithLabelValues("host", reason).Add(0)
		}
	}
	for _, counter := range m.countersWithHost {
		counter.WithLabelValues("host").Add(0)
	}

	for _, name := range []string{
		"loki_write_dropped_bytes_total",
		"loki_write_dropped_entries_total",
	} {
		count, err := testutil.GatherAndCount(reg, name)
		require.NoError(t, err)
		require.Equal(t, len(reasons), count, name)
	}
	for _, name := range []string{
		"loki_write_batch_retries_total",
		"loki_write_sent_bytes_total",
		"loki_write_sent_entries_total",
	} {
		count, err := testutil.GatherAndCount(reg, name)
		require.NoError(t, err)
		require.Equal(t, 1, count, name)
	}
}
