package fanoutconsumer

// This file is a near copy of
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.54.0/service/internal/fanoutconsumer/logs.go
//
// A copy was made because the upstream package is internal. If it is ever made
// public, our copy can be removed.

import (
	"context"

	"github.com/grafana/agent/internal/component/otelcol"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
)

// Logs creates a new fanout consumer for logs.
func Logs(in []otelcol.Consumer) otelconsumer.Logs {
	if len(in) == 0 {
		return &logsFanout{}
	} else if len(in) == 1 {
		return in[0]
	}

	var passthrough, clone []otelconsumer.Logs

	// Iterate through all the consumers besides the last.
	for i := 0; i < len(in)-1; i++ {
		consumer := in[i]

		if consumer == nil {
			continue
		}

		if consumer.Capabilities().MutatesData {
			clone = append(clone, consumer)
		} else {
			passthrough = append(passthrough, consumer)
		}
	}

	last := in[len(in)-1]

	// The final consumer can be given to the passthrough list regardless of
	// whether it mutates as long as there's no other read-only consumers.
	if last != nil {
		if len(passthrough) == 0 || !last.Capabilities().MutatesData {
			passthrough = append(passthrough, last)
		} else {
			clone = append(clone, last)
		}
	}

	return &logsFanout{
		passthrough: passthrough,
		clone:       clone,
	}
}

type logsFanout struct {
	passthrough []otelconsumer.Logs // Consumers where data can be passed through directly
	clone       []otelconsumer.Logs // Consumes which require cloning data
}

func (f *logsFanout) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{MutatesData: false}
}

// ConsumeLogs exports the pmetric.Logs to all consumers wrapped by the current one.
func (f *logsFanout) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var errs error

	// Initially pass to clone exporter to avoid the case where the optimization
	// of sending the incoming data to a mutating consumer is used that may
	// change the incoming data before cloning.
	for _, f := range f.clone {
		newLogs := plog.NewLogs()
		ld.CopyTo(newLogs)
		errs = multierr.Append(errs, f.ConsumeLogs(ctx, newLogs))
	}
	for _, f := range f.passthrough {
		errs = multierr.Append(errs, f.ConsumeLogs(ctx, ld))
	}

	return errs
}
