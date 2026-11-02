package faro_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/faro"
	"github.com/grafana/alloy/internal/component/otelcol/internal/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestDefaultArguments(t *testing.T) {
	var args faro.Arguments
	args.SetToDefault()

	cfg, err := args.Convert()
	require.NoError(t, err)
	// Canary for the upstream defaults our docs promise. If this fails, a contrib bump
	// changed one: update the docs, then these values.
	require.Equal(t, &faroexporter.Config{
		ClientConfig: confighttp.ClientConfig{
			Timeout:           30 * time.Second,
			Headers:           configopaque.MapList{},
			Compression:       configcompression.TypeGzip,
			WriteBufferSize:   512 * 1024,
			MaxIdleConns:      100,              //nolint:staticcheck // migrate to confighttp Keepalive
			IdleConnTimeout:   90 * time.Second, //nolint:staticcheck // migrate to confighttp Keepalive
			ForceAttemptHTTP2: true,
		},
		QueueConfig: configoptional.Some(exporterhelper.QueueBatchConfig{
			Sizer:        exporterhelper.RequestSizerTypeRequests,
			QueueSize:    1000,
			NumConsumers: 10,
			Batch:        testutils.DefaultExporterBatch(),
		}),
		RetryConfig: configretry.BackOffConfig{
			Enabled:             true,
			InitialInterval:     5 * time.Second,
			RandomizationFactor: 0.5,
			Multiplier:          1.5,
			MaxInterval:         30 * time.Second,
			MaxElapsedTime:      5 * time.Minute,
		},
	}, cfg)
}
