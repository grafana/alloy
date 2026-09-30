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
		v, err := counterValue(reg, "remote_aws_secrets_manager_fetches_total", result)
		require.NoError(t, err)
		require.Zero(t, v)
	}
}
