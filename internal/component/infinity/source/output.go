package source

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol"
)

const scopeName = "infinity.source"

// outputs is the only place that knows about output types.
type outputs struct {
	prom        storage.Appendable
	loki        *loki.Fanout
	otelMetrics []otelcol.Consumer
	otelLogs    []otelcol.Consumer
}

// sendMetrics sends samples and stale markers with one timestamp.
func (o outputs) sendMetrics(ctx context.Context, job, instance string, ts time.Time, samples []sample, stale []labels.Labels) error {
	var errs []error

	app := o.prom.Appender(ctx)
	t := ts.UnixMilli()
	staleNaN := math.Float64frombits(value.StaleNaN)

	total := len(samples) + len(stale)
	var appendFailures int
	var firstAppendErr error
	appendOne := func(ls labels.Labels, v float64) {
		if _, err := app.Append(0, ls, t, v); err != nil {
			appendFailures++
			if firstAppendErr == nil {
				firstAppendErr = err
			}
		}
	}
	for _, s := range samples {
		appendOne(s.labels, s.value)
	}
	for _, ls := range stale {
		appendOne(ls, staleNaN)
	}
	if appendFailures > 0 {
		// One error per failed append would flood the log. Report the
		// count and the first error instead.
		errs = append(errs, fmt.Errorf("%d of %d appends failed, first: %w", appendFailures, total, firstAppendErr))
	}

	if err := app.Commit(); err != nil {
		errs = append(errs, err)
	}

	// Build a new batch for each consumer, because a consumer can change it.
	for _, c := range o.otelMetrics {
		if err := c.ConsumeMetrics(ctx, toMetrics(job, instance, ts, samples, stale)); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func toMetrics(job, instance string, ts time.Time, samples []sample, stale []labels.Labels) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	setResource(rm.Resource(), job, instance)
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName(scopeName)

	gauges := map[string]pmetric.Gauge{}
	add := func(ls labels.Labels, v float64, isStale bool) {
		name := ls.Get(model.MetricNameLabel)
		g, ok := gauges[name]
		if !ok {
			m := sm.Metrics().AppendEmpty()
			m.SetName(name)
			g = m.SetEmptyGauge()
			gauges[name] = g
		}
		dp := g.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		dp.SetDoubleValue(v)
		if isStale {
			// otelcol.receiver.prometheus uses the same flag for stale markers.
			dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
		}
		ls.Range(func(l labels.Label) {
			switch l.Name {
			case model.MetricNameLabel, model.JobLabel, model.InstanceLabel:
				return
			}
			dp.Attributes().PutStr(l.Name, l.Value)
		})
	}
	for _, s := range samples {
		add(s.labels, s.value, false)
	}
	for _, ls := range stale {
		add(ls, math.NaN(), true)
	}
	return md
}

// sendLogs sends entries to Loki receivers and OTel consumers.
func (o outputs) sendLogs(ctx context.Context, job, instance string, observed time.Time, entries []entry) error {
	var errs []error
	for _, e := range entries {
		le := loki.NewEntry(e.labels.Clone(), push.Entry{Timestamp: e.ts, Line: e.line, StructuredMetadata: e.structuredMetadata})
		if err := o.loki.Send(ctx, le); err != nil {
			// Send fails only when ctx ends, so the other entries fail too.
			errs = append(errs, err)
			break
		}
	}
	if len(entries) > 0 {
		for _, c := range o.otelLogs {
			if err := c.ConsumeLogs(ctx, toLogs(job, instance, observed, entries)); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func toLogs(job, instance string, observed time.Time, entries []entry) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	setResource(rl.Resource(), job, instance)
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName(scopeName)
	for _, e := range entries {
		rec := sl.LogRecords().AppendEmpty()
		rec.Body().SetStr(e.line)
		rec.SetTimestamp(pcommon.NewTimestampFromTime(e.ts))
		rec.SetObservedTimestamp(pcommon.NewTimestampFromTime(observed))
		if level, ok := e.labels["level"]; ok {
			rec.SetSeverityText(string(level))
		}
		for k, v := range e.labels {
			if k == model.JobLabel || k == model.InstanceLabel {
				continue
			}
			rec.Attributes().PutStr(string(k), string(v))
		}
		for _, m := range e.structuredMetadata {
			rec.Attributes().PutStr(m.Name, m.Value)
		}
	}
	return ld
}

// setResource maps job and instance the same way otelcol.receiver.prometheus does.
func setResource(r pcommon.Resource, job, instance string) {
	r.Attributes().PutStr("service.name", job)
	r.Attributes().PutStr("service.instance.id", instance)
}
