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
	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	require.Equal(t, &awsecscontainermetricsreceiver.Config{
		CollectionInterval: 20 * time.Second,
	}, cfg)
}
