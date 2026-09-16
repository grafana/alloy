// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package livedebugging implements the live_debugging processor: a
// pass-through OTel Collector processor that tracks, without streaming any
// raw span data anywhere, which (namespace, service_name) sends the
// largest traces and which span names are most frequent. It forwards
// every batch to the next consumer unchanged.
package livedebugging // import "github.com/grafana/alloy/processor/livedebugging"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

var componentType = component.MustNewType("live_debugging")

// NewFactory returns a new factory for the live_debugging processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		componentType,
		createDefaultConfig,
		processor.WithTraces(createTraces, component.StabilityLevelDevelopment),
	)
}

func createTraces(_ context.Context, set processor.Settings, cfg component.Config, next consumer.Traces) (processor.Traces, error) {
	return newTracesProcessor(cfg.(*Config), set, next)
}
