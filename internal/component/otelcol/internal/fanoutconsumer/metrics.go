package fanoutconsumer

// This file is a near copy of
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.54.0/service/internal/fanoutconsumer/metrics.go
//
// A copy was made because the upstream package is internal. If it is ever made
// public, our copy can be removed.

import (
	"context"

	"github.com/grafana/agent/internal/component/otelcol"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
)

// Metrics creates a new fanout consumer for metrics.
func Metrics(in []otelcol.Consumer) otelconsumer.Metrics {
	if len(in) == 0 {
		return &metricsFanout{}
	} else if len(in) == 1 {
		return in[0]
	}

	var passthrough, clone []otelconsumer.Metrics

	// Iterate through all the consumers besides the last.
	for i := 0; i < len(in)-1; i++ {
		consumer := in[i]

		if consumer.Capabilities().MutatesData {
			clone = append(clone, consumer)
		} else {
			passthrough = append(passthrough, consumer)
		}
	}

	last := in[len(in)-1]

	// The final consumer can be given to the passthrough list regardless of
	// whether it mutates as long as there's no other read-only consumers.
	if len(passthrough) == 0 || !last.Capabilities().MutatesData {
		passthrough = append(passthrough, last)
	} else {
		clone = append(clone, last)
	}

	return &metricsFanout{
		passthrough: passthrough,
		clone:       clone,
	}
}

type metricsFanout struct {
	passthrough []otelconsumer.Metrics // Consumers where data can be passed through directly
	clone       []otelconsumer.Metrics // Consumes which require cloning data
}

func (f *metricsFanout) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{MutatesData: false}
}

// ConsumeMetrics exports the pmetric.Metrics to all consumers wrapped by the current one.
func (f *metricsFanout) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var errs error

	// Initially pass to clone exporter to avoid the case where the optimization
	// of sending the incoming data to a mutating consumer is used that may
	// change the incoming data before cloning.
	for _, f := range f.clone {
		newMetrics := pmetric.NewMetrics()
		md.CopyTo(newMetrics)
		errs = multierr.Append(errs, f.ConsumeMetrics(ctx, newMetrics))
	}
	for _, f := range f.passthrough {
		errs = multierr.Append(errs, f.ConsumeMetrics(ctx, md))
	}

	return errs
}
