package awsecscontainermetrics_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/awsecscontainermetrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver"
	"github.com/stretchr/testify/require"
)

func TestDefaultArguments(t *testing.T) {
	var args awsecscontainermetrics.Arguments
	args.SetToDefault()

	cfg, err := args.Convert()
	require.NoError(t, err)
	// Literal, not factory-derived, so a contrib bump that changes a default fails here.
	require.Equal(t, &awsecscontainermetricsreceiver.Config{
		CollectionInterval: 20 * time.Second,
	}, cfg)
}
