package fanoutconsumer

// This file is a near copy of
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.54.0/service/internal/fanoutconsumer/traces.go
//
// A copy was made because the upstream package is internal. If it is ever made
// public, our copy can be removed.

import (
	"context"

	"github.com/grafana/agent/internal/component/otelcol"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
)

// Traces creates a new fanout consumer for traces.
func Traces(in []otelcol.Consumer) otelconsumer.Traces {
	if len(in) == 0 {
		return &tracesFanout{}
	} else if len(in) == 1 {
		return in[0]
	}

	var passthrough, clone []otelconsumer.Traces

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

	return &tracesFanout{
		passthrough: passthrough,
		clone:       clone,
	}
}

type tracesFanout struct {
	passthrough []otelconsumer.Traces // Consumers where data can be passed through directly
	clone       []otelconsumer.Traces // Consumes which require cloning data
}

func (f *tracesFanout) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{MutatesData: false}
}

// ConsumeTraces exports the pmetric.Traces to all consumers wrapped by the current one.
func (f *tracesFanout) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var errs error

	// Initially pass to clone exporter to avoid the case where the optimization
	// of sending the incoming data to a mutating consumer is used that may
	// change the incoming data before cloning.
	for _, f := range f.clone {
		newTraces := ptrace.NewTraces()
		td.CopyTo(newTraces)
		errs = multierr.Append(errs, f.ConsumeTraces(ctx, newTraces))
	}
	for _, f := range f.passthrough {
		errs = multierr.Append(errs, f.ConsumeTraces(ctx, td))
	}

	return errs
}
