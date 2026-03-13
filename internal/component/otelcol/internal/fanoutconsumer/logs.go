package fanoutconsumer

// This file is a near copy of
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.54.0/service/internal/fanoutconsumer/logs.go
//
// A copy was made because the upstream package is internal. If it is ever made
// public, our copy can be removed.

import (
	"context"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/prometheus/client_golang/prometheus"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
)

// Logs creates a new fanout consumer for logs.
func Logs(in []otelcol.Consumer, reg prometheus.Registerer) otelconsumer.Logs {
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "otelcol_forwarded_log_records_total",
		Help: "Total number of log records forwarded to downstream components.",
	}, []string{"destination"})
	if err := reg.Register(counter); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			counter = are.ExistingCollector.(*prometheus.CounterVec)
		}
	}

	if len(in) == 0 {
		return &logsFanout{logRecordsCounter: counter}
	} else if len(in) == 1 {
		return &logsPassthrough{
			consumer:          in[0],
			destID:            componentID(in[0]),
			logRecordsCounter: counter,
		}
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
		passthrough:       passthrough,
		clone:             clone,
		logRecordsCounter: counter,
	}
}

type logsPassthrough struct {
	consumer          otelconsumer.Logs
	destID            string
	logRecordsCounter *prometheus.CounterVec
}

func (p *logsPassthrough) Capabilities() otelconsumer.Capabilities {
	return p.consumer.Capabilities()
}

func (p *logsPassthrough) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if err := p.consumer.ConsumeLogs(ctx, ld); err != nil {
		return err
	}
	p.logRecordsCounter.WithLabelValues(p.destID).Add(float64(ld.LogRecordCount()))
	return nil
}

type logsFanout struct {
	passthrough       []otelconsumer.Logs // Consumers where data can be passed through directly
	clone             []otelconsumer.Logs // Consumes which require cloning data
	logRecordsCounter *prometheus.CounterVec
}

func (f *logsFanout) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{MutatesData: false}
}

// ConsumeLogs exports the pmetric.Logs to all consumers wrapped by the current one.
func (f *logsFanout) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var errs error

	logRecordCount := ld.LogRecordCount()

	// Initially pass to clone exporter to avoid the case where the optimization
	// of sending the incoming data to a mutating consumer is used that may
	// change the incoming data before cloning.
	for _, c := range f.clone {
		newLogs := plog.NewLogs()
		ld.CopyTo(newLogs)
		if err := c.ConsumeLogs(ctx, newLogs); err != nil {
			errs = multierr.Append(errs, err)
		} else {
			f.logRecordsCounter.WithLabelValues(componentID(c)).Add(float64(logRecordCount))
		}
	}
	for _, c := range f.passthrough {
		if err := c.ConsumeLogs(ctx, ld); err != nil {
			errs = multierr.Append(errs, err)
		} else {
			f.logRecordsCounter.WithLabelValues(componentID(c)).Add(float64(logRecordCount))
		}
	}

	return errs
}
