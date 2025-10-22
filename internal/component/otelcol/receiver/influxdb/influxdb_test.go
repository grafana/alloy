// influxdb_test.go
package influxdb_test

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/influxdb"
	alloycomponenttest "github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/syntax"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb1 "github.com/influxdata/influxdb1-client/v2"
	influxdbreceiver "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

type mockConsumer struct {
	lastMetricsConsumed pmetric.Metrics
}

func (m *mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (m *mockConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	m.lastMetricsConsumed = pmetric.NewMetrics()
	md.CopyTo(m.lastMetricsConsumed)
	return nil
}

// provided channel.
func makeMetricsOutput(ch chan pmetric.Metrics) *otelcol.ConsumerArguments {
	metricConsumer := fakeconsumer.Consumer{
		ConsumeMetricsFunc: func(ctx context.Context, t pmetric.Metrics) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- t:
				return nil
			}
		},
	}

	return &otelcol.ConsumerArguments{
		Metrics: []otelcol.Consumer{&metricConsumer},
	}
}

// TestInfluxdbUnmarshal tests the unmarshaling of the Alloy configuration into influxdb.Arguments
func TestInfluxdbUnmarshal(t *testing.T) {
	metricCh := make(chan pmetric.Metrics)

	influxdbConfig := `
    endpoint = "localhost:8086"
    compression_algorithms = ["gzip", "zstd"]

    debug_metrics {
        disable_high_cardinality_metrics = false
    }
    output {
    }
    `
	var args influxdb.Arguments
	err := syntax.Unmarshal([]byte(influxdbConfig), &args)
	require.NoError(t, err, "Unmarshaling should not produce an error")

	// Set up the metrics output for testing
	args.Output = makeMetricsOutput(metricCh)

	// Validate HTTPServer block
	assert.Equal(t, "localhost:8086", args.HTTPServer.Endpoint, "HTTPServer.Endpoint should match")
	assert.ElementsMatch(t, []string{"gzip", "zstd"}, args.HTTPServer.CompressionAlgorithms, "HTTPServer.CompressionAlgorithms should match")

	// Validate debug_metrics block
	assert.Equal(t, false, args.DebugMetrics.DisableHighCardinalityMetrics, "DebugMetrics.DisableHighCardinalityMetrics should be false")

	// Validate output block
	require.NotNil(t, args.Output, "Output block should not be nil")
	require.Len(t, args.Output.Metrics, 1, "There should be exactly one metrics output")
}

// TestWriteLineProtocol_Alloy tests the InfluxDB receiver's ability to process metrics
func TestWriteLineProtocol_Alloy(t *testing.T) {
	addr := alloycomponenttest.GetFreeAddr(t)
	config := &influxdbreceiver.Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: addr,
		},
	}
	nextConsumer := new(mockConsumer)

	receiver, outerErr := influxdbreceiver.NewFactory().CreateMetrics(t.Context(), receivertest.NewNopSettings(component.MustNewType("influxdb")), config, nextConsumer)
	require.NoError(t, outerErr)
	require.NotNil(t, receiver)

	require.NoError(t, receiver.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, receiver.Shutdown(t.Context())) })

	// Send test data using InfluxDB client v1
	t.Run("influxdb-client-v1", func(t *testing.T) {
		nextConsumer.lastMetricsConsumed = pmetric.NewMetrics()

		client, err := influxdb1.NewHTTPClient(influxdb1.HTTPConfig{
			Addr:    "http://" + addr,
			Timeout: time.Second,
		})
		require.NoError(t, err)

		batchPoints, err := influxdb1.NewBatchPoints(influxdb1.BatchPointsConfig{Precision: "µs"})
		require.NoError(t, err)
		point, err := influxdb1.NewPoint("cpu_temp", map[string]string{"foo": "bar"}, map[string]any{"gauge": 87.332})
		require.NoError(t, err)
		batchPoints.AddPoint(point)
		err = client.Write(batchPoints)
		require.NoError(t, err)

		metrics := nextConsumer.lastMetricsConsumed
		assert.NotNil(t, metrics)
		assert.Equal(t, 1, metrics.MetricCount())
		metric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		assert.Equal(t, "cpu_temp", metric.Name())
		assert.InEpsilon(t, 87.332, metric.Gauge().DataPoints().At(0).DoubleValue(), 0.001)
	})

	// Send test data using InfluxDB client v2
	t.Run("influxdb-client-v2", func(t *testing.T) {
		nextConsumer.lastMetricsConsumed = pmetric.NewMetrics()

		o := influxdb2.DefaultOptions()
		o.SetPrecision(time.Microsecond)
		client := influxdb2.NewClientWithOptions("http://"+addr, "", o)
		t.Cleanup(client.Close)

		err := client.WriteAPIBlocking("my-org", "my-bucket").WriteRecord(t.Context(), "cpu_temp,foo=bar gauge=87.332")
		require.NoError(t, err)

		metrics := nextConsumer.lastMetricsConsumed
		assert.NotNil(t, metrics)
		assert.Equal(t, 1, metrics.MetricCount())
		metric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		assert.Equal(t, "cpu_temp", metric.Name())
		assert.InEpsilon(t, 87.332, metric.Gauge().DataPoints().At(0).DoubleValue(), 0.001)
	})
}
func TestReceiverStart(t *testing.T) {
	addr := alloycomponenttest.GetFreeAddr(t)
	metricCh := make(chan pmetric.Metrics)
	config := influxdb.Arguments{
		HTTPServer: otelcol.HTTPServerArguments{
			Endpoint:              addr,
			CompressionAlgorithms: []string{"gzip", "zstd"},
		},
		Output: makeMetricsOutput(metricCh),
	}

	convertedConfig, err := config.Convert()
	require.NoError(t, err, "Failed to convert configuration")

	receiver, err := influxdbreceiver.NewFactory().CreateMetrics(
		t.Context(),
		receivertest.NewNopSettings(component.MustNewType("influxdb")),
		convertedConfig,
		new(mockConsumer),
	)
	require.NoError(t, err, "Failed to create receiver")

	require.NoError(t, receiver.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, receiver.Shutdown(t.Context())) }()

	require.NoError(t, nil, "Receiver failed to start")
}
func TestReceiverProcessesMetrics(t *testing.T) {
	addr := alloycomponenttest.GetFreeAddr(t)
	nextConsumer := &mockConsumer{}

	config := influxdb.Arguments{
		HTTPServer: otelcol.HTTPServerArguments{
			Endpoint:              addr,
			CompressionAlgorithms: []string{"gzip"},
		},
		Output: nil, // Output will not be used since we are directly testing the consumer
	}

	convertedConfig, err := config.Convert()
	require.NoError(t, err, "Failed to convert configuration")

	receiver, err := influxdbreceiver.NewFactory().CreateMetrics(
		t.Context(),
		receivertest.NewNopSettings(component.MustNewType("influxdb")),
		convertedConfig,
		nextConsumer,
	)
	require.NoError(t, err, "Failed to create receiver")

	require.NoError(t, receiver.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, receiver.Shutdown(t.Context())) }()

	t.Log("Receiver started successfully")

	// Simulate sending data to the receiver
	o := influxdb2.DefaultOptions().SetUseGZip(true)
	o.SetPrecision(time.Microsecond)
	client := influxdb2.NewClientWithOptions("http://"+addr, "", o)
	defer client.Close()

	t.Log("Sending test payload")
	err = client.WriteAPIBlocking("org", "bucket").WriteRecord(t.Context(), "cpu_temp,foo=bar gauge=87.332")
	require.NoError(t, err, "Failed to send metrics")

	// Validate the output
	t.Log("Waiting for metrics to be consumed")
	require.NotNil(t, nextConsumer.lastMetricsConsumed, "No metrics consumed")
	require.Equal(t, 1, nextConsumer.lastMetricsConsumed.MetricCount(), "Unexpected metric count")

	metric := nextConsumer.lastMetricsConsumed.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "cpu_temp", metric.Name())
	assert.InEpsilon(t, 87.332, metric.Gauge().DataPoints().At(0).DoubleValue(), 0.001)
}
