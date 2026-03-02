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
	t.Cleanup(func() { store.Clear() })

	child1 := &mockAppender{}
	child2 := &mockAppender{}
	app := New([]storage.Appender{child1, child2}, store, 0, nil, nil)

	_, ok := app.(*seriesRefMapping)
	assert.True(t, ok, "expected seriesRefMapping appender for multiple children")
}

func TestNew_PassthroughReceivesDeadRefThreshold(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)

	// Issue a mapping so nextUniqueRef advances past 1.
	lbls := labels.FromStrings("job", "test")
	store.CreateMapping([]storage.SeriesRef{100, 200}, lbls)

	// Clear advances firstRefOfCurrentGeneration to the current nextUniqueRef.
	threshold := store.Clear()
	require.Greater(t, uint64(threshold), uint64(0))

	sf := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_forwarded", Help: "test"}, []string{})
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
