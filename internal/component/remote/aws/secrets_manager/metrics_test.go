package secrets_manager

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics_BothResultSeriesStartAtZero(t *testing.T) {
	reg := prometheus.NewRegistry()
	newMetrics(reg)

	for _, result := range []string{"success", "error"} {
		v, err := counterValue(reg, result)
		require.NoError(t, err)
		require.Zero(t, v)
	}
}

func TestNewMetrics_LastAccessedGaugeName(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)
	m.lastAccessed.SetToCurrentTime()

	families, err := reg.Gather()
	require.NoError(t, err)
	var names []string
	for _, f := range families {
		names = append(names, f.GetName())
	}
	require.Contains(t, names, "remote_aws_secrets_manager_timestamp_last_accessed_unix_seconds")
}
