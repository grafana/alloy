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
