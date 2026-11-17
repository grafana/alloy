package adapter_test

import (
	"errors"
	"testing"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/tsdbutil"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/prometheus/appenders/adapter"
)

var testLabels = labels.FromStrings("__name__", "test_metric", "job", "test")

func TestAppenderV1AsV2_FloatSample(t *testing.T) {
	rec := &recordingAppender{}
	app := adapter.AppenderV1AsV2(rec)

	e1 := exemplar.Exemplar{Labels: labels.FromStrings("trace_id", "1"), Value: 1, Ts: 900, HasTs: true}
	e2 := exemplar.Exemplar{Labels: labels.FromStrings("trace_id", "2"), Value: 2, Ts: 950, HasTs: true}
	meta := metadata.Metadata{Type: "counter", Help: "help", Unit: "seconds"}

	// The recorder allocates ref 7 for the sample, which must be used for
	// the exemplars and metadata.
	rec.appendRef = 7
	ref, err := app.Append(0, testLabels, 500, 1000, 42, nil, nil, storage.AppendV2Options{
		Exemplars: []exemplar.Exemplar{e1, e2},
		Metadata:  meta,
	})
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(7), ref)

	require.Equal(t, []call{
		{method: "AppendSTZeroSample", ref: 0, t: 1000, st: 500},
		{method: "Append", ref: 0, t: 1000, v: 42},
		{method: "AppendExemplar", ref: 7, e: e1},
		{method: "AppendExemplar", ref: 7, e: e2},
		{method: "UpdateMetadata", ref: 7, m: meta},
	}, rec.calls)
}

func TestAppenderV1AsV2_Histograms(t *testing.T) {
	h := tsdbutil.GenerateTestHistogram(1)
	fh := tsdbutil.GenerateTestFloatHistogram(2)

	for _, tc := range []struct {
		name string
		h    *histogram.Histogram
		fh   *histogram.FloatHistogram
	}{
		{name: "integer histogram", h: h},
		{name: "float histogram", fh: fh},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingAppender{}
			app := adapter.AppenderV1AsV2(rec)

			_, err := app.Append(3, testLabels, 500, 1000, 0, tc.h, tc.fh, storage.AppendV2Options{})
			require.NoError(t, err)

			require.Equal(t, []call{
				{method: "AppendHistogramSTZeroSample", ref: 3, t: 1000, st: 500, h: tc.h, fh: tc.fh},
				{method: "AppendHistogram", ref: 3, t: 1000, h: tc.h, fh: tc.fh},
			}, rec.calls)
		})
	}
}

func TestAppenderV1AsV2_NoSTOrMetadata(t *testing.T) {
	rec := &recordingAppender{}
	app := adapter.AppenderV1AsV2(rec)

	// A zero start timestamp and empty metadata must not produce V1 calls.
	_, err := app.Append(3, testLabels, 0, 1000, 42, nil, nil, storage.AppendV2Options{})
	require.NoError(t, err)

	require.Equal(t, []call{
		{method: "Append", ref: 3, t: 1000, v: 42},
	}, rec.calls)
}

func TestAppenderV1AsV2_STRefIsNotPropagated(t *testing.T) {
	rec := &recordingAppender{stRef: 999}
	app := adapter.AppenderV1AsV2(rec)

	_, err := app.Append(3, testLabels, 500, 1000, 42, nil, nil, storage.AppendV2Options{})
	require.NoError(t, err)

	require.Equal(t, []call{
		{method: "AppendSTZeroSample", ref: 3, t: 1000, st: 500},
		{method: "Append", ref: 3, t: 1000, v: 42},
	}, rec.calls)
}

func TestAppenderV1AsV2_STErrorIsIgnored(t *testing.T) {
	rec := &recordingAppender{stErr: storage.ErrOutOfOrderST}
	app := adapter.AppenderV1AsV2(rec)

	ref, err := app.Append(3, testLabels, 500, 1000, 42, nil, nil, storage.AppendV2Options{})
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(3), ref)
	require.Len(t, rec.calls, 2)
}

func TestAppenderV1AsV2_AppendErrorSkipsExemplarsAndMetadata(t *testing.T) {
	appendErr := errors.New("append failed")
	rec := &recordingAppender{appendErr: appendErr}
	app := adapter.AppenderV1AsV2(rec)

	_, err := app.Append(3, testLabels, 0, 1000, 42, nil, nil, storage.AppendV2Options{
		Exemplars: []exemplar.Exemplar{{Value: 1, Ts: 900, HasTs: true}},
		Metadata:  metadata.Metadata{Type: "gauge"},
	})
	require.ErrorIs(t, err, appendErr)

	require.Equal(t, []call{
		{method: "Append", ref: 3, t: 1000, v: 42},
	}, rec.calls)
}

func TestAppenderV1AsV2_ExemplarErrorsArePartial(t *testing.T) {
	oooErr := storage.ErrOutOfOrderExemplar
	rec := &recordingAppender{exemplarErrs: []error{nil, storage.ErrDuplicateExemplar, oooErr}}
	app := adapter.AppenderV1AsV2(rec)

	meta := metadata.Metadata{Type: "gauge"}
	ref, err := app.Append(3, testLabels, 0, 1000, 42, nil, nil, storage.AppendV2Options{
		Exemplars: []exemplar.Exemplar{
			{Value: 1, Ts: 900, HasTs: true},
			{Value: 2, Ts: 900, HasTs: true},
			{Value: 3, Ts: 800, HasTs: true},
		},
		Metadata: meta,
	})
	require.Equal(t, storage.SeriesRef(3), ref)

	// Duplicates are dropped silently, like in Prometheus' own appenders;
	// other exemplar failures are reported as a partial error.
	var partialErr *storage.AppendPartialError
	require.ErrorAs(t, err, &partialErr)
	require.Equal(t, []error{oooErr}, partialErr.ExemplarErrors)

	// Metadata is still updated after exemplar failures.
	require.Equal(t, call{method: "UpdateMetadata", ref: 3, m: meta}, rec.calls[len(rec.calls)-1])
}

func TestAppenderV1AsV2_RejectOutOfOrder(t *testing.T) {
	rec := &recordingAppender{}
	app := adapter.AppenderV1AsV2(rec)

	for _, reject := range []bool{false, true, true, false} {
		_, err := app.Append(3, testLabels, 0, 1000, 42, nil, nil, storage.AppendV2Options{RejectOutOfOrder: reject})
		require.NoError(t, err)
	}

	// SetOptions is only called when the option changes.
	require.Equal(t, []storage.AppendOptions{
		{DiscardOutOfOrder: true},
		{DiscardOutOfOrder: false},
	}, rec.setOptions)
}

func TestAppenderV1AsV2_CommitAndRollback(t *testing.T) {
	rec := &recordingAppender{}
	app := adapter.AppenderV1AsV2(rec)
	require.NoError(t, app.Commit())
	require.NoError(t, app.Rollback())
	require.Equal(t, 1, rec.commits)
	require.Equal(t, 1, rec.rollbacks)
}

type call struct {
	method string
	ref    storage.SeriesRef
	t, st  int64
	v      float64
	h      *histogram.Histogram
	fh     *histogram.FloatHistogram
	e      exemplar.Exemplar
	m      metadata.Metadata
}

// recordingAppender records every V1 call made to it. Sample appends return
// appendRef if set, and the input ref otherwise.
type recordingAppender struct {
	calls      []call
	setOptions []storage.AppendOptions
	commits    int
	rollbacks  int

	appendRef    storage.SeriesRef
	appendErr    error
	stRef        storage.SeriesRef
	stErr        error
	exemplarErrs []error
}

var _ storage.Appender = (*recordingAppender)(nil)

func (r *recordingAppender) sampleRef(ref storage.SeriesRef) storage.SeriesRef {
	if r.appendRef != 0 {
		return r.appendRef
	}
	return ref
}

func (r *recordingAppender) stResult(ref storage.SeriesRef) (storage.SeriesRef, error) {
	if r.stErr != nil {
		return 0, r.stErr
	}
	if r.stRef != 0 {
		return r.stRef, nil
	}
	return ref, nil
}

func (r *recordingAppender) Append(ref storage.SeriesRef, _ labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	r.calls = append(r.calls, call{method: "Append", ref: ref, t: t, v: v})
	return r.sampleRef(ref), r.appendErr
}

func (r *recordingAppender) AppendHistogram(ref storage.SeriesRef, _ labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	r.calls = append(r.calls, call{method: "AppendHistogram", ref: ref, t: t, h: h, fh: fh})
	return r.sampleRef(ref), r.appendErr
}

func (r *recordingAppender) AppendSTZeroSample(ref storage.SeriesRef, _ labels.Labels, t, st int64) (storage.SeriesRef, error) {
	r.calls = append(r.calls, call{method: "AppendSTZeroSample", ref: ref, t: t, st: st})
	return r.stResult(ref)
}

func (r *recordingAppender) AppendHistogramSTZeroSample(ref storage.SeriesRef, _ labels.Labels, t, st int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	r.calls = append(r.calls, call{method: "AppendHistogramSTZeroSample", ref: ref, t: t, st: st, h: h, fh: fh})
	return r.stResult(ref)
}

func (r *recordingAppender) AppendExemplar(ref storage.SeriesRef, _ labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	var err error
	if n := len(r.exemplarCalls()); n < len(r.exemplarErrs) {
		err = r.exemplarErrs[n]
	}
	r.calls = append(r.calls, call{method: "AppendExemplar", ref: ref, e: e})
	return ref, err
}

func (r *recordingAppender) exemplarCalls() []call {
	var out []call
	for _, c := range r.calls {
		if c.method == "AppendExemplar" {
			out = append(out, c)
		}
	}
	return out
}

func (r *recordingAppender) UpdateMetadata(ref storage.SeriesRef, _ labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	r.calls = append(r.calls, call{method: "UpdateMetadata", ref: ref, m: m})
	return ref, nil
}

func (r *recordingAppender) SetOptions(opts *storage.AppendOptions) {
	r.setOptions = append(r.setOptions, *opts)
}

func (r *recordingAppender) Commit() error {
	r.commits++
	return nil
}

func (r *recordingAppender) Rollback() error {
	r.rollbacks++
	return nil
}
