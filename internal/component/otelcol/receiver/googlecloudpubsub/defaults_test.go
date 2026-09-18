package googlecloudpubsub_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/googlecloudpubsub"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestDefaultArguments(t *testing.T) {
	var args googlecloudpubsub.Arguments
	args.SetToDefault()

	cfg, err := args.Convert()
	require.NoError(t, err)
	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	require.Equal(t, &googlecloudpubsubreceiver.Config{
		UserAgent:       "opentelemetry-collector-contrib {{version}}",
		TimeoutSettings: exporterhelper.TimeoutConfig{Timeout: 12 * time.Second},
		FlowControlConfig: googlecloudpubsubreceiver.FlowControlConfig{
			TriggerAckBatchDuration: 10 * time.Second,
			StreamAckDeadline:       time.Minute,
		},
	}, cfg)
}
