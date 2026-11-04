package remotewrite

import (
	"testing"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/prometheus/prometheus/tsdb/wlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/static/metrics/wal"
	"github.com/grafana/alloy/internal/util"
)

// TestSTZeroSampleLandsOnCorrectWALSeries sends ST zero samples through the
// real Fanout, label store, interceptor, and WAL, and reads the WAL back to
// check which series each zero was written to.
func TestSTZeroSampleLandsOnCorrectWALSeries(t *testing.T) {
	const (
		st = 4000
		ts = 5000
	)
	h := &histogram.Histogram{Count: 1, Sum: 1, PositiveSpans: []histogram.Span{{Offset: 0, Length: 1}}, PositiveBuckets: []int64{1}}

	for _, tc := range []struct {
		name   string
		append func(storage.Appender, labels.Labels) (storage.SeriesRef, error)
	}{
		{
			name: "st_zero_sample",
			append: func(app storage.Appender, l labels.Labels) (storage.SeriesRef, error) {
				ref, err := app.AppendSTZeroSample(0, l, ts, st)
				if err != nil {
					return ref, err
				}
				return app.Append(ref, l, ts, 1)
			},
		},
		{
			name: "histogram_st_zero_sample",
			append: func(app storage.Appender, l labels.Labels) (storage.SeriesRef, error) {
				ref, err := app.AppendHistogramSTZeroSample(0, l, ts, st, h, nil)
				if err != nil {
					return ref, err
				}
				return app.AppendHistogram(ref, l, ts, h, nil)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			w, err := wal.NewStorage(util.TestAlloyLogger(t).Slog(), promclient.NewRegistry(), dir)
			require.NoError(t, err)
			defer w.Close()

			// A series already in the WAL, for example one replayed after a
			// restart, gets WAL ref 1.
			oldSeries := labels.FromStrings("__name__", "old_series")
			app := w.Appender(t.Context())
			_, err = app.Append(0, oldSeries, 1000, 5)
			require.NoError(t, err)
			require.NoError(t, app.Commit())

			// A fresh label store, as after a restart, gives the new series
			// global ref 1 too, so the global and WAL refs collide.
			ls := labelstore.New(nil, promclient.NewRegistry())
			rw := NewInterceptor("prometheus.remote_write.test", &atomic.Bool{}, &debugDataPublisherSpy{}, ls, w)
			fanout := prometheus.NewFanout([]storage.AppendableV2{rw}, "prometheus.scrape.test", promclient.NewRegistry(), ls)

			newSeries := labels.FromStrings("__name__", "new_series")
			app = fanout.Appender(t.Context())
			ref, err := tc.append(app, newSeries)
			require.NoError(t, err)
			require.Equal(t, storage.SeriesRef(1), ref, "global ref of the new series")
			require.NoError(t, app.Commit())

			var zeroOn []string
			for _, s := range readWALSamples(t, dir) {
				if s.t == st {
					zeroOn = append(zeroOn, s.labels)
				}
			}
			require.Equal(t, []string{newSeries.String()}, zeroOn, "the ST zero must be written to the new series only")
		})
	}
}

type walSample struct {
	labels string
	t      int64
}

// readWALSamples returns every float and histogram sample in the WAL under
// dir, with the labels of the series it was written to.
func readWALSamples(t *testing.T, dir string) []walSample {
	t.Helper()

	sr, err := wlog.NewSegmentsReader(wal.SubDirectory(dir))
	require.NoError(t, err)
	defer sr.Close()

	r := wlog.NewReader(sr)
	dec := record.NewDecoder(labels.NewSymbolTable(), nil)
	series := map[uint64]string{}
	var out []walSample
	for r.Next() {
		rec := r.Record()
		switch dec.Type(rec) {
		case record.Series:
			ss, err := dec.Series(rec, nil)
			require.NoError(t, err)
			for _, s := range ss {
				series[uint64(s.Ref)] = s.Labels.String()
			}
		case record.Samples:
			ss, err := dec.Samples(rec, nil)
			require.NoError(t, err)
			for _, s := range ss {
				out = append(out, walSample{labels: series[uint64(s.Ref)], t: s.T})
			}
		case record.HistogramSamples, record.CustomBucketsHistogramSamples:
			hs, err := dec.HistogramSamples(rec, nil)
			require.NoError(t, err)
			for _, s := range hs {
				out = append(out, walSample{labels: series[uint64(s.Ref)], t: s.T})
			}
		}
	}
	require.NoError(t, r.Err())
	return out
}
