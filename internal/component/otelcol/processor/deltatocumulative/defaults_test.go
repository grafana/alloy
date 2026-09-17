package deltatocumulative_test

import (
	"math"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/processor/deltatocumulative"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"
	"github.com/stretchr/testify/require"
)

func TestDefaultArguments(t *testing.T) {
	var args deltatocumulative.Arguments
	args.SetToDefault()

	cfg, err := args.Convert()
	require.NoError(t, err)
	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	require.Equal(t, &deltatocumulativeprocessor.Config{
		MaxStale:   5 * time.Minute,
		MaxStreams: math.MaxInt,
	}, cfg)
}
