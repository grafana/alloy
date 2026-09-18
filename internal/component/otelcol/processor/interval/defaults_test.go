package interval_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/processor/interval"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor"
	"github.com/stretchr/testify/require"
)

func TestDefaultArguments(t *testing.T) {
	var args interval.Arguments
	args.SetToDefault()

	cfg, err := args.Convert()
	require.NoError(t, err)
	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	require.Equal(t, &intervalprocessor.Config{Interval: time.Minute}, cfg)
}
