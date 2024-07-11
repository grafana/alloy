// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func totalHistBucketCounts(hist pmetric.ExponentialHistogramDataPoint) uint64 {
	var totalCount uint64
	for i := 0; i < hist.Negative().BucketCounts().Len(); i++ {
		totalCount += hist.Negative().BucketCounts().At(i)
	}

	totalCount += hist.ZeroCount()
	for i := 0; i < hist.Positive().BucketCounts().Len(); i++ {
		totalCount += hist.Positive().BucketCounts().At(i)
	}
	return totalCount
}

func createMetricsTranslator() *MetricsTranslator {
	mt := newMetricsTranslator()
	mt.buildInfo = component.BuildInfo{
		Command:     "otelcol",
		Description: "OpenTelemetry Collector",
		Version:     "latest",
	}
	return mt
}

func requireResourceMetrics(t *testing.T, result pmetric.Metrics, expectedAttrs pcommon.Map, expectedLen int) {
	require.Equal(t, expectedLen, result.ResourceMetrics().Len())
	require.Equal(t, expectedAttrs, result.ResourceMetrics().At(0).Resource().Attributes())
}

func requireScopeMetrics(t *testing.T, result pmetric.Metrics, expectedScopeMetricsLen, expectedMetricsLen int) {
	require.Equal(t, expectedScopeMetricsLen, result.ResourceMetrics().At(0).ScopeMetrics().Len())
	require.Equal(t, expectedMetricsLen, result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
}

func requireScope(t *testing.T, result pmetric.Metrics, expectedAttrs pcommon.Map, expectedName, expectedVersion string) {
	require.Equal(t, expectedName, result.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Name())
	require.Equal(t, expectedVersion, result.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Version())
	require.Equal(t, expectedAttrs, result.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes())
}

func requireMetricAndDataPointCounts(t *testing.T, result pmetric.Metrics, expectedMetricCount, expectedDpCount int) {
	require.Equal(t, expectedMetricCount, result.MetricCount())
	require.Equal(t, expectedDpCount, result.DataPointCount())
}

func requireSum(t *testing.T, metric pmetric.Metric, expectedName string, expectedAggregationTemporality pmetric.AggregationTemporality, expectedDpsLen int) {
	require.Equal(t, expectedName, metric.Name())
	require.Equal(t, pmetric.MetricTypeSum, metric.Type())
	require.Equal(t, expectedAggregationTemporality, metric.Sum().AggregationTemporality())
	require.Equal(t, expectedDpsLen, metric.Sum().DataPoints().Len())
}

func requireGauge(t *testing.T, metric pmetric.Metric, expectedName string, expectedDpsLen int) {
	require.Equal(t, expectedName, metric.Name())
	require.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	require.Equal(t, expectedDpsLen, metric.Gauge().DataPoints().Len())
}

func requireDp(t *testing.T, dp pmetric.NumberDataPoint, expectedAttrs pcommon.Map, expectedTime int64, expectedValue float64) {
	require.Equal(t, expectedTime, dp.Timestamp().AsTime().Unix())
	require.Equal(t, expectedValue, dp.DoubleValue())
	require.Equal(t, expectedAttrs, dp.Attributes())
}
