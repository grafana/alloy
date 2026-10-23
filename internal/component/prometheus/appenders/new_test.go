package appenders

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_NoChildrenReturnsNoop(t *testing.T) {
	app := New(nil, nil, 0, nil, nil)

	_, ok := app.(Noop)
	assert.True(t, ok, "expected Noop appender for zero children")
}

func TestNew_SingleChildReturnsPassthrough(t *testing.T) {
	child := &mockAppender{}
	app := New([]storage.Appender{child}, nil, 0, nil, nil)

	_, ok := app.(*passthrough)
	assert.True(t, ok, "expected passthrough appender for single child")
}

func TestNew_MultipleChildrenReturnsSeriesRefMapping(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)

	child1 := &mockAppender{}
	child2 := &mockAppender{}
	app := New([]storage.Appender{child1, child2}, store, 0, nil, nil)

	_, ok := app.(*seriesRefMapping)
	assert.True(t, ok, "expected seriesRefMapping appender for multiple children")
}

func TestNew_PassthroughReceivesDeadRefThreshold(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)

	// Issue a mapping so nextUniqueRef advances past 1.
	lbls := labels.FromStrings("job", "test")
	store.CreateMapping([]storage.SeriesRef{100, 200}, lbls)

	// Clear advances firstRefOfCurrentGeneration to the current nextUniqueRef.
	threshold := store.Clear()
	require.Greater(t, uint64(threshold), uint64(0))

	sf := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_forwarded", Help: "test"})
	child := &mockAppender{appendFn: func(ref storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		return ref, nil // echo back whatever ref we receive
	}}
	app := New([]storage.Appender{child}, store, threshold, nil, sf)

	// A ref below the threshold must be zeroed by the passthrough.
	staleRef := threshold - 1
	_, err := app.Append(staleRef, lbls, 1, 1.0)
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(0), child.appendRefs[0],
		"passthrough must zero refs below the dead ref threshold")
}

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
