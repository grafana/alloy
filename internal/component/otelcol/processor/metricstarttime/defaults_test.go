package metricstarttime_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/processor/metricstarttime"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor"
	"github.com/stretchr/testify/require"
)

func TestDefaultArguments(t *testing.T) {
	var args metricstarttime.Arguments
	args.SetToDefault()

	cfg, err := args.Convert()
	require.NoError(t, err)
	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	require.Equal(t, &metricstarttimeprocessor.Config{
		Strategy:   "true_reset_point",
		GCInterval: 10 * time.Minute,
	}, cfg)
}
