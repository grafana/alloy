package process

import (
	"context"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

var _ storage.Appender = (*bulkAppender)(nil)

type bulkAppender struct {
	ctx     context.Context
	wasm    *WasmPlugin
	metrics []*PrometheusMetric
	next    storage.Appendable
}

func (b *bulkAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	labels := make([]*Label, len(l))
	for j, lbl := range l {
		labels[j] = &Label{
			Name:  lbl.Name,
			Value: lbl.Value,
		}
	}
	m := &PrometheusMetric{
		Value:       v,
		Timestampms: t,
		Labels:      labels,
	}
	b.metrics = append(b.metrics, m)
	return ref, nil
}

func (b *bulkAppender) Commit() error {
	return b.process()
}

func (b *bulkAppender) Rollback() error {
	return nil
}

func (b *bulkAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (b *bulkAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (b *bulkAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (b *bulkAppender) AppendCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64) (storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (b *bulkAppender) process() error {
	pt := &Passthrough{
		// TODO reduce the number of random types that
		// represent the same thing.
		Prommetrics: b.metrics,
	}
	outpt, err := b.wasm.Process(pt)
	if err != nil {
		return err
	}
	app := b.next.Appender(b.ctx)
	for _, m := range outpt.Prommetrics {
		labelsBack := make(labels.Labels, len(m.Labels))
		for i, l := range m.Labels {
			labelsBack[i] = labels.Label{
				Name:  l.Name,
				Value: l.Value,
			}
		}
		// We explicitly dont care about errors from append
		_, _ = app.Append(0, labelsBack, m.Timestampms, m.Value)
	}
	// We explicitly dont care about errors from commit
	_ = app.Commit()
	return nil

}
