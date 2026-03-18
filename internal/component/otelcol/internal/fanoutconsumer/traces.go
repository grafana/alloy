package fanoutconsumer

// This file is a near copy of
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.54.0/service/internal/fanoutconsumer/traces.go
//
// A copy was made because the upstream package is internal. If it is ever made
// public, our copy can be removed.

import (
	"context"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/prometheus/client_golang/prometheus"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
)

// Traces creates a new fanout consumer for traces.
func Traces(in []otelcol.Consumer, reg prometheus.Registerer) otelconsumer.Traces {
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "otelcol_forwarded_spans_total",
		Help: "Total number of spans forwarded to downstream components.",
	}, []string{"destination"})
	if err := reg.Register(counter); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			counter = are.ExistingCollector.(*prometheus.CounterVec)
		}
	}

	if len(in) == 0 {
		return &tracesFanout{spansCounter: counter}
	} else if len(in) == 1 {
		return &tracesPassthrough{
			consumer:     in[0],
			destID:       componentID(in[0]),
			spansCounter: counter,
		}
	}

	var passthrough, clone []otelconsumer.Traces

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
	if len(passthrough) == 0 || !last.Capabilities().MutatesData {
		passthrough = append(passthrough, last)
	} else {
		clone = append(clone, last)
	}

	return &tracesFanout{
		passthrough:  passthrough,
		clone:        clone,
		spansCounter: counter,
	}
}

type tracesPassthrough struct {
	consumer     otelconsumer.Traces
	destID       string
	spansCounter *prometheus.CounterVec
}

func (p *tracesPassthrough) Capabilities() otelconsumer.Capabilities {
	return p.consumer.Capabilities()
}

func (p *tracesPassthrough) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if err := p.consumer.ConsumeTraces(ctx, td); err != nil {
		return err
	}
	p.spansCounter.WithLabelValues(p.destID).Add(float64(td.SpanCount()))
	return nil
}

type tracesFanout struct {
	passthrough    []otelconsumer.Traces // Consumers where data can be passed through directly
	passthroughIDs []string
	clone          []otelconsumer.Traces // Consumes which require cloning data
	cloneIDs       []string
	spansCounter   *prometheus.CounterVec
}

func (f *tracesFanout) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{MutatesData: false}
}

// ConsumeTraces exports the pmetric.Traces to all consumers wrapped by the current one.
func (f *tracesFanout) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var errs error

	spanCount := td.SpanCount()

	// Initially pass to clone exporter to avoid the case where the optimization
	// of sending the incoming data to a mutating consumer is used that may
	// change the incoming data before cloning.
	for _, c := range f.clone {
		newTraces := ptrace.NewTraces()
		td.CopyTo(newTraces)
		if err := c.ConsumeTraces(ctx, newTraces); err != nil {
			errs = multierr.Append(errs, err)
		} else {
			f.spansCounter.WithLabelValues(componentID(c)).Add(float64(spanCount))
		}
	}
	for _, c := range f.passthrough {
		if err := c.ConsumeTraces(ctx, td); err != nil {
			errs = multierr.Append(errs, err)
		} else {
			f.spansCounter.WithLabelValues(componentID(c)).Add(float64(spanCount))
		}
	}

	return errs
}
