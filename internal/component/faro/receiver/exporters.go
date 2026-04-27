package receiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-logfmt/logfmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/faro/receiver/internal/payload"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/loki/pkg/push"
)

type exporter interface {
	Name() string
	Export(ctx context.Context, payload payload.Payload) error
}

//
// Metrics
//

type metricsExporter struct {
	totalLogs         prometheus.Counter
	totalMeasurements prometheus.Counter
	totalExceptions   prometheus.Counter
	totalEvents       prometheus.Counter
}

var _ exporter = (*metricsExporter)(nil)

func newMetricsExporter(reg prometheus.Registerer) *metricsExporter {
	exp := &metricsExporter{
		totalLogs: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "faro_receiver_logs_total",
			Help: "Total number of ingested logs",
		}),
		totalMeasurements: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "faro_receiver_measurements_total",
			Help: "Total number of ingested measurements",
		}),
		totalExceptions: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "faro_receiver_exceptions_total",
			Help: "Total number of ingested exceptions",
		}),
		totalEvents: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "faro_receiver_events_total",
			Help: "Total number of ingested events",
		}),
	}

	exp.totalLogs = util.MustRegisterOrGet(reg, exp.totalLogs).(prometheus.Counter)
	exp.totalMeasurements = util.MustRegisterOrGet(reg, exp.totalMeasurements).(prometheus.Counter)
	exp.totalExceptions = util.MustRegisterOrGet(reg, exp.totalExceptions).(prometheus.Counter)
	exp.totalEvents = util.MustRegisterOrGet(reg, exp.totalEvents).(prometheus.Counter)

	return exp
}

func (exp *metricsExporter) Name() string { return "receiver metrics exporter" }

func (exp *metricsExporter) Export(ctx context.Context, p payload.Payload) error {
	exp.totalExceptions.Add(float64(len(p.Exceptions)))
	exp.totalLogs.Add(float64(len(p.Logs)))
	exp.totalMeasurements.Add(float64(len(p.Measurements)))
	exp.totalEvents.Add(float64(len(p.Events)))
	return nil
}

//
// Logs
//

type logsExporter struct {
	log        log.Logger
	sourceMaps sourceMapsStore
	format     LogFormat

	fanout *loki.Fanout

	labelsMut sync.RWMutex
	labels    model.LabelSet
}

var _ exporter = (*logsExporter)(nil)

func newLogsExporter(log log.Logger, sourceMaps sourceMapsStore, format LogFormat) *logsExporter {
	return &logsExporter{
		log:        log,
		sourceMaps: sourceMaps,
		format:     format,
		fanout:     loki.NewFanout([]loki.LogsReceiver{}),
	}
}

// SetReceivers updates the set of logs receivers which will receive logs
// emitted by the exporter.
func (exp *logsExporter) SetReceivers(receivers []loki.LogsReceiver) {
	exp.fanout.UpdateChildren(receivers)
}

func (exp *logsExporter) Name() string { return "logs exporter" }

func (exp *logsExporter) Export(ctx context.Context, p payload.Payload) error {
	var (
		errs []error
		meta = p.Meta.KeyVal()
	)

	// send returns early on context cancellation.
	// Other errors are accumulated in errs.
	send := func(kv *payload.KeyVal) error {
		payload.MergeKeyVal(kv, meta)

		err := exp.sendKeyValsToLogsPipeline(ctx, kv)
		if errors.Is(err, context.Canceled) {
			return err
		}

		if err != nil {
			errs = append(errs, err)
		}

		return nil
	}

	// log events
	for _, logItem := range p.Logs {
		if err := send(logItem.KeyVal()); err != nil {
			return err
		}
	}

	// exceptions
	for _, exception := range p.Exceptions {
		transformedException := transformException(exp.log, exp.sourceMaps, &exception, p.Meta.App.Release)
		if err := send(transformedException.KeyVal()); err != nil {
			return err
		}
	}

	// measurements
	for _, measurement := range p.Measurements {
		if err := send(measurement.KeyVal()); err != nil {
			return err
		}
	}

	// events
	for _, event := range p.Events {
		if err := send(event.KeyVal()); err != nil {
			return err
		}
	}

	return errors.Join(errs...)
}

func (exp *logsExporter) sendKeyValsToLogsPipeline(ctx context.Context, kv *payload.KeyVal) error {
	var (
		line []byte
		err  error
	)
	switch exp.format {
	case FormatLogfmt:
		line, err = logfmt.MarshalKeyvals(payload.KeyValToInterfaceSlice(kv)...)
	case FormatJSON:
		line, err = json.Marshal(payload.KeyValToInterfaceMap(kv))
	default:
		line, err = logfmt.MarshalKeyvals(payload.KeyValToInterfaceSlice(kv)...)
	}

	if err != nil {
		level.Error(exp.log).Log("msg", "failed to logfmt a frontend log event", "err", err)
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second) // TODO(rfratto): potentially make this configurable
	defer cancel()
	return exp.fanout.Send(ctx, loki.NewEntry(exp.labelSet(kv), push.Entry{
		Timestamp: time.Now(),
		Line:      string(line),
	}))
}

func (exp *logsExporter) labelSet(kv *payload.KeyVal) model.LabelSet {
	exp.labelsMut.RLock()
	defer exp.labelsMut.RUnlock()

	// Attach extra label to log lines
	set := make(model.LabelSet, len(exp.labels))
	for k, v := range exp.labels {
		if len(v) > 0 {
			set[k] = v
		} else {
			if val, ok := kv.Get(string(k)); ok {
				set[k] = model.LabelValue(fmt.Sprint(val))
			}
		}
	}

	return set
}

func (exp *logsExporter) SetLabels(newLabels map[string]string) {
	exp.labelsMut.Lock()
	defer exp.labelsMut.Unlock()

	ls := make(model.LabelSet, len(newLabels))
	for k, v := range newLabels {
		ls[model.LabelName(k)] = model.LabelValue(v)
	}
	exp.labels = ls
}

//
// Traces
//

type tracesExporter struct {
	log log.Logger

	mut       sync.RWMutex
	consumers []otelcol.Consumer
}

var _ exporter = (*tracesExporter)(nil)

func newTracesExporter(log log.Logger) *tracesExporter {
	return &tracesExporter{
		log: log,
	}
}

// SetConsumers updates the set of OTLP consumers which will receive traces
// emitted by the exporter.
func (exp *tracesExporter) SetConsumers(consumers []otelcol.Consumer) {
	exp.mut.Lock()
	defer exp.mut.Unlock()

	exp.consumers = consumers
}

func (exp *tracesExporter) Name() string { return "traces exporter" }

func (exp *tracesExporter) Export(ctx context.Context, p payload.Payload) error {
	if p.Traces == nil {
		return nil
	}

	var errs []error
	for _, consumer := range exp.getTracesConsumers() {
		errs = append(errs, consumer.ConsumeTraces(ctx, p.Traces.Traces))
	}
	return errors.Join(errs...)
}

func (exp *tracesExporter) getTracesConsumers() []otelcol.Consumer {
	exp.mut.RLock()
	defer exp.mut.RUnlock()

	return exp.consumers
}
