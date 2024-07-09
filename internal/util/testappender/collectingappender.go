package testappender

import (
	"context"
	"sync"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"golang.org/x/exp/maps"
)

type MetricSample struct {
	Timestamp int64
	Value     float64
	Labels    labels.Labels
}

// CollectingAppender is an Appender that collects the samples it receives in a map. Useful for testing and verifying
// the samples that are being written.
type CollectingAppender interface {
	storage.Appender
	CollectedSamples() map[string]*MetricSample
	LatestSampleFor(labels string) *MetricSample
}

type collectingAppender struct {
	mut           sync.Mutex
	latestSamples map[string]*MetricSample
}

func NewCollectingAppender() CollectingAppender {
	return &collectingAppender{
		latestSamples: map[string]*MetricSample{},
	}
}

func (c *collectingAppender) CollectedSamples() map[string]*MetricSample {
	c.mut.Lock()
	defer c.mut.Unlock()
	cp := map[string]*MetricSample{}
	maps.Copy(cp, c.latestSamples)
	return cp
}

func (c *collectingAppender) LatestSampleFor(labels string) *MetricSample {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.latestSamples[labels]
}

func (c *collectingAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.latestSamples[l.String()] = &MetricSample{
		Timestamp: t,
		Value:     v,
		Labels:    l,
	}
	return ref, nil
}

func (c *collectingAppender) Commit() error {
	return nil
}

func (c *collectingAppender) Rollback() error {
	return nil
}

func (c *collectingAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	panic("not implemented yet")
}

func (c *collectingAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	panic("not implemented yet")
}

func (c *collectingAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	panic("not implemented yet")
}

func (c *collectingAppender) AppendCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64) (storage.SeriesRef, error) {
	panic("not implemented yet for this test appender")
}

type ConstantAppendable struct {
	Inner CollectingAppender
}

func (c ConstantAppendable) Appender(_ context.Context) storage.Appender {
	return c.Inner
}

var _ storage.Appendable = &ConstantAppendable{}
