package appenders_test

import (
	"testing"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/prometheus/appenders"
)

func TestAppenderV1AsV2(t *testing.T) {
	ls := labels.FromStrings("__name__", "test_metric")
	h := &histogram.Histogram{Count: 10, Sum: 100}
	fh := &histogram.FloatHistogram{Count: 10, Sum: 100}
	ex := []exemplar.Exemplar{{Labels: labels.FromStrings("trace_id", "abc"), Ts: 1000, Value: 42.5}}
	md := metadata.Metadata{Type: "gauge", Help: "A test metric"}

	tests := []struct {
		name string
		opts storage.AppendV2Options
		h    *histogram.Histogram
		fh   *histogram.FloatHistogram
		st   int64

		expectAppends    int
		expectHistograms int
		expectExemplars  int
		expectMetadata   int
		expectSTZero     int
		expectHistSTZero int
	}{
		{
			name:          "float sample",
			expectAppends: 1,
		},
		{
			name:             "histogram",
			h:                h,
			expectHistograms: 1,
		},
		{
			name:             "float histogram",
			fh:               fh,
			expectHistograms: 1,
		},
		{
			name:           "with metadata",
			opts:           storage.AppendV2Options{Metadata: md},
			expectAppends:  1,
			expectMetadata: 1,
		},
		{
			name:            "with exemplars",
			opts:            storage.AppendV2Options{Exemplars: ex},
			expectAppends:   1,
			expectExemplars: 1,
		},
		{
			name:          "with ST zero sample",
			st:            500,
			expectAppends: 1,
			expectSTZero:  1,
		},
		{
			name:             "histogram with ST zero",
			h:                h,
			st:               500,
			expectHistograms: 1,
			expectHistSTZero: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := &recordingAppender{}
			adapter := &appenders.AppenderV1AsV2{Inner: inner}

			ref, err := adapter.Append(0, ls, tt.st, 1000, 42.5, tt.h, tt.fh, tt.opts)
			require.NoError(t, err)
			require.NotZero(t, ref)

			require.Len(t, inner.appends, tt.expectAppends)
			require.Len(t, inner.histograms, tt.expectHistograms)
			require.Len(t, inner.exemplars, tt.expectExemplars)
			require.Len(t, inner.metadataUpdates, tt.expectMetadata)
			require.Len(t, inner.stZeroSamples, tt.expectSTZero)
			require.Len(t, inner.histSTZeroSamples, tt.expectHistSTZero)
		})
	}
}

func TestAppenderV1AsV2_CommitRollback(t *testing.T) {
	inner := &recordingAppender{}
	adapter := &appenders.AppenderV1AsV2{Inner: inner}
	require.NoError(t, adapter.Commit())
	require.True(t, inner.committed)

	inner2 := &recordingAppender{}
	adapter2 := &appenders.AppenderV1AsV2{Inner: inner2}
	require.NoError(t, adapter2.Rollback())
	require.True(t, inner2.rolledBack)
}

type recordingAppender struct {
	storage.Appender
	appends           []recordedAppend
	histograms        []recordedHistogram
	exemplars         []recordedExemplar
	metadataUpdates   []recordedMetadata
	stZeroSamples     []recordedSTZero
	histSTZeroSamples []recordedHistSTZero
	committed         bool
	rolledBack        bool
}

type (
	recordedAppend     struct{ ref storage.SeriesRef }
	recordedHistogram  struct{ ref storage.SeriesRef }
	recordedExemplar   struct{ ref storage.SeriesRef }
	recordedMetadata   struct{ ref storage.SeriesRef }
	recordedSTZero     struct{ ref storage.SeriesRef }
	recordedHistSTZero struct{ ref storage.SeriesRef }
)

func (r *recordingAppender) Append(ref storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
	r.appends = append(r.appends, recordedAppend{ref})
	return storage.SeriesRef(len(r.appends)), nil
}

func (r *recordingAppender) AppendHistogram(ref storage.SeriesRef, _ labels.Labels, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	r.histograms = append(r.histograms, recordedHistogram{ref})
	return storage.SeriesRef(100 + len(r.histograms)), nil
}

func (r *recordingAppender) AppendExemplar(ref storage.SeriesRef, _ labels.Labels, _ exemplar.Exemplar) (storage.SeriesRef, error) {
	r.exemplars = append(r.exemplars, recordedExemplar{ref})
	return ref, nil
}

func (r *recordingAppender) UpdateMetadata(ref storage.SeriesRef, _ labels.Labels, _ metadata.Metadata) (storage.SeriesRef, error) {
	r.metadataUpdates = append(r.metadataUpdates, recordedMetadata{ref})
	return ref, nil
}

func (r *recordingAppender) AppendSTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64) (storage.SeriesRef, error) {
	r.stZeroSamples = append(r.stZeroSamples, recordedSTZero{ref})
	return ref, nil
}

func (r *recordingAppender) AppendHistogramSTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	r.histSTZeroSamples = append(r.histSTZeroSamples, recordedHistSTZero{ref})
	return ref, nil
}

func (r *recordingAppender) SetOptions(_ *storage.AppendOptions) {}
func (r *recordingAppender) Commit() error                       { r.committed = true; return nil }
func (r *recordingAppender) Rollback() error                     { r.rolledBack = true; return nil }
