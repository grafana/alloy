package signaltometrics_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/connector/signaltometrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"github.com/stretchr/testify/require"
)

func TestDefaultArguments(t *testing.T) {
	var args signaltometrics.Arguments
	args.SetToDefault()

	cfg, err := args.Convert()
	require.NoError(t, err)
	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	require.Equal(t, &config.Config{ErrorMode: "propagate"}, cfg)
}
