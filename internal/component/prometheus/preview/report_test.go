package preview

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/prometheus/appenders"
	"github.com/grafana/alloy/internal/util/testappender"
)

func TestReportOnlyContainsSummaries(t *testing.T) {
	sink := testappender.NewCollectingAppender()
	args := testArguments()
	args.GroupBy = []string{"job"}
	args.ReportTo = []storage.AppendableV2{testappender.ConstantAppendable{Inner: sink}}
	c := newTestComponent(t, args)
	app := c.Appender(t.Context())
	_, err := app.Append(0, labels.FromStrings("__name__", "private_metric", "job", "api", "token", "secret"), 1000, 123456)
	require.NoError(t, err)
	require.NoError(t, app.Commit())
	require.Empty(t, sink.CollectedSamples())

	now := c.started.Add(time.Minute)
	c.now = func() time.Time { return now }
	require.NoError(t, c.report(t.Context()))
	got := sink.CollectedSamples()
	require.Len(t, got, 16) // Four observation fields and four fields for each of total/group/overflow.
	for _, sample := range got {
		require.True(t, strings.HasPrefix(sample.Labels.Get("__name__"), "prometheus_preview_"))
		require.Equal(t, "prometheus.preview.candidate", sample.Labels.Get("preview_id"))
		require.NotContains(t, sample.Labels.String(), "private_metric")
		require.NotContains(t, sample.Labels.String(), "secret")
		require.NotContains(t, sample.Labels.String(), "token")
		require.NotEqual(t, float64(123456), sample.Value)
		require.Equal(t, now.UnixMilli(), sample.Timestamp)
	}
	require.Empty(t, sink.CollectedHistograms())
	require.Empty(t, sink.CollectedSTZeroSamples())
	for key, md := range sink.CollectedMetadata() {
		if strings.Contains(key, "samples_total") {
			require.Equal(t, model.MetricTypeCounter, md.Type)
		} else {
			require.Equal(t, model.MetricTypeGauge, md.Type)
		}
	}
	for _, scope := range []string{"total", "group"} {
		ls := labels.FromStrings("__name__", "prometheus_preview_samples_total", "preview_id", c.opts.ID, "preview_scope", scope, "sample_type", "float")
		if scope == "group" {
			builder := labels.NewBuilder(ls)
			builder.Set("preview_group_job", "api")
			ls = builder.Labels()
		}
		require.Equal(t, float64(1), got[ls.String()].Value)
	}
}

func reportValue(t *testing.T, samples []reportSample, name, scope string) float64 {
	t.Helper()
	for _, s := range samples {
		if s.labels.Get("__name__") == "prometheus_preview_"+name && s.labels.Get("preview_scope") == scope {
			return s.value
		}
	}
	t.Fatalf("missing metric %s with scope %s", name, scope)
	return 0
}

func TestWindowLifecycle(t *testing.T) {
	args := testArguments()
	c := newTestComponent(t, args)
	start := c.started
	appendSample(t, c, "first")

	samples, _ := c.snapshot(start.Add(time.Minute))
	require.Equal(t, float64(1), reportValue(t, samples, "series", "total"))
	require.Zero(t, reportValue(t, samples, "window_complete", ""))
	require.Equal(t, float64(60), reportValue(t, samples, "window_duration_seconds", ""))

	samples, _ = c.snapshot(start.Add(args.Window))
	require.Equal(t, float64(1), reportValue(t, samples, "series", "total"))
	require.Equal(t, float64(1), reportValue(t, samples, "window_complete", ""))
	require.Empty(t, c.stats.series)
	require.Equal(t, uint64(1), c.stats.total.samples[0])

	appendSample(t, c, "first")
	samples, _ = c.snapshot(start.Add(args.Window + time.Minute))
	require.Equal(t, float64(1), reportValue(t, samples, "series", "total"))
	require.Zero(t, reportValue(t, samples, "window_complete", ""))
	require.Equal(t, float64(start.Add(args.Window).UnixMilli())/1000, reportValue(t, samples, "window_start_time_seconds", ""))
	require.Equal(t, uint64(2), c.stats.total.samples[0])
}

type failingReceiver struct {
	appenders.NoopV2
	appendErr, commitErr error
	rolledBack           bool
	committed            bool
}

func (f *failingReceiver) AppenderV2(context.Context) storage.AppenderV2 { return f }
func (f *failingReceiver) Append(storage.SeriesRef, labels.Labels, int64, int64, float64, *histogram.Histogram, *histogram.FloatHistogram, storage.AppendV2Options) (storage.SeriesRef, error) {
	return 0, f.appendErr
}
func (f *failingReceiver) Commit() error {
	f.committed = true
	return f.commitErr
}
func (f *failingReceiver) Rollback() error {
	f.rolledBack = true
	return nil
}

func TestReportFailureDoesNotAffectOtherReceiversOrInput(t *testing.T) {
	for _, atCommit := range []bool{false, true} {
		t.Run(map[bool]string{false: "append", true: "commit"}[atCommit], func(t *testing.T) {
			failure := errors.New("destination unavailable")
			bad := &failingReceiver{}
			if atCommit {
				bad.commitErr = failure
			} else {
				bad.appendErr = failure
			}
			sink := testappender.NewCollectingAppender()
			args := testArguments()
			args.ReportTo = []storage.AppendableV2{bad, testappender.ConstantAppendable{Inner: sink}}
			c := newTestComponent(t, args)
			appendSample(t, c, "first")
			require.ErrorIs(t, c.report(t.Context()), failure)
			require.Equal(t, !atCommit, bad.rolledBack)
			require.Equal(t, atCommit, bad.committed)
			require.NotEmpty(t, sink.CollectedSamples())
			appendSample(t, c, "second")
			require.Equal(t, uint64(2), c.stats.total.samples[0])
			require.Equal(t, uint64(2), c.stats.total.series)
		})
	}
}
