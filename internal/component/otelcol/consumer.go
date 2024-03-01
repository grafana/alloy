package otelcol

import (
	otelconsumer "go.opentelemetry.io/collector/consumer"
)

// Consumer is a combined OpenTelemetry Collector consumer which can consume
// any telemetry signal.
type Consumer interface {
	otelconsumer.Traces
	otelconsumer.Metrics
	otelconsumer.Logs
}

// ConsumerArguments is a common Arguments type for Flow components which can
// send data to otelcol consumers.
//
// It is expected to use ConsumerArguments as a block within the top-level
// arguments block for a component.
type ConsumerArguments struct {
	Metrics []Consumer `river:"metrics,attr,optional"`
	Logs    []Consumer `river:"logs,attr,optional"`
	Traces  []Consumer `river:"traces,attr,optional"`
}

// ConsumerExports is a common Exports type for Flow components which are
// otelcol processors or otelcol exporters.
type ConsumerExports struct {
	Input Consumer `river:"input,attr"`
}
