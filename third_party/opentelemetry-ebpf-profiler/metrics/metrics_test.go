// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/metric/noop"
)

// TestMetrics
func TestMetrics(t *testing.T) {
	// NOP report to avoid timing-based interference.
	report = func() {}

	inputMetrics := []Metric{
		{IDELFInfoCacheHit, MetricValue(33)},
		{IDELFInfoCacheMiss, MetricValue(55)},
		{IDErrProcESRCH, MetricValue(66)},
		{IDErrProcNotExist, MetricValue(20)},
		{IDUnwindCallInterpreter, MetricValue(0)},
	}

	meter := noop.NewMeterProvider().Meter("test")
	Start(meter)

	AddSlice(inputMetrics[0:2])                    // 33, 55
	Add(inputMetrics[1].ID, inputMetrics[1].Value) // 55, dropped
	Add(inputMetrics[2].ID, inputMetrics[2].Value) // 66
	AddSlice(inputMetrics[3:4])                    // 20
	Add(inputMetrics[0].ID, inputMetrics[0].Value) // 33, dropped
	AddSlice(inputMetrics[1:3])                    // 55, 66 dropped
	AddSlice(inputMetrics[2:5])                    // 66 dropped, 20 dropped, 0 dropped
	// Drop counter with 0 value as we don't expect it to appear in output
	inputMetrics = inputMetrics[:4]

	outputMetrics := make([]Metric, nMetrics)
	for j := range nMetrics {
		outputMetrics[j].ID = metricsBuffer[j].ID
		outputMetrics[j].Value = metricsBuffer[j].Value
	}
	assert.Equal(t, inputMetrics, outputMetrics)
}

func TestGetDefinitions(t *testing.T) {
	defs := GetDefinitions()
	assert.Greater(t, len(defs), 1)
}
