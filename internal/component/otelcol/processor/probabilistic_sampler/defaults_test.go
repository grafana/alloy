package probabilistic_sampler_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/processor/probabilistic_sampler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"
	"github.com/stretchr/testify/require"
)

func TestDefaultArguments(t *testing.T) {
	var args probabilistic_sampler.Arguments
	args.SetToDefault()

	cfg, err := args.Convert()
	require.NoError(t, err)
	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	require.Equal(t, &probabilisticsamplerprocessor.Config{
		FailClosed:        true,
		SamplingPrecision: 4,
		AttributeSource:   "traceID",
	}, cfg)
}
