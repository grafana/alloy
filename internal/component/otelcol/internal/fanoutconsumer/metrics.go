package fanoutconsumer

// This file is a near copy of
// https://github.com/open-telemetry/opentelemetry-collector/blob/v0.54.0/service/internal/fanoutconsumer/metrics.go
//
// A copy was made because the upstream package is internal. If it is ever made
// public, our copy can be removed.

import (
	"context"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/prometheus/client_golang/prometheus"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
)

// Metrics creates a new fanout consumer for metrics.
func Metrics(in []otelcol.Consumer, reg prometheus.Registerer) otelconsumer.Metrics {
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "otelcol_forwarded_metric_points_total",
		Help: "Total number of metric data points forwarded to downstream components.",
	}, []string{"destination"})
	if err := reg.Register(counter); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			counter = are.ExistingCollector.(*prometheus.CounterVec)
		}
	}

	if len(in) == 0 {
		return &metricsFanout{metricPointsCounter: counter}
	} else if len(in) == 1 {
		return &metricsPassthrough{
			consumer:            in[0],
			destID:              componentID(in[0]),
			metricPointsCounter: counter,
		}
	}

	var passthrough, clone []otelconsumer.Metrics

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

	return &metricsFanout{
		passthrough:         passthrough,
		clone:               clone,
		metricPointsCounter: counter,
	}
}

type metricsPassthrough struct {
	consumer            otelconsumer.Metrics
	destID              string
	metricPointsCounter *prometheus.CounterVec
}

func (p *metricsPassthrough) Capabilities() otelconsumer.Capabilities {
	return p.consumer.Capabilities()
}

func (p *metricsPassthrough) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if err := p.consumer.ConsumeMetrics(ctx, md); err != nil {
		return err
	}
	p.metricPointsCounter.WithLabelValues(p.destID).Add(float64(md.DataPointCount()))
	return nil
}

type metricsFanout struct {
	passthrough         []otelconsumer.Metrics // Consumers where data can be passed through directly
	clone               []otelconsumer.Metrics // Consumes which require cloning data
	metricPointsCounter *prometheus.CounterVec
}

func (f *metricsFanout) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{MutatesData: false}
}

// ConsumeMetrics exports the pmetric.Metrics to all consumers wrapped by the current one.
func (f *metricsFanout) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var errs error

	dataPointCount := md.DataPointCount()

	// Initially pass to clone exporter to avoid the case where the optimization
	// of sending the incoming data to a mutating consumer is used that may
	// change the incoming data before cloning.
	for _, c := range f.clone {
		newMetrics := pmetric.NewMetrics()
		md.CopyTo(newMetrics)
		if err := c.ConsumeMetrics(ctx, newMetrics); err != nil {
			errs = multierr.Append(errs, err)
		} else {
			f.metricPointsCounter.WithLabelValues(componentID(c)).Add(float64(dataPointCount))
		}
	}
	for _, c := range f.passthrough {
		if err := c.ConsumeMetrics(ctx, md); err != nil {
			errs = multierr.Append(errs, err)
		} else {
			f.metricPointsCounter.WithLabelValues(componentID(c)).Add(float64(dataPointCount))
		}
	}

	return errs
}
