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
	// Literal, not factory-derived, so a contrib bump that changes a default fails here.
	require.Equal(t, &config.Config{ErrorMode: "propagate"}, cfg)
}
