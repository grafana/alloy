package appenders

import (
	"testing"

	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/assert"
)

func TestNew_NoChildrenReturnsNoop(t *testing.T) {
	app := New(nil, nil, false, nil, nil)

	_, ok := app.(Noop)
	assert.True(t, ok, "expected Noop appender for zero children")
}

func TestNew_SingleChildReturnsPassthrough(t *testing.T) {
	child := &mockAppender{}
	app := New([]storage.Appender{child}, nil, false, nil, nil)

	_, ok := app.(*passthrough)
	assert.True(t, ok, "expected passthrough appender for single child")
}

func TestNew_MultipleChildrenReturnsSeriesRefMapping(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)

	child1 := &mockAppender{}
	child2 := &mockAppender{}
	app := New([]storage.Appender{child1, child2}, store, false, nil, nil)

	_, ok := app.(*seriesRefMapping)
	assert.True(t, ok, "expected seriesRefMapping appender for multiple children")
}

func TestNew_SingleChildMappedReturnsSeriesRefMapping(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)

	app := New([]storage.Appender{&mockAppender{}}, store, true, nil, nil)

	_, ok := app.(*seriesRefMapping)
	assert.True(t, ok, "expected seriesRefMapping appender for a mapped single child")
}
