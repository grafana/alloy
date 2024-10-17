package scrape

import (
	"context"
	"errors"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"sync"
)

var _ storage.Appendable = (*batchAppendable)(nil)

type batchAppendable struct {
	mut   sync.Mutex
	ctx   context.Context
	child *prometheus.Fanout
}

func (b *batchAppendable) Appender(ctx context.Context) storage.Appender {
	b.mut.Lock()
	defer b.mut.Unlock()

	return &batchAppender{
		ctx:        ctx,
		child:      b.child,
		samples:    make([]*prometheus.Sample, 0),
		histograms: make([]*prometheus.Histogram, 0),
		metadata:   make([]metadata.Metadata, 0),
	}
}

func (b *batchAppendable) UpdateChildren(children []prometheus.Appender) {
	b.mut.Lock()
	defer b.mut.Unlock()

	b.child.UpdateChildren(children)
}

var _ storage.Appender = (*batchAppender)(nil)

type batchAppender struct {
	ctx        context.Context
	samples    []*prometheus.Sample
	histograms []*prometheus.Histogram
	metadata   []metadata.Metadata
	child      *prometheus.Fanout
}

func (b *batchAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	s := &prometheus.Sample{
		Timestamp: t,
		Value:     v,
		Ref:       l.Hash(),
	}
	b.samples = append(b.samples, s)
	return storage.SeriesRef(s.Ref), nil
}

func (b *batchAppender) Commit() error {
	sampleErr := b.child.AppendSamples(b.samples)
	histErr := b.child.AppendHistograms(b.histograms)
	metaErr := b.child.AppendMetadata(b.metadata)
	return errors.Join(sampleErr, histErr, metaErr)
}

func (b *batchAppender) Rollback() error {
	return nil
}

func (b *batchAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	// Should I use a map, most samples don't have exemplars so its more costly memory but cheaper cpu?
	for _, s := range b.samples {
		if ref == storage.SeriesRef(s.Ref) {
			s.Exemplar = e
			break
		}
	}
	for _, s := range b.histograms {
		if ref == storage.SeriesRef(s.Ref) {
			s.Exemplar = e
			break
		}
	}
	return ref, nil
}

func (b *batchAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	s := &prometheus.Histogram{
		Timestamp: t,
		Ref:       l.Hash(),
	}
	if h != nil {
		s.Histogram = h
	} else {
		s.FloatHistogram = fh
	}
	b.histograms = append(b.histograms, s)
	return storage.SeriesRef(s.Ref), nil
}

func (b *batchAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	b.metadata = append(b.metadata, m)
	return ref, nil
}

func (b *batchAppender) AppendCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64) (storage.SeriesRef, error) {
	return ref, nil
}
