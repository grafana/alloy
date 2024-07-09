// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/DataDog/agent-payload/v5/gogen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/multierr"
)

func TestDatadogReceiver_Lifecycle(t *testing.T) {

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Endpoint = "localhost:0"
	ddr, err := factory.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err, "Receiver should be created")

	err = ddr.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err, "Server should start")

	err = ddr.Shutdown(context.Background())
	assert.NoError(t, err, "Server should stop")
}

func TestDatadogMetricsReceiver_Lifecycle(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Endpoint = "localhost:0"
	ddr, err := factory.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumertest.NewNop())

	assert.NoError(t, err, "Receiver should be created")

	err = ddr.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err, "Server should start")

	err = ddr.Shutdown(context.Background())
	assert.NoError(t, err, "Server should stop")
}

func TestDatadogServer(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:0" // Using a randomly assigned address
	dd, err := newDataDogReceiver(
		cfg,
		receivertest.NewNopCreateSettings(),
	)
	require.NoError(t, err, "Must not error when creating receiver")
	dd.(*datadogReceiver).nextTracesConsumer = consumertest.NewNop()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	require.NoError(t, dd.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, dd.Shutdown(ctx), "Must not error shutting down")
	})

	for _, tc := range []struct {
		name string
		op   io.Reader

		expectCode    int
		expectContent string
	}{
		{
			name:          "invalid data",
			op:            strings.NewReader("{"),
			expectCode:    http.StatusBadRequest,
			expectContent: "Unable to unmarshal reqs\n",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequest(
				http.MethodPost,
				fmt.Sprintf("http://%s/v0.7/traces", dd.(*datadogReceiver).address),
				tc.op,
			)
			require.NoError(t, err, "Must not error when creating request")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err, "Must not error performing request")

			actual, err := io.ReadAll(resp.Body)
			require.NoError(t, errors.Join(err, resp.Body.Close()), "Must not error when reading body")

			assert.Equal(t, tc.expectContent, string(actual))
			assert.Equal(t, tc.expectCode, resp.StatusCode, "Must match the expected status code")
		})
	}
}

func TestDatadogSketches_EndToEnd(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:0" // Using a randomly assigned address
	sink := new(consumertest.MetricsSink)

	dd, err := newDataDogReceiver(
		cfg,
		receivertest.NewNopCreateSettings(),
	)
	require.NoError(t, err, "Must not error when creating receiver")
	dd.(*datadogReceiver).nextMetricsConsumer = sink

	require.NoError(t, dd.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, dd.Shutdown(context.Background()))
	}()

	sketchPayload := gogen.SketchPayload{
		Sketches: []gogen.SketchPayload_Sketch{
			{
				Metric:        "Test1",
				Host:          "Host1",
				Tags:          []string{"env:tag1", "version:tag2"},
				Distributions: []gogen.SketchPayload_Sketch_Distribution{},
				Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
					{
						Ts:  400,
						Cnt: 13,
						Min: -6.0,
						Max: 6.0,
						Avg: 1.0,
						Sum: 11.0,
						K:   []int32{-1442, -1427, -1409, -1383, -1338, 0, 1338, 1383, 1409, 1427, 1442, 1454, 1464},
						N:   []uint32{152, 124, 68, 231, 97, 55, 101, 239, 66, 43, 167, 209, 154},
					},
				},
			},
		},
	}

	pb, err := sketchPayload.Marshal()
	assert.NoError(t, err)

	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("http://%s/api/beta/sketches", dd.(*datadogReceiver).address),
		io.NopCloser(bytes.NewReader(pb)),
	)
	require.NoError(t, err, "Must not error when creating request")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Must not error performing request")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, multierr.Combine(err, resp.Body.Close()), "Must not error when reading body")
	require.Equal(t, string(body), "OK", "Expected response to be 'OK', got %s", string(body))
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	mds := sink.AllMetrics()
	require.Len(t, mds, 1)
	got := mds[0]
	require.Equal(t, 1, got.ResourceMetrics().Len())
	metrics := got.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	assert.Equal(t, 1, metrics.Len())
	metric := metrics.At(0)
	assert.Equal(t, pmetric.MetricTypeExponentialHistogram, metric.Type())
	assert.Equal(t, "Test1", metric.Name())
	assert.Equal(t, pmetric.AggregationTemporalityDelta, metric.ExponentialHistogram().AggregationTemporality())
	assert.Equal(t, pcommon.Timestamp(400*1_000_000_000), metric.ExponentialHistogram().DataPoints().At(0).Timestamp())
	assert.Equal(t, uint64(13), metric.ExponentialHistogram().DataPoints().At(0).Count())
	assert.Equal(t, 11.0, metric.ExponentialHistogram().DataPoints().At(0).Sum())
	assert.Equal(t, -6.0, metric.ExponentialHistogram().DataPoints().At(0).Min())
	assert.Equal(t, 6.0, metric.ExponentialHistogram().DataPoints().At(0).Max())
	assert.Equal(t, int32(scale), metric.ExponentialHistogram().DataPoints().At(0).Scale())
	assert.Equal(t, uint64(55), metric.ExponentialHistogram().DataPoints().At(0).ZeroCount())
	assert.Equal(t, 91, metric.ExponentialHistogram().DataPoints().At(0).Positive().BucketCounts().Len())
	expectedPositiveInputBuckets := map[int]uint64{64: 26, 74: 131, 75: 36, 0: 101, 32: 239, 50: 16, 51: 50, 63: 17, 83: 209, 90: 154}
	for k, v := range metric.ExponentialHistogram().DataPoints().At(0).Positive().BucketCounts().AsRaw() {
		assert.Equal(t, expectedPositiveInputBuckets[k], v)
	}
	assert.Equal(t, 76, metric.ExponentialHistogram().DataPoints().At(0).Negative().BucketCounts().Len())
	expectedNegativeInputBuckets := map[int]uint64{74: 119, 75: 33, 63: 51, 64: 73, 50: 17, 51: 51, 32: 231, 0: 97}
	for k, v := range metric.ExponentialHistogram().DataPoints().At(0).Negative().BucketCounts().AsRaw() {
		assert.Equal(t, expectedNegativeInputBuckets[k], v)
	}
}

func TestDatadogServices_EndToEnd(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:0" // Using a randomly assigned address
	sink := new(consumertest.MetricsSink)

	dd, err := newDataDogReceiver(
		cfg,
		receivertest.NewNopCreateSettings(),
	)
	require.NoError(t, err, "Must not error when creating receiver")
	dd.(*datadogReceiver).nextMetricsConsumer = sink

	require.NoError(t, dd.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, dd.Shutdown(context.Background()))
	}()

	servicesPayload := []byte(`[
                {
                	"check": "app.working",
			"host_name": "hosta",
			"status": 2,
			"tags": ["environment:test"]
		}
        ]`)

	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("http://%s/api/v1/check_run", dd.(*datadogReceiver).address),
		io.NopCloser(bytes.NewReader(servicesPayload)),
	)
	require.NoError(t, err, "Must not error when creating request")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Must not error performing request")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, multierr.Combine(err, resp.Body.Close()), "Must not error when reading body")
	require.Equal(t, string(body), "OK", "Expected response to be 'OK', got %s", string(body))
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	mds := sink.AllMetrics()
	require.Len(t, mds, 1)
	got := mds[0]
	require.Equal(t, 1, got.ResourceMetrics().Len())
	metrics := got.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	assert.Equal(t, 1, metrics.Len())
	metric := metrics.At(0)
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	dps := metric.Gauge().DataPoints()
	assert.Equal(t, 1, dps.Len())
	dp := dps.At(0)
	assert.Equal(t, int64(2), dp.IntValue())
	assert.Equal(t, 1, dp.Attributes().Len())
	environment, _ := dp.Attributes().Get("environment")
	assert.Equal(t, "test", environment.AsString())
	hostName, _ := got.ResourceMetrics().At(0).Resource().Attributes().Get("host.name")
	assert.Equal(t, "hosta", hostName.AsString())
}

func TestDatadogMetricsV1_EndToEnd(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:0" // Using a randomly assigned address
	sink := new(consumertest.MetricsSink)

	dd, err := newDataDogReceiver(
		cfg,
		receivertest.NewNopCreateSettings(),
	)
	require.NoError(t, err, "Must not error when creating receiver")
	dd.(*datadogReceiver).nextMetricsConsumer = sink

	require.NoError(t, dd.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, dd.Shutdown(context.Background()))
	}()

	metricsPayloadV1 := []byte(`{
		"series": [
			{
				"metric": "system.load.1",
				"host": "testHost",
				"type": "count",
				"points": [[1636629071,0.7]],
				"source_type_name": "kubernetes",
				"tags": ["environment:test"]
			}
		]
	}`)

	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("http://%s/api/v1/series", dd.(*datadogReceiver).address),
		io.NopCloser(bytes.NewReader(metricsPayloadV1)),
	)
	require.NoError(t, err, "Must not error when creating request")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Must not error performing request")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, multierr.Combine(err, resp.Body.Close()), "Must not error when reading body")
	require.Equal(t, string(body), "OK", "Expected response to be 'OK', got %s", string(body))
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	mds := sink.AllMetrics()
	require.Len(t, mds, 1)
	got := mds[0]
	require.Equal(t, 1, got.ResourceMetrics().Len())
	metrics := got.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	assert.Equal(t, 1, metrics.Len())
	metric := metrics.At(0)
	assert.Equal(t, pmetric.MetricTypeSum, metric.Type())
	assert.Equal(t, "system.load.1", metric.Name())
	assert.Equal(t, pmetric.AggregationTemporalityDelta, metric.Sum().AggregationTemporality())
	assert.Equal(t, false, metric.Sum().IsMonotonic())
	assert.Equal(t, pcommon.Timestamp(1636629071*1_000_000_000), metric.Sum().DataPoints().At(0).Timestamp())
	assert.Equal(t, 0.7, metric.Sum().DataPoints().At(0).DoubleValue())
	expectedEnvironment, _ := metric.Sum().DataPoints().At(0).Attributes().Get("environment")
	assert.Equal(t, "test", expectedEnvironment.AsString())
}

func TestDatadogMetricsV2_EndToEnd(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:0" // Using a randomly assigned address
	sink := new(consumertest.MetricsSink)

	dd, err := newDataDogReceiver(
		cfg,
		receivertest.NewNopCreateSettings(),
	)
	require.NoError(t, err, "Must not error when creating receiver")
	dd.(*datadogReceiver).nextMetricsConsumer = sink

	require.NoError(t, dd.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, dd.Shutdown(context.Background()))
	}()

	metricsPayloadV2 := gogen.MetricPayload{
		Series: []*gogen.MetricPayload_MetricSeries{
			{
				Resources: []*gogen.MetricPayload_Resource{
					{
						Type: "host",
						Name: "Host1",
					},
				},
				Metric: "system.load.1",
				Tags:   []string{"env:test"},
				Points: []*gogen.MetricPayload_MetricPoint{
					{
						Timestamp: 1636629071,
						Value:     1.5,
					},
					{
						Timestamp: 1636629081,
						Value:     2.0,
					},
				},
				Type: gogen.MetricPayload_COUNT,
			},
		},
	}

	pb, err := metricsPayloadV2.Marshal()
	assert.NoError(t, err)

	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("http://%s/api/v2/series", dd.(*datadogReceiver).address),
		io.NopCloser(bytes.NewReader(pb)),
	)
	require.NoError(t, err, "Must not error when creating request")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "Must not error performing request")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, multierr.Combine(err, resp.Body.Close()), "Must not error when reading body")
	require.Equal(t, string(body), "OK", "Expected response to be 'OK', got %s", string(body))
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	mds := sink.AllMetrics()
	require.Len(t, mds, 1)
	got := mds[0]
	require.Equal(t, 1, got.ResourceMetrics().Len())
	metrics := got.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	assert.Equal(t, 1, metrics.Len())
	metric := metrics.At(0)
	assert.Equal(t, pmetric.MetricTypeSum, metric.Type())
	assert.Equal(t, "system.load.1", metric.Name())
	assert.Equal(t, pmetric.AggregationTemporalityDelta, metric.Sum().AggregationTemporality())
	assert.Equal(t, false, metric.Sum().IsMonotonic())
	assert.Equal(t, pcommon.Timestamp(1636629071*1_000_000_000), metric.Sum().DataPoints().At(0).Timestamp())
	assert.Equal(t, 1.5, metric.Sum().DataPoints().At(0).DoubleValue())
	assert.Equal(t, pcommon.Timestamp(0), metric.Sum().DataPoints().At(0).StartTimestamp())
	assert.Equal(t, pcommon.Timestamp(1636629081*1_000_000_000), metric.Sum().DataPoints().At(1).Timestamp())
	assert.Equal(t, 2.0, metric.Sum().DataPoints().At(1).DoubleValue())
	assert.Equal(t, pcommon.Timestamp(1636629071*1_000_000_000), metric.Sum().DataPoints().At(1).StartTimestamp())
}
