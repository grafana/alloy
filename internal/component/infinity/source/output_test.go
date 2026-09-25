package source

import (
	"math"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/util/testappender"
)

func TestSendMetricsPrometheus(t *testing.T) {
	app := testappender.NewCollectingAppender()
	o := outputs{prom: testappender.ConstantAppendable{Inner: app}, loki: loki.NewFanout(nil)}
	ts := time.UnixMilli(1_700_000_000_000)
	live := sample{labels: labels.FromStrings("__name__", "v", "job", "j", "instance", "i"), value: 3}
	gone := labels.FromStrings("__name__", "w", "job", "j", "instance", "i")

	require.NoError(t, o.sendMetrics(t.Context(), "j", "i", ts, []sample{live}, []labels.Labels{gone}))

	got := app.LatestSampleFor(live.labels.String())
	require.NotNil(t, got)
	require.Equal(t, 3.0, got.Value)
	require.Equal(t, ts.UnixMilli(), got.Timestamp)
	staleSample := app.LatestSampleFor(gone.String())
	require.NotNil(t, staleSample)
	require.True(t, value.IsStaleNaN(staleSample.Value))
}

func TestSendMetricsOTel(t *testing.T) {
	c := &testConsumer{}
	o := outputs{prom: testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()}, loki: loki.NewFanout(nil), otelMetrics: []otelcol.Consumer{c}}
	live := sample{labels: labels.FromStrings("__name__", "v", "job", "j", "instance", "i", "service", "api"), value: 3}
	gone := labels.FromStrings("__name__", "v", "job", "j", "instance", "i", "service", "db")

	require.NoError(t, o.sendMetrics(t.Context(), "j", "i", time.Now(), []sample{live}, []labels.Labels{gone}))

	mds, _ := c.snapshot()
	require.Len(t, mds, 1)
	rm := mds[0].ResourceMetrics().At(0)
	name, _ := rm.Resource().Attributes().Get("service.name")
	require.Equal(t, "j", name.Str())
	inst, _ := rm.Resource().Attributes().Get("service.instance.id")
	require.Equal(t, "i", inst.Str())

	m := rm.ScopeMetrics().At(0).Metrics().At(0)
	require.Equal(t, "v", m.Name())
	require.Equal(t, pmetric.MetricTypeGauge, m.Type())
	dps := m.Gauge().DataPoints()
	require.Equal(t, 2, dps.Len())

	svc, _ := dps.At(0).Attributes().Get("service")
	require.Equal(t, "api", svc.Str())
	_, hasJob := dps.At(0).Attributes().Get("job")
	require.False(t, hasJob)
	require.Equal(t, 3.0, dps.At(0).DoubleValue())
	require.False(t, dps.At(0).Flags().NoRecordedValue())

	require.True(t, dps.At(1).Flags().NoRecordedValue())
	require.True(t, math.IsNaN(dps.At(1).DoubleValue()))
}

func TestSendLogs(t *testing.T) {
	recv := loki.NewLogsReceiver()
	c := &testConsumer{}
	o := outputs{prom: testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()}, loki: loki.NewFanout([]loki.LogsReceiver{recv}), otelLogs: []otelcol.Consumer{c}}
	ts := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	e := entry{
		ts:                 ts,
		line:               "disk full",
		labels:             model.LabelSet{"job": "j", "instance": "i", "level": "error"},
		structuredMetadata: push.LabelsAdapter{{Name: "actor", Value: "bob"}},
	}

	errc := make(chan error, 1)
	go func() { errc <- o.sendLogs(t.Context(), "j", "i", ts, []entry{e}) }()

	select {
	case got := <-recv.Chan():
		require.Equal(t, e.labels, got.Labels)
		require.Equal(t, "disk full", got.Line)
		require.Equal(t, ts, got.Timestamp)
		require.Equal(t, e.structuredMetadata, got.StructuredMetadata)
	case <-time.After(time.Second):
		t.Fatal("no loki entry")
	}
	require.NoError(t, <-errc)

	_, lds := c.snapshot()
	require.Len(t, lds, 1)
	rec := lds[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	require.Equal(t, "disk full", rec.Body().Str())
	require.Equal(t, "error", rec.SeverityText())
	actor, _ := rec.Attributes().Get("actor")
	require.Equal(t, "bob", actor.Str())
	_, hasInstance := rec.Attributes().Get("instance")
	require.False(t, hasInstance)
}
