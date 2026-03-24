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

// ComponentMetadata can be implemented by, for example, consumers exported by components, to provide the ID of the component which is exporting given consumer. This is used for the graph and the live debugging.
type ComponentMetadata interface {
	ComponentID() string
}

// GetComponentMetadata returns a list of ComponentMetadata from a list of Consumers.
func GetComponentMetadata(cons []Consumer) []ComponentMetadata {
	metadata := make([]ComponentMetadata, 0, len(cons))
	for _, cons := range cons {
		if consWithID, ok := cons.(ComponentMetadata); ok {
			metadata = append(metadata, consWithID)
		}
	}
	return metadata
}

// ConsumerArguments is a common Arguments type for Alloy components which can
// send data to otelcol consumers.
//
// It is expected to use ConsumerArguments as a block within the top-level
// arguments block for a component.
type ConsumerArguments struct {
	Metrics []Consumer `alloy:"metrics,attr,optional"`
	Logs    []Consumer `alloy:"logs,attr,optional"`
	Traces  []Consumer `alloy:"traces,attr,optional"`
}

// ConsumerExports is a common Exports type for Alloy components which are
// otelcol processors or otelcol exporters.
type ConsumerExports struct {
	Input Consumer `alloy:"input,attr"`
}
