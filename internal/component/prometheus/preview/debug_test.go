package preview

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util/testappender"
	"github.com/grafana/alloy/syntax/encoding/alloyjson"
)

func TestDebugInfoMatchesReport(t *testing.T) {
	sink := testappender.NewCollectingAppender()
	args := testArguments()
	args.GroupBy = []string{"job"}
	args.ReportTo = []storage.AppendableV2{testappender.ConstantAppendable{Inner: sink}}
	c := newTestComponent(t, args)
	require.Equal(t, debugInfo{}, c.DebugInfo())

	app := c.Appender(t.Context())
	_, err := app.Append(0, labels.FromStrings("__name__", "private_metric", "job", "api", "secret", "private_value"), 1, 123456)
	require.NoError(t, err)
	require.NoError(t, app.Commit())
	require.Equal(t, debugInfo{}, c.DebugInfo(), "reading debug info must not generate a report")
	require.Empty(t, sink.CollectedSamples())

	now := c.started.Add(time.Minute)
	c.now = func() time.Time { return now }
	require.NoError(t, c.report(t.Context()))
	info := c.DebugInfo().(debugInfo)
	require.True(t, info.ReportAvailable)
	require.Equal(t, now.UTC().Format(time.RFC3339Nano), info.GeneratedAt)
	got := sink.CollectedSamples()
	require.Len(t, info.Metrics, len(got))
	for _, metric := range info.Metrics {
		builder := labels.NewBuilder(labels.FromMap(metric.Labels))
		builder.Set(model.MetricNameLabel, metric.Name)
		key := builder.Labels().String()
		require.Contains(t, got, key)
		require.Equal(t, got[key].Value, metric.Value)
		require.Equal(t, string(sink.CollectedMetadata()[key].Type), metric.Type)
	}

	// Exercise the same encoding used by the component HTTP and GraphQL APIs.
	body, err := alloyjson.MarshalBody(info)
	require.NoError(t, err)
	require.True(t, json.Valid(body))
	require.Contains(t, string(body), "report_available")
	require.Contains(t, string(body), "generated_at")
	require.Contains(t, string(body), "prometheus_preview_samples_total")
	require.NotContains(t, string(body), "private_metric")
	require.NotContains(t, string(body), "private_value")
	require.NotContains(t, string(body), "123456")

	// Debug consumers own their data, independently of the report and other readers.
	info.Metrics[0].Labels["preview_id"] = "changed"
	info.Metrics[0].Name = "changed"
	info.Metrics[0].Value = -1
	again := c.DebugInfo().(debugInfo)
	require.Equal(t, c.opts.ID, again.Metrics[0].Labels["preview_id"])
	require.NotEqual(t, "changed", again.Metrics[0].Name)
	require.NotEqual(t, float64(-1), again.Metrics[0].Value)
}

func TestDebugInfoDoesNotAdvanceWindows(t *testing.T) {
	c := newTestComponent(t, testArguments())
	appendSample(t, c, "first")
	now := c.started.Add(c.args.Window)
	c.now = func() time.Time { return now }
	require.NoError(t, c.report(t.Context()))
	completed := c.DebugInfo().(debugInfo)
	for _, m := range completed.Metrics {
		if m.Name == "prometheus_preview_window_complete" {
			require.Equal(t, float64(1), m.Value)
		}
	}
	require.Empty(t, c.stats.series)

	appendSample(t, c, "next_window")
	windowStart := c.windowStarted
	now = now.Add(c.args.Window)
	for range 3 {
		require.Equal(t, completed, c.DebugInfo())
		require.Equal(t, windowStart, c.windowStarted)
		require.Equal(t, uint64(1), c.stats.total.series)
		require.Equal(t, uint64(2), c.stats.total.samples[0])
	}

	require.NoError(t, c.report(t.Context()))
	require.NotEqual(t, completed.GeneratedAt, c.DebugInfo().(debugInfo).GeneratedAt)
	require.Equal(t, now, c.windowStarted)
}

func TestDebugInfoAvailableWhenDeliveryFails(t *testing.T) {
	args := testArguments()
	args.ReportTo = []storage.AppendableV2{&failingReceiver{appendErr: errors.New("offline")}}
	c := newTestComponent(t, args)
	appendSample(t, c, "first")
	require.ErrorContains(t, c.report(t.Context()), "offline")
	require.True(t, c.DebugInfo().(debugInfo).ReportAvailable)
	require.NotEmpty(t, c.DebugInfo().(debugInfo).Metrics)
}

func TestDebugInfoClearedOnMeasurementReset(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Arguments)
	}{
		{"window", func(a *Arguments) { a.Window *= 2 }},
		{"group_by", func(a *Arguments) { a.GroupBy = []string{"job"} }},
		{"max_groups", func(a *Arguments) { a.MaxGroups++ }},
		{"max_series", func(a *Arguments) { a.MaxSeries++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := testArguments()
			c := newTestComponent(t, args)
			require.NoError(t, c.report(t.Context()))
			info := c.DebugInfo()
			// Delivery-only changes preserve the observation and cached report.
			args.ReportInterval *= 2
			args.ReportTo = []storage.AppendableV2{testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()}}
			require.NoError(t, c.Update(args))
			require.Equal(t, info, c.DebugInfo())
			tc.change(&args)
			require.NoError(t, c.Update(args))
			require.Equal(t, debugInfo{}, c.DebugInfo())
			require.NoError(t, c.report(t.Context()))
			require.True(t, c.DebugInfo().(debugInfo).ReportAvailable)
		})
	}
}
