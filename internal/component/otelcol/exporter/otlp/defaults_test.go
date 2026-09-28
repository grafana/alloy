package otlp_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/otlp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
)

func TestDefaultArguments(t *testing.T) {
	var args otlp.Arguments
	args.SetToDefault()

	cfg, err := args.Convert()
	require.NoError(t, err)
	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	require.Equal(t, &otlpexporter.Config{
		TimeoutConfig: exporterhelper.TimeoutConfig{Timeout: 5 * time.Second},
		QueueConfig: configoptional.Some(exporterhelper.QueueBatchConfig{
			Sizer:        exporterhelper.RequestSizerTypeRequests,
			QueueSize:    1000,
			NumConsumers: 10,
			Batch: configoptional.Default(exporterhelper.BatchConfig{
				FlushTimeout: 200 * time.Millisecond,
				Sizer:        exporterhelper.RequestSizerTypeItems,
				MinSize:      8192,
			}),
		}),
		RetryConfig: configretry.BackOffConfig{
			Enabled:             true,
			InitialInterval:     5 * time.Second,
			RandomizationFactor: 0.5,
			Multiplier:          1.5,
			MaxInterval:         30 * time.Second,
			MaxElapsedTime:      5 * time.Minute,
		},
		ClientConfig: configgrpc.ClientConfig{
			Compression:     configcompression.TypeGzip,
			WriteBufferSize: 512 * 1024,
			Headers:         configopaque.MapList{},
			BalancerName:    "round_robin",
		},
	}, cfg)
}
