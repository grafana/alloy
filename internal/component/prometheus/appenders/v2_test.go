package appenders

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/tsdbutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewV2_NoChildrenReturnsNoop(t *testing.T) {
	app := NewV2(nil, nil, 0, nil, nil)

	_, ok := app.(NoopV2)
	assert.True(t, ok, "expected NoopV2 appender for zero children")
}

func TestNewV2_SingleChildReturnsPassthrough(t *testing.T) {
	app := NewV2([]storage.AppenderV2{&mockAppenderV2{}}, nil, 0, nil, nil)

	_, ok := app.(*passthroughV2)
	assert.True(t, ok, "expected passthroughV2 appender for single child")
}

func TestNewV2_MultipleChildrenReturnsSeriesRefMapping(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(func() { store.Clear() })

	app := NewV2([]storage.AppenderV2{&mockAppenderV2{}, &mockAppenderV2{}}, store, 0, nil, nil)

	_, ok := app.(*seriesRefMappingV2)
	assert.True(t, ok, "expected seriesRefMappingV2 appender for multiple children")
}

func TestNoopV2_ReturnsInputRef(t *testing.T) {
	ref, err := NoopV2{}.Append(42, labels.FromStrings("job", "test"), 0, 1, 1, nil, nil, storage.AppendV2Options{})
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(42), ref)
}

func TestPassthroughV2_ZerosDeadRefs(t *testing.T) {
	child := &mockAppenderV2{}
	app := NewPassthroughV2(child, 10, newTestHistogram(), newTestCounter())

	lbls := labels.FromStrings("job", "test")
	_, err := app.Append(9, lbls, 0, 1, 1, nil, nil, storage.AppendV2Options{})
	require.NoError(t, err)
	_, err = app.Append(10, lbls, 0, 1, 1, nil, nil, storage.AppendV2Options{})
	require.NoError(t, err)

	require.Equal(t, []storage.SeriesRef{0, 10}, child.appendRefs)
}

func TestPassthroughV2_CountsFloatSamples(t *testing.T) {
	partialErr := &storage.AppendPartialError{ExemplarErrors: []error{storage.ErrOutOfOrderExemplar}}
	hardErr := errors.New("append failed")

	for _, tc := range []struct {
		name          string
		h             *histogram.Histogram
		childErr      error
		expectedCount float64
	}{
		{name: "float sample", expectedCount: 1},
		{name: "float sample with exemplar errors", childErr: partialErr, expectedCount: 1},
		{name: "failed float sample", childErr: hardErr, expectedCount: 0},
		// Histograms aren't counted, to match the V1 passthrough.
		{name: "histogram sample", h: tsdbutil.GenerateTestHistogram(1), expectedCount: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			child := &mockAppenderV2{err: tc.childErr}
			samplesForwarded := newTestCounter()
			app := NewPassthroughV2(child, 0, newTestHistogram(), samplesForwarded)

			_, err := app.Append(0, labels.FromStrings("job", "test"), 0, 1, 1, tc.h, nil, storage.AppendV2Options{})
			require.ErrorIs(t, err, tc.childErr)
			require.Equal(t, tc.expectedCount, testutil.ToFloat64(samplesForwarded))
		})
	}
}

func TestPassthroughV2_CommitAndRollback(t *testing.T) {
	child := &mockAppenderV2{}
	app := NewPassthroughV2(child, 0, newTestHistogram(), newTestCounter())
	require.NoError(t, app.Commit())
	require.NoError(t, app.Rollback())
	require.Equal(t, 1, child.commitCalls)
	require.Equal(t, 1, child.rollbackCalls)
}

func TestSeriesRefMappingV2_CreatesAndReusesMapping(t *testing.T) {
	store := newMockMappingStore()
	child1 := &mockAppenderV2{nextRef: 100}
	child2 := &mockAppenderV2{nextRef: 200}
	samplesForwarded := newTestCounter()
	app := NewSeriesRefMappingV2([]storage.AppenderV2{child1, child2}, store, newTestHistogram(), samplesForwarded)

	lbls := labels.FromStrings("job", "test")
	ref, err := app.Append(0, lbls, 0, 1, 1, nil, nil, storage.AppendV2Options{})
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(1000), ref)
	require.Len(t, store.createCalls, 1)
	require.Equal(t, []storage.SeriesRef{101, 201}, store.createCalls[0].refs)

	ref, err = app.Append(ref, lbls, 0, 2, 2, nil, nil, storage.AppendV2Options{})
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(1000), ref)
	require.Len(t, store.createCalls, 1)

	require.Equal(t, []storage.SeriesRef{0, 101}, child1.appendRefs)
	require.Equal(t, []storage.SeriesRef{0, 201}, child2.appendRefs)
	require.Equal(t, float64(4), testutil.ToFloat64(samplesForwarded))

	require.NoError(t, app.Commit())
	require.Len(t, store.trackCalls, 1)
	require.Equal(t, []storage.SeriesRef{1000, 1000}, store.trackCalls[0].refs)
	require.Equal(t, 1, child1.commitCalls)
	require.Equal(t, 1, child2.commitCalls)
}

func TestSeriesRefMappingV2_PartialErrorsDoNotFailAppend(t *testing.T) {
	err1 := errors.New("exemplar 1 failed")
	err2 := errors.New("exemplar 2 failed")
	store := newMockMappingStore()
	child1 := &mockAppenderV2{nextRef: 100, err: &storage.AppendPartialError{ExemplarErrors: []error{err1}}}
	child2 := &mockAppenderV2{nextRef: 200, err: &storage.AppendPartialError{ExemplarErrors: []error{err2}}}
	app := NewSeriesRefMappingV2([]storage.AppenderV2{child1, child2}, store, newTestHistogram(), newTestCounter())

	ref, err := app.Append(0, labels.FromStrings("job", "test"), 0, 1, 1, nil, nil, storage.AppendV2Options{})

	// The mapping is still created, and the exemplar errors of all children
	// are merged into a single partial error.
	require.Equal(t, storage.SeriesRef(1000), ref)
	require.Len(t, store.createCalls, 1)
	var partialErr *storage.AppendPartialError
	require.ErrorAs(t, err, &partialErr)
	require.Equal(t, []error{err1, err2}, partialErr.ExemplarErrors)
}

func TestSeriesRefMappingV2_HardErrorFailsAppend(t *testing.T) {
	hardErr := errors.New("append failed")
	store := newMockMappingStore()
	child1 := &mockAppenderV2{nextRef: 100}
	child2 := &mockAppenderV2{nextRef: 200, err: hardErr}
	app := NewSeriesRefMappingV2([]storage.AppenderV2{child1, child2}, store, newTestHistogram(), newTestCounter())

	ref, err := app.Append(0, labels.FromStrings("job", "test"), 0, 1, 1, nil, nil, storage.AppendV2Options{})
	require.ErrorIs(t, err, hardErr)
	require.Equal(t, storage.SeriesRef(0), ref)
	require.Empty(t, store.createCalls)
}

func newTestHistogram() prometheus.Histogram {
	return prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_write_latency", Help: "test"})
}

func newTestCounter() prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{Name: "test_samples_forwarded", Help: "test"})
}

// mockAppenderV2 records the refs it receives. When it gets ref 0 it
// allocates a new ref from nextRef, otherwise it returns the ref it was
// given. Every Append returns err.
type mockAppenderV2 struct {
	nextRef storage.SeriesRef
	err     error

	appendRefs    []storage.SeriesRef
	commitCalls   int
	rollbackCalls int
}

var _ storage.AppenderV2 = (*mockAppenderV2)(nil)

func (m *mockAppenderV2) Append(ref storage.SeriesRef, _ labels.Labels, _, _ int64, _ float64, _ *histogram.Histogram, _ *histogram.FloatHistogram, _ storage.AppendV2Options) (storage.SeriesRef, error) {
	m.appendRefs = append(m.appendRefs, ref)
	if ref == 0 {
		m.nextRef++
		ref = m.nextRef
	}
	return ref, m.err
}

func (m *mockAppenderV2) Commit() error {
	m.commitCalls++
	return nil
}

func (m *mockAppenderV2) Rollback() error {
	m.rollbackCalls++
	return nil
}
